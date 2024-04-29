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

type CacheReader interface {
	GetCachedResult(dir, name string) (cache.TaskResult, error)
	Validate(node *graph.Node, deps map[string]struct{}) (bool, error)
}

type CacheWriter interface {
	WriteTaskResult(dir, name string, res cache.TaskResult) error
	Update() error
}

type Executor struct {
	reader CacheReader
	writer CacheWriter
	stats  *statistics
}

func NewExecutor(cr CacheReader, cw CacheWriter) *Executor {
	return &Executor{
		reader: cr,
		writer: cw,
		stats:  newStatistics(),
	}
}

func (e *Executor) ExecuteTask(node *graph.Node, deps map[string]struct{}) {
	if e.hasFailedDependency(deps) {
		return
	}

	if err := e.executeTaskHelper(node, deps); err != nil {
		e.stats.errors.append(err)
	}
}

func (e *Executor) hasFailedDependency(deps map[string]struct{}) bool {
	for id := range deps {
		if e.stats.failed.contains(id) {
			return true
		}
	}
	return false
}

func (e *Executor) executeTaskHelper(node *graph.Node, deps map[string]struct{}) error {
	valid, err := e.reader.Validate(node, deps)
	if err != nil {
		return err
	}

	var res cache.TaskResult
	if valid {
		res, err = e.reader.GetCachedResult(node.Dir, node.Name)
	} else {
		res = e.executeTaskCommand(node.Pipeline.Command, node.Dir)
	}
	if err != nil {
		return err
	}

	return e.processTaskResult(node, valid, res)
}

func (e *Executor) processTaskResult(node *graph.Node, isClean bool, res cache.TaskResult) error {
	e.stats.total.increment()
	if res.Failed {
		e.stats.failed.put(node.Id)
	}

	var logs string
	if isClean {
		logs = "cache hit, replaying logs...\n" + res.Logs
	} else {
		logs = "cache miss, executing task...\n" + res.Logs
	}

	log.TaskOutput(node.Id, logs)
	if isClean {
		e.stats.hits.increment()
		return nil
	}

	return e.writer.WriteTaskResult(node.Dir, node.Name, res)
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
	return cache.NewTaskResult(strings.TrimSpace(buf.String()), err != nil)
}

func (e *Executor) FinalizeResults(t time.Time) {
	if err := e.writer.Update(); err != nil {
		e.stats.errors.append(err)
	}

	hits := e.stats.hits.val
	total := e.stats.total.val
	failed := len(e.stats.failed.val)
	duration := time.Since(t)
	log.Metrics(hits, total, failed, duration)

	for _, err := range e.stats.errors.val {
		fmt.Print("\n")
		log.Error(err)
	}
}
