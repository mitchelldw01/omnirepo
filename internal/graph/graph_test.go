package graph_test

import (
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/mitchelldw01/omnirepo/internal/graph"
	"github.com/mitchelldw01/omnirepo/usercfg"
)

type executor struct{}

func (e executor) ExecuteTask(node *graph.Node, dependencies map[string]struct{}) {}

func (e executor) FinalizeResults(t time.Time) {}

func TestPopulateNodes(t *testing.T) {
	type expected struct {
		nodeIds []string
		deps    map[string]map[string]struct{}
		err     bool
	}

	testCases := []struct {
		name          string
		targetConfigs map[string]usercfg.TargetConfig
		tasks         []string
		dir           string
		expected      expected
	}{
		{
			name: "one config with zero dependencies",
			targetConfigs: map[string]usercfg.TargetConfig{
				"foo": {
					Pipeline: map[string]usercfg.PipelineConfig{
						"test": {},
					},
				},
			},
			tasks: []string{"test"},
			expected: expected{
				nodeIds: []string{"foo:test"},
				deps: map[string]map[string]struct{}{
					"foo:test": {},
				},
			},
		},
		{
			name: "one config with one sibling dependency",
			targetConfigs: map[string]usercfg.TargetConfig{
				"foo": {
					Pipeline: map[string]usercfg.PipelineConfig{
						"test": {
							DependsOn: []string{"build"},
						},
						"build": {},
					},
				},
			},
			tasks: []string{"test"},
			expected: expected{
				nodeIds: []string{"foo:test", "foo:build"},
				deps: map[string]map[string]struct{}{
					"foo:test": {
						"foo:build": {},
					},
					"foo:build": {},
				},
			},
		},
		{
			name: "three configs with one ancestor dependency",
			targetConfigs: map[string]usercfg.TargetConfig{
				"foo": {
					Dependencies: []string{"bar"},
					Pipeline: map[string]usercfg.PipelineConfig{
						"test": {
							DependsOn: []string{"^test"},
						},
					},
				},
				"bar": {
					Pipeline: map[string]usercfg.PipelineConfig{
						"test": {},
					},
				},
				"baz": {
					Pipeline: map[string]usercfg.PipelineConfig{
						"test": {},
					},
				},
			},
			tasks: []string{"test"},
			dir:   "foo",
			expected: expected{
				nodeIds: []string{"foo:test", "bar:test"},
				deps: map[string]map[string]struct{}{
					"foo:test": {
						"bar:test": {},
					},
					"bar:test": {},
				},
			},
		},
		{
			name: "complex dependency tree",
			targetConfigs: map[string]usercfg.TargetConfig{
				"foo": {
					Dependencies: []string{"bar"},
					Pipeline: map[string]usercfg.PipelineConfig{
						"test": {
							DependsOn: []string{"^test"},
						},
					},
				},
				"bar": {
					Dependencies: []string{"baz", "quux"},
					Pipeline: map[string]usercfg.PipelineConfig{
						"test": {
							DependsOn: []string{"^test"},
						},
					},
				},
				"baz": {
					Pipeline: map[string]usercfg.PipelineConfig{
						"test": {
							DependsOn: []string{"^test"},
						},
					},
				},
				"qux": {
					Pipeline: map[string]usercfg.PipelineConfig{
						"test": {},
					},
				},
				"quux": {
					Dependencies: []string{"qux"},
					Pipeline: map[string]usercfg.PipelineConfig{
						"test": {
							DependsOn: []string{"^test"},
						},
					},
				},
			},
			tasks: []string{"test"},
			expected: expected{
				nodeIds: []string{"foo:test", "bar:test", "baz:test", "qux:test", "quux:test"},
				deps: map[string]map[string]struct{}{
					"foo:test": {
						"bar:test": struct{}{},
					},
					"bar:test": {
						"baz:test":  struct{}{},
						"quux:test": struct{}{},
					},
					"baz:test": {},
					"qux:test": {},
					"quux:test": {
						"qux:test": struct{}{},
					},
				},
			},
		},
		{
			name: "indirect circular dependency",
			targetConfigs: map[string]usercfg.TargetConfig{
				"foo": {
					Dependencies: []string{"bar"},
					Pipeline: map[string]usercfg.PipelineConfig{
						"test": {
							DependsOn: []string{"^test"},
						},
					},
				},
				"bar": {
					Dependencies: []string{"baz"},
					Pipeline: map[string]usercfg.PipelineConfig{
						"test": {
							DependsOn: []string{"^test"},
						},
					},
				},
				"baz": {
					Dependencies: []string{"foo"},
					Pipeline: map[string]usercfg.PipelineConfig{
						"test": {
							DependsOn: []string{"^test"},
						},
					},
				},
			},
			tasks: []string{"test"},
			expected: expected{
				err: true,
			},
		},
		{
			name: "no tasks matching inputs",
			targetConfigs: map[string]usercfg.TargetConfig{
				"foo": {
					Pipeline: map[string]usercfg.PipelineConfig{
						"test": {},
					},
				},
			},
			tasks: []string{"build"},
			expected: expected{
				err: true,
			},
		},
		{
			name: "invalid task name",
			targetConfigs: map[string]usercfg.TargetConfig{
				"foo": {
					Pipeline: map[string]usercfg.PipelineConfig{
						"has:colon": {},
					},
				},
			},
			tasks: []string{"has:colon"},
			expected: expected{
				err: true,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			graph := graph.NewDependencyGraph(executor{}, tc.targetConfigs)
			err := graph.PopulateNodes(tc.tasks, tc.dir)

			if tc.expected.err {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("expected nil, got %v", err)
			}

			nodeIds := make([]string, 0, len(graph.Nodes))
			for id := range graph.Nodes {
				nodeIds = append(nodeIds, id)
			}

			slices.Sort(tc.expected.nodeIds)
			slices.Sort(nodeIds)

			if !reflect.DeepEqual(tc.expected.nodeIds, nodeIds) {
				t.Errorf("expected %v, got %v", tc.expected.nodeIds, nodeIds)
			}

			if !reflect.DeepEqual(tc.expected.deps, graph.Dependencies) {
				t.Errorf("expected %v, got %v", tc.expected.deps, graph.Dependencies)
			}
		})
	}
}

