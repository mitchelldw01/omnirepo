package cache

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/mitchelldw01/omnirepo/internal/graph"
	"github.com/mitchelldw01/omnirepo/usercfg"
)

// Provides an io.ReadCloser that reads from the cache location.
// The location can be either the file system or an S3 bucket.
type TransportReader interface {
	Reader(path string) (io.ReadCloser, error)
}

type CacheReader struct {
	transport     TransportReader
	targetConfigs map[string]usercfg.TargetConfig
	targets       []string
	hasher        *sha256Hasher
	// The temporary directory that the existing cache will be extracted to.
	tmpCache string
	// Map from target directories to the ouput patterns for every node
	outputs *concurrentMap[[]string]
	// Map of hashes of cache inputs in the workspace cache
	workCache *concurrentMap[struct{}]
	// Map from target directories to hashes of cache inputs
	targetCache *nestedConcurrentMap[struct{}]
	// Map from node directories to node names
	invalidNodes *nestedConcurrentMap[struct{}]
	// Ensures thread-safe initializion of the workspace cache
	initWorkLock sync.Mutex
	isWorkValid  bool
}

func NewCacheReader(tr TransportReader, configs map[string]usercfg.TargetConfig, targets []string) *CacheReader {
	cleaned := make([]string, 0, len(configs))
	for target := range configs {
		cleaned = append(targets, filepath.Clean(target))
	}

	return &CacheReader{
		transport:     tr,
		targetConfigs: configs,
		targets:       cleaned,
		hasher:        newSha256Hasher(),
		tmpCache:      filepath.Join(os.TempDir(), "omni-prev-cache"),
		outputs:       newConcurrentMap[[]string](),
		targetCache:   newNestedConcurrentMap[struct{}](),
		invalidNodes:  newNestedConcurrentMap[struct{}](),
		initWorkLock:  sync.Mutex{},
		isWorkValid:   true,
	}
}

func (r *CacheReader) GetCachedResult(dir, name string) (TaskResult, error) {
	path := filepath.Join(r.tmpCache, dir, "results", name+".json")
	b, err := os.ReadFile(path)
	if err != nil {
		return TaskResult{}, fmt.Errorf("failed to read task result %q: %v", path, err)
	}

	var res TaskResult
	if err := json.Unmarshal(b, &res); err != nil {
		return TaskResult{}, fmt.Errorf("failed to unmarshal task result: %v", err)
	}

	return res, nil
}

func (r *CacheReader) Validate(node *graph.Node, deps map[string]struct{}) (bool, error) {
	r.outputs.mutex.Lock()
	if r.outputs.data[node.Dir] == nil {
		r.outputs.data[node.Dir] = []string{}
	}
	r.outputs.data[node.Dir] = append(r.outputs.data[node.Dir], node.Pipeline.Outputs...)
	r.outputs.mutex.Unlock()

	isClean, err := r.validateAll(node, deps)
	if !isClean {
		nameSet, _ := r.invalidNodes.getOrPut(node.Dir)
		nameSet.put(node.Name, struct{}{})
	}

	return isClean, err
}

func (r *CacheReader) validateAll(node *graph.Node, deps map[string]struct{}) (bool, error) {
	if !r.isWorkValid || r.hasInvalidDependency(deps) {
		return false, nil
	}

	isWorkClean, err := r.validateWorkspace(node.Dir)
	if err != nil {
		return false, err
	}
	if !isWorkClean {
		r.isWorkValid = false
		return false, nil
	}

	return r.validateTarget(node)
}

func (r *CacheReader) hasInvalidDependency(deps map[string]struct{}) bool {
	for id := range deps {
		index := strings.LastIndex(id, ":")
		dir, name := id[:index], id[index+1:]

		connMap, ok := r.invalidNodes.get(dir)
		if !ok {
			continue
		}
		_, ok = connMap.get(name)
		if ok {
			return true
		}
	}
	return false
}

func (r *CacheReader) validateWorkspace(dir string) (bool, error) {
	paths, err := getCacheableWorkspacePaths(r.targetConfigs[dir].WorkspaceAssets, r.targets)
	if err != nil {
		return false, err
	}

	connMap, err := r.getWorkspaceCache()
	if err != nil && !isNotExistError(err) {
		return false, err
	}
	if len(paths) == 0 {
		if isNotExistError(err) {
			return false, nil
		}
		return true, nil
	}

	return r.mapContainsHashes(connMap, paths)
}

func (r *CacheReader) getWorkspaceCache() (*concurrentMap[struct{}], error) {
	r.initWorkLock.Lock()
	defer r.initWorkLock.Unlock()

	if r.workCache != nil {
		return r.workCache, nil
	}

	connMap := newConcurrentMap[struct{}]()
	r.workCache = connMap

	tr, err := r.transport.Reader("workspace.json")
	if err != nil {
		return connMap, err
	}
	defer tr.Close()

	return connMap, connMap.loadFromReader(tr)
}

func (r *CacheReader) validateTarget(node *graph.Node) (bool, error) {
	paths, err := getCacheableTargetPaths(node.Dir, node.Pipeline.Includes, node.Pipeline.Excludes)
	if err != nil {
		return false, err
	}

	connMap, err := r.getTargetCache(node.Dir)
	if err != nil && !isNotExistError(err) {
		return false, err
	}
	if len(paths) == 0 {
		if isNotExistError(err) {
			return false, nil
		}
		return true, nil
	}

	return r.mapContainsHashes(connMap, paths)
}

func (r *CacheReader) getTargetCache(dir string) (*concurrentMap[struct{}], error) {
	connMap, ok := r.targetCache.getOrPut(dir)
	if ok {
		return connMap, nil
	}

	dst, err := r.unpackTargetCache(dir)
	if err != nil {
		return connMap, err
	}

	path := filepath.Join(dst, "inputs.json")
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read cache artifact %q: %v", path, err)
	}

	return connMap, connMap.loadFromReader(file)
}

func (r *CacheReader) unpackTargetCache(dir string) (string, error) {
	dst := filepath.Join(r.tmpCache, dir)
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return "", fmt.Errorf("failed to create cache directory %q: %v", dst, err)
	}

	src := fmt.Sprintf("%s-meta.tar.zst", dir)
	tr, err := r.transport.Reader(src)
	if err != nil {
		return "", err
	}
	defer tr.Close()

	return dst, unpackTarZst(tr, dst)
}

func (r *CacheReader) mapContainsHashes(connMap *concurrentMap[struct{}], paths []string) (bool, error) {
	hashes, err := r.hasher.hash(paths...)
	if err != nil {
		return false, err
	}
	return connMap.contains(hashes...), nil
}
