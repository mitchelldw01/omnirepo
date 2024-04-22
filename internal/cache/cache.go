package cache

import (
	"time"

	"github.com/mitchelldw01/omnirepo/internal/graph"
	"github.com/mitchelldw01/omnirepo/usercfg"
)

// Produces readers and writers to be used for reading/writing cache assets.
type Transporter interface{}

// Manages concurrent access to caches.
// It prevents users from accessing the cache when another user has acquired the lock.
type Locker interface {
	Lock() error
	Unlock() error
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
	transporter   Transporter
	locker        Locker
	targetConfigs map[string]usercfg.TargetConfig
}

func NewCache(t Transporter, l Locker, targetCfgs map[string]usercfg.TargetConfig) *Cache {
	return &Cache{
		transporter:   t,
		locker:        l,
		targetConfigs: targetCfgs,
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
	return nil
}
