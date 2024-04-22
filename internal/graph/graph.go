package graph

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/mitchelldw01/omnirepo/usercfg"
)

// Both executes task commands and interfaces with the cache.
type Executor interface {
	ExecuteTask(node *Node, deps map[string]struct{})
	CleanUp(t time.Time)
}

type DependencyGraph struct {
	Nodes map[string]*Node
	// Map of node IDs to the node IDs of dependencies
	Dependencies  map[string]map[string]struct{}
	executor      Executor
	targetConfigs map[string]usercfg.TargetConfig
}

func NewDependencyGraph(ex Executor, cfgs map[string]usercfg.TargetConfig) *DependencyGraph {
	return &DependencyGraph{
		Nodes:         map[string]*Node{},
		Dependencies:  map[string]map[string]struct{}{},
		executor:      ex,
		targetConfigs: cfgs,
	}
}

func (dg *DependencyGraph) PopulateNodes(tasks []string, target string) error {
	for dir, cfg := range dg.targetConfigs {
		if target != "" && filepath.Clean(target) != dir {
			continue
		}

		for _, t := range tasks {
			if err := dg.populateFromTaskName(t, dir, cfg); err != nil {
				return err
			}
		}
	}

	return dg.validateNodes()
}

func (dg *DependencyGraph) populateFromTaskName(task, dir string, cfg usercfg.TargetConfig) error {
	pl, ok := cfg.Pipeline[task]
	if !ok {
		return nil
	}

	node := NewNode(task, dir, pl)
	if _, ok := dg.Nodes[node.Id]; ok {
		return nil
	}
	dg.Nodes[node.Id] = node
	dg.Dependencies[node.Id] = map[string]struct{}{}

	return dg.populateDependencies(node, cfg)
}

func (dg *DependencyGraph) populateDependencies(node *Node, cfg usercfg.TargetConfig) error {
	for _, pattern := range node.Pipeline.DependsOn {
		if !strings.HasPrefix(pattern, "^") {
			if err := dg.populateDependency(node, pattern, node.Dir); err != nil {
				return err
			}
		}

		task := pattern[1:]
		for _, dir := range cfg.Dependencies {
			if err := dg.populateDependency(node, task, filepath.Clean(dir)); err != nil {
				return err
			}
		}
	}

	return nil
}

func (dg *DependencyGraph) populateDependency(prevNode *Node, task, dir string) error {
	depCfg, ok := dg.targetConfigs[dir]
	if !ok {
		return fmt.Errorf("dependency on target %q that does not exist", dir)
	}
	pl, ok := depCfg.Pipeline[task]
	if !ok {
		return nil
	}

	depNode := dg.addDependencyNode(prevNode, task, dir, pl)
	if err := dg.checkCircularDependency(prevNode.Id, dg.Dependencies[prevNode.Id]); err != nil {
		return err
	}

	return dg.populateDependencies(depNode, depCfg)
}

func (dg *DependencyGraph) addDependencyNode(prevNode *Node, task, dir string, pl usercfg.PipelineConfig) *Node {
	depNode := NewNode(task, dir, pl)
	dg.Nodes[depNode.Id] = depNode

	if _, ok := dg.Dependencies[depNode.Id]; !ok {
		dg.Dependencies[depNode.Id] = map[string]struct{}{}
	}
	dg.Dependencies[prevNode.Id][depNode.Id] = struct{}{}
	prevNode.incrementIndegree()

	return depNode
}

func (dg *DependencyGraph) checkCircularDependency(id string, deps map[string]struct{}) error {
	for depId := range deps {
		if depId == id {
			return fmt.Errorf("circular dependency detected in task %q", id)
		}

		if err := dg.checkCircularDependency(id, dg.Dependencies[depId]); err != nil {
			return err
		}
	}

	return nil
}

func (dg *DependencyGraph) validateNodes() error {
	if len(dg.Nodes) == 0 {
		return errors.New("no tasks were found to process")
	}

	for _, node := range dg.Nodes {
		if strings.Contains(node.Name, ":") {
			return fmt.Errorf("invalid task name %q, task names cannot contain contain colons", node.Name)
		}
	}

	return nil
}
