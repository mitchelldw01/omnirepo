package exec

import (
	"bytes"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/mitchelldw01/omnirepo/internal/cache"
	"github.com/mitchelldw01/omnirepo/internal/graph"
	"github.com/mitchelldw01/omnirepo/internal/log"
)

// Reads, writes, and retrieves assets from the cache.
type Cacher interface {
	IsClean(node *graph.Node, deps map[string]struct{}) (bool, error)
	GetTaskResult(node *graph.Node) (cache.TaskResult, error)
	WriteTaskResult(dir, name string, res cache.TaskResult) error
	CleanUp(t time.Time) error
}

type Executor struct {
	cache   Cacher
	noCache bool
	metrics *runtimeMetrics
}

func NewExecutor(cache Cacher, noCache bool) *Executor {
	return &Executor{
		cache:   cache,
		noCache: noCache,
		metrics: newRuntimeMetrics(),
	}
}

func (e *Executor) ExecuteTask(node *graph.Node, deps map[string]struct{}) {
	if e.hasFailedDependency(deps) {
		return
	}

	if err := e.executeTaskHelper(node, deps); err != nil {
		e.metrics.errors.append(err)
	}
}

func (e *Executor) hasFailedDependency(deps map[string]struct{}) bool {
	for id := range deps {
		if e.metrics.failed.contains(id) {
			return true
		}
	}
	return false
}

func (e *Executor) executeTaskHelper(node *graph.Node, deps map[string]struct{}) error {
	isClean, err := e.cache.IsClean(node, deps)
	if err != nil {
		return err
	}

	var res cache.TaskResult
	if isClean {
		res, err = e.cache.GetTaskResult(node)
	} else {
		res = e.executeTaskCommand(node.Pipeline.Command, node.Dir)
	}
	if err != nil {
		return err
	}

	return e.processTaskResult(node, isClean, res)
}

func (e *Executor) processTaskResult(node *graph.Node, isClean bool, res cache.TaskResult) error {
	e.metrics.total.increment()
	if res.IsFailed {
		e.metrics.failed.put(node.Id)
	}

	log.TaskOutput(node.Id, res.Output)
	if isClean {
		return nil
	}

	return e.cache.WriteTaskResult(node.Dir, node.Name, res)
}

func (e *Executor) executeTaskCommand(command, dir string) cache.TaskResult {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("powershell", "-NoProfile", "-Command", command)
	default:
		cmd = exec.Command("bash", "-c", command)
	}

	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	cmd.Dir = dir

	err := cmd.Run()
	return cache.NewTaskResult(err != nil, strings.TrimSpace(buf.String()))
}

func (e *Executor) CleanUp(t time.Time) {
	if err := e.cache.CleanUp(t); err != nil {
		e.metrics.errors.append(err)
	}

	hits := e.metrics.hits.val
	total := e.metrics.total.val
	failed := len(e.metrics.failed.val)
	duration := time.Since(t)
	log.Metrics(hits, total, failed, duration)

	for _, err := range e.metrics.errors.val {
		fmt.Print("\n")
		log.Error(err)
	}
}
