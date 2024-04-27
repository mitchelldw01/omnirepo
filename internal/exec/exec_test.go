package exec_test

import (
	"testing"

	"github.com/mitchelldw01/omnirepo/internal/cache"
	"github.com/mitchelldw01/omnirepo/internal/exec"
	"github.com/mitchelldw01/omnirepo/internal/graph"
	"github.com/mitchelldw01/omnirepo/usercfg"
)

type cacher struct {
	isFailed bool
}

func (c *cacher) IsClean(node *graph.Node, deps map[string]struct{}) (bool, error) {
	return false, nil
}

func (c *cacher) GetTaskResult(dir, name string) (cache.TaskResult, error) {
	return cache.TaskResult{}, nil
}

func (c *cacher) WriteTaskResult(dir, name string, res cache.TaskResult) error {
	c.isFailed = res.IsFailed
	return nil
}

func (c *cacher) CleanUp() error {
	return nil
}

func TestExecTask(t *testing.T) {
	testCases := []struct {
		name     string
		cmd      string
		expected bool
	}{
		{
			name: "command should exit cleanly",
			cmd:  "exit 0",
		},
		{
			name:     "command should not exit cleanly",
			cmd:      "exit 1",
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cacher := cacher{}
			ex := exec.NewExecutor(&cacher, false)

			ex.ExecuteTask(graph.NewNode("", "", usercfg.PipelineConfig{
				Command: tc.cmd,
			}), map[string]struct{}{})

			if tc.expected != cacher.isFailed {
				t.Fatalf("expected %v, got %v", tc.expected, cacher.isFailed)
			}
		})
	}
}
