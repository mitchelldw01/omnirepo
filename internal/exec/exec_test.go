package exec_test

import (
	"testing"

	"github.com/mitchelldw01/omnirepo/internal/cache"
	"github.com/mitchelldw01/omnirepo/internal/exec"
	"github.com/mitchelldw01/omnirepo/internal/graph"
	"github.com/mitchelldw01/omnirepo/usercfg"
)

type reader struct{}

func (r *reader) GetCachedResult(dir, name string) (cache.TaskResult, error) {
	return cache.TaskResult{}, nil
}

func (r *reader) Validate(node *graph.Node, deps map[string]struct{}) (bool, error) {
	return false, nil
}

type writer struct {
	failed bool
}

func (w *writer) WriteTaskResult(dir, name string, res cache.TaskResult) error {
	w.failed = res.Failed
	return nil
}

func (w *writer) Update() error {
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
			w := writer{}
			ex := exec.NewExecutor(&reader{}, &w, false)

			ex.ExecuteTask(graph.NewNode("", "", usercfg.PipelineConfig{
				Command: tc.cmd,
			}), map[string]struct{}{})

			if tc.expected != w.failed {
				t.Fatalf("expected %v, got %v", tc.expected, w.failed)
			}
		})
	}
}