func TestFormat(t *testing.T) {
	testCases := []struct {
		name          string
		targetConfigs map[string]usercfg.TargetConfig
		tasks         []string
		expected      map[string]interface{}
	}{
		{
			name: "one config with zero dependencies",
			targetConfigs: map[string]usercfg.TargetConfig{
				"foo": {
					Pipeline: map[string]usercfg.PipelineConfig{
						"test":  {},
						"build": {},
					},
				},
			},
			tasks: []string{"test", "build"},
			expected: map[string]interface{}{
				"foo:test":  map[string]interface{}{},
				"foo:build": map[string]interface{}{},
			},
		},
		{
			name: "two configs with one ancestor dependency",
			targetConfigs: map[string]usercfg.TargetConfig{
				"foo": {
					Dependencies: []string{"bar"},
					Pipeline: map[string]usercfg.PipelineConfig{
						"test": {
							DependsOn: []string{"^test"},
						},
					},
				},
				"bar": {
					Pipeline: map[string]usercfg.PipelineConfig{
						"test": {},
					},
				},
			},
			tasks: []string{"test"},
			expected: map[string]interface{}{
				"foo:test": map[string]interface{}{
					"bar:test": map[string]interface{}{},
				},
			},
		},
		{
			name: "complex dependency tree",
			targetConfigs: map[string]usercfg.TargetConfig{
				"foo": {
					Dependencies: []string{"bar"},
					Pipeline: map[string]usercfg.PipelineConfig{
						"test": {
							DependsOn: []string{"^test"},
						},
					},
				},
				"bar": {
					Dependencies: []string{"baz", "quux"},
					Pipeline: map[string]usercfg.PipelineConfig{
						"test": {
							DependsOn: []string{"^test"},
						},
					},
				},
				"baz": {
					Pipeline: map[string]usercfg.PipelineConfig{
						"test": {},
					},
				},
				"qux": {
					Dependencies: []string{"corge"},
					Pipeline: map[string]usercfg.PipelineConfig{
						"test": {
							DependsOn: []string{"^test"},
						},
					},
				},
				"quux": {
					Pipeline: map[string]usercfg.PipelineConfig{
						"test": {},
					},
				},
				"corge": {
					Pipeline: map[string]usercfg.PipelineConfig{
						"test": {},
					},
				},
			},
			tasks: []string{"test"},
			expected: map[string]interface{}{
				"foo:test": map[string]interface{}{
					"bar:test": map[string]interface{}{
						"baz:test":  map[string]interface{}{},
						"quux:test": map[string]interface{}{},
					},
				},
				"qux:test": map[string]interface{}{
					"corge:test": map[string]interface{}{},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			graph := graph.NewDependencyGraph(executor{}, tc.targetConfigs)
			if err := graph.PopulateNodes(tc.tasks, ""); err != nil {
				t.Fatalf("expected nil, got %v", err)
			}

			tree := graph.ToMap()
			if !reflect.DeepEqual(tc.expected, tree) {
				t.Fatalf("expected %v, got %v", tc.expected, tree)
			}
		})
	}
}
