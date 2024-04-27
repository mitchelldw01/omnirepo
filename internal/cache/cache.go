package cache

// CACHE STRATEGY OVERVIEW
//
// - workspace.json: keys of the map are the hashes of workspace inputs
// - <dir>-meta.tar.zst
//   - inputs.json: keys of the map are the hashes of target inputs
//   - outputs/: top-level directory containing the outputs of all tasks for the target directory
//     - outputs/<task>/: contains the assets produced by the given task
//   - results.json: contains the output from the previous task run and if it failed or not
//
// READING INPUTS/RESULTS: When the cache for a given target directory is read, the tarball is first unpacked to a
// temporary location (omni-prev-cache). The appropriate `inputs.json` or `results.json` is then
// read into memory.
//
// WRITING INPUTS/RESULTS: The results of given task are written to a temporary location (omni-next-cache) as
// soon as they are received. Once all tasks are complete, tarballs are generated from this
// location and written to their final destination.
//
// OUTPUT RESTORATION: Once all tasks are complete, the cached outputs are copied from their
// temporary location (omni-prev-cache) to the correct location in the user's workspace.

import (
	"sync"
	"time"

	"github.com/mitchelldw01/omnirepo/internal/graph"
	"github.com/mitchelldw01/omnirepo/usercfg"
)

// Produces readers and writers to be used for reading/writing cache assets.
type Transporter interface{}

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
	hasher        sha256Hasher
	workCache     *concurrentMap[struct{}]
	targetCache   *nestedConcurrentMap[struct{}]
	// Map from `node.Dir`s to set of `node.Name`s of invalid caches
	invalidCaches *nestedConcurrentMap[struct{}]
	// Ensures safe initialization of the workspace cache
	initWorkLock  sync.Mutex
	isWorkInvalid bool
}

func NewCache(transport Transporter, targetCfgs map[string]usercfg.TargetConfig) *Cache {
	return &Cache{
		transport:     transport,
		targetConfigs: targetCfgs,
		hasher:        *newSha256Hasher(),
		workCache:     newConcurrentMap[struct{}](),
		targetCache:   newNestedConcurrentMap[struct{}](),
		invalidCaches: newNestedConcurrentMap[struct{}](),
		initWorkLock:  sync.Mutex{},
	}
}

func (c *Cache) IsClean(node *graph.Node, deps map[string]struct{}) (bool, error) {
	return false, nil
}

func (c *Cache) GetTaskResult(node *graph.Node) (TaskResult, error) {
	return TaskResult{}, nil
}

func (c *Cache) WriteTaskResult(id string, res TaskResult) error {
	return nil
}

func (c *Cache) CleanUp(t time.Time) error {
	// write the final caches
	// restore the outputs
	return nil
}
