package graph

import (
	"fmt"
	"sync"

	"github.com/mitchelldw01/omnirepo/usercfg"
)

type Node struct {
	Id       string
	Name     string
	Dir      string
	Pipeline usercfg.PipelineConfig
	mutex    sync.RWMutex
	indegree int
}

func NewNode(name, dir string, pl usercfg.PipelineConfig) *Node {
	return &Node{
		Id:       fmt.Sprintf("%s:%s", dir, name),
		Name:     name,
		Dir:      dir,
		Pipeline: pl,
		mutex:    sync.RWMutex{},
	}
}

func (node *Node) getIndegree() int {
	node.mutex.RLock()
	indegree := node.indegree
	node.mutex.RUnlock()
	return indegree
}

func (n *Node) incrementIndegree() {
	n.mutex.Lock()
	n.indegree += 1
	n.mutex.Unlock()
}

func (node *Node) decrementIndegree() {
	node.mutex.Lock()
	node.indegree -= 1
	node.mutex.Unlock()
}
