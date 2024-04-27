package cache

// READING INPUTS/RESULTS: When the cache for a given target directory is read, the tarball is first unpacked to a
// temporary location (omni-prev-cache). The appropriate `inputs.json` or `results.json` is then
// read into memory.

// WRITING INPUTS/RESULTS: The results of given task are written to a temporary location (omni-next-cache) as
// soon as they are received. Once all tasks are complete, tarballs are generated from this
// location and written to their final destination.

// RESTORING OUTPUTS: Once all tasks are complete, the cached outputs are copied from their
// temporary location (omni-prev-cache) to the correct location in the user's workspace.

// CACHE ASSET BREAKDOWN
// - `workspace.json`: keys of the map are the hashes of workspace inputs
// - `<dir>-meta.tar.zst`: tarball of all cache assets for the target directory
//   - `inputs.json`: keys of the map are the hashes of target inputs
//   - `outputs/`: the outputs of all tasks for the target directory
//     - `outputs/<task>/`: the files produced by the given task
//   - `results/`: the results of all tasks for the target directory
//     - `<task>.json`: the output from the previous task run and if it failed or not

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/mitchelldw01/omnirepo/internal/graph"
	"github.com/mitchelldw01/omnirepo/usercfg"
)

// Produces readers and writers to be used for reading/writing cache assets.
type Transporter interface {
	Reader(path string) (io.ReadCloser, error)
	Writer(path string) (io.WriteCloser, error)
}

type TaskResult struct {
	IsFailed bool
	Output   string
}

func NewTaskResult(isFailed bool, out string) TaskResult {
	return TaskResult{
		Output:   out,
		IsFailed: isFailed,
	}
}

type Cache struct {
	transport     Transporter
	targetConfigs map[string]usercfg.TargetConfig
	// List of keys from targetConfigs
	targets []string
	hasher  sha256Hasher
	// Map from target directories to the output patterns from every node
	outputs      *concurrentMap[[]string]
	workCache    *concurrentMap[struct{}]
	targetCaches *nestedConcurrentMap[struct{}]
	// Map from target directories to set of node names of invalid caches
	invalidCaches *nestedConcurrentMap[struct{}]
	// Ensures safe initialization of the workspace cache
	initWorkLock  sync.Mutex
	isWorkInvalid bool
	prevCacheDir  string
	nextCacheDir  string
}

func NewCache(transport Transporter, targetCfgs map[string]usercfg.TargetConfig) *Cache {
	targets := make([]string, 0, len(targetCfgs))
	for path := range targetCfgs {
		targets = append(targets, path)
	}

	return &Cache{
		transport:     transport,
		targetConfigs: targetCfgs,
		targets:       targets,
		hasher:        *newSha256Hasher(),
		outputs:       newConcurrentMap[[]string](),
		targetCaches:  newNestedConcurrentMap[struct{}](),
		invalidCaches: newNestedConcurrentMap[struct{}](),
		initWorkLock:  sync.Mutex{},
		prevCacheDir:  filepath.Join(os.TempDir(), "omni-prev-cache"),
		nextCacheDir:  filepath.Join(os.TempDir(), "omni-next-cache"),
	}
}

func (c *Cache) Init() error {
	prev := filepath.Join(os.TempDir(), "omni-prev-cache")
	if err := os.RemoveAll(prev); err != nil {
		return err
	}

	next := filepath.Join(os.TempDir(), "omni-next-cache")
	if err := os.RemoveAll(next); err != nil {
		return err
	}

	return os.Mkdir(next, 0o755)
}

func (c *Cache) IsClean(node *graph.Node, deps map[string]struct{}) (bool, error) {
	c.outputs.mutex.Lock()
	if c.outputs.data[node.Dir] == nil {
		c.outputs.data[node.Dir] = []string{}
	}
	c.outputs.data[node.Dir] = append(c.outputs.data[node.Dir], node.Pipeline.Outputs...)
	c.outputs.mutex.Unlock()

	isClean, err := c.isCleanHelper(node, deps)
	if !isClean {
		nameSet, _ := c.invalidCaches.getOrPut(node.Dir)
		nameSet.put(node.Name, struct{}{})
	}

	return isClean, err
}

func (c *Cache) isCleanHelper(node *graph.Node, deps map[string]struct{}) (bool, error) {
	if c.isWorkInvalid || c.hasInvalidDependency(deps) {
		return false, nil
	}

	isWorkClean, err := c.isWorkspaceClean(node.Dir)
	if err != nil {
		return false, err
	}
	if !isWorkClean {
		c.isWorkInvalid = true
		return false, nil
	}

	isTargetClean, err := c.isTargetClean(node)
	if err != nil {
		return false, err
	}
	if !isTargetClean {
		return false, nil
	}

	return false, nil
}

func (c *Cache) hasInvalidDependency(deps map[string]struct{}) bool {
	for id := range deps {
		index := strings.LastIndex(id, ":")
		dir, name := id[:index], id[index+1:]

		set, ok := c.invalidCaches.get(dir)
		if !ok {
			continue
		}
		_, ok = set.get(name)
		if ok {
			return true
		}
	}

	return false
}

func (c *Cache) isWorkspaceClean(dir string) (bool, error) {
	paths, err := getCacheableWorkspacePaths(c.targetConfigs[dir].WorkspaceAssets, c.targets)
	if err != nil {
		return false, err
	}

	connMap, err := c.getWorkspaceCache()
	if err != nil && !isNotExistError(err) {
		return false, err
	}
	if len(paths) == 0 {
		if isNotExistError(err) {
			return false, nil
		}
		return true, nil
	}

	return c.cacheContainsHashes(connMap, paths)
}

func (c *Cache) getWorkspaceCache() (*concurrentMap[struct{}], error) {
	c.initWorkLock.Lock()
	defer c.initWorkLock.Unlock()

	if c.workCache != nil {
		return c.workCache, nil
	}

	connMap := newConcurrentMap[struct{}]()
	c.workCache = connMap

	r, err := c.transport.Reader("workspace.json")
	if err != nil {
		return connMap, err
	}
	defer r.Close()

	return connMap, connMap.loadFromReader(r)
}

func (c *Cache) isTargetClean(node *graph.Node) (bool, error) {
	paths, err := getCacheableTargetPaths(node.Dir, node.Pipeline.Includes, node.Pipeline.Excludes)
	if err != nil {
		return false, err
	}

	connMap, err := c.getTargetCache(node.Dir)
	if err != nil && !isNotExistError(err) {
		return false, err
	}
	if len(paths) == 0 {
		if isNotExistError(err) {
			return false, nil
		}
		return true, nil
	}

	return c.cacheContainsHashes(connMap, paths)
}

func (c *Cache) getTargetCache(dir string) (*concurrentMap[struct{}], error) {
	connMap, ok := c.targetCaches.getOrPut(dir)
	if ok {
		return connMap, nil
	}

	dst, err := c.unpackTargetCache(dir)
	if err != nil {
		return connMap, err
	}

	path := filepath.Join(dst, "inputs.json")
	r, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %q: %v", path, err)
	}

	return connMap, connMap.loadFromReader(r)
}

func (c *Cache) unpackTargetCache(dir string) (string, error) {
	dst := filepath.Join(c.prevCacheDir, dir)
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return "", fmt.Errorf("failed to create cache directory: %v", err)
	}

	src := fmt.Sprintf("%s-meta.tar.zst", dir)
	r, err := c.transport.Reader(src)
	if err != nil {
		return "", err
	}
	defer r.Close()

	return dst, unpackTarZst(r, dst)
}

func (c *Cache) cacheContainsHashes(cache *concurrentMap[struct{}], paths []string) (bool, error) {
	hashes, err := c.hasher.hash(paths...)
	if err != nil {
		return false, err
	}
	return cache.contains(hashes...), nil
}

func (c *Cache) GetTaskResult(dir, name string) (TaskResult, error) {
	path := filepath.Join(c.prevCacheDir, dir, "results", name+".json")
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

func (c *Cache) WriteTaskResult(dir, name string, res TaskResult) error {
	path := filepath.Join(c.nextCacheDir, dir, "results", name+".json")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	b, err := json.Marshal(res)
	if err != nil {
		return fmt.Errorf("failed to marshal task result: %v", err)
	}

	if err := os.WriteFile(path, b, 0644); err != nil {
		return fmt.Errorf("failed to write cache result: %v", err)
	}

	return nil
}

func (c *Cache) CleanUp(t time.Time) error {
	// write the final caches (including ouputs)
	// restore the outputs
	return nil
}
