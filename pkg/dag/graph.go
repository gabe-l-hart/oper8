package dag

import (
	"fmt"
	"sync"
)

// Graph is a generic directed acyclic graph (DAG) that can hold any node type
// implementing the Node interface. It uses Go 1.21+ generics for type safety.
type Graph[T Node] struct {
	root     *NodeWrapper[T]
	nodeDict map[string]*NodeWrapper[T]
	mu       sync.RWMutex
}

// NewGraph creates a new empty graph.
func NewGraph[T Node]() *Graph[T] {
	// Create a zero-value node for the root
	// The root connects to all top-level nodes
	var zeroNode T
	root := newNodeWrapper(zeroNode)

	return &Graph[T]{
		root:     root,
		nodeDict: make(map[string]*NodeWrapper[T]),
	}
}

// AddNode adds a node to the graph. The node is initially added as a child
// of the root, making it a top-level node. Dependencies can be added later
// using AddEdge.
//
// Returns an error if a node with the same name already exists.
func (g *Graph[T]) AddNode(node T) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	name := node.GetName()
	if _, exists := g.nodeDict[name]; exists {
		return fmt.Errorf("node %s already exists in graph", name)
	}

	wrapper := newNodeWrapper(node)
	g.nodeDict[name] = wrapper

	// Add as child of root (top-level node)
	g.root.children[wrapper] = nil

	return nil
}

// AddEdge adds a directed edge from parent to child, with an optional verification function.
// The verification function is called to check if the dependency is satisfied.
//
// This establishes that 'child' depends on 'parent', meaning 'parent' must be
// processed before 'child' in topological order.
//
// Returns an error if either node doesn't exist or if adding the edge would create a cycle.
func (g *Graph[T]) AddEdge(parent, child T, verifyFn EdgeFunc) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	parentName := parent.GetName()
	childName := child.GetName()

	parentWrapper, parentExists := g.nodeDict[parentName]
	if !parentExists {
		return fmt.Errorf("parent node %s not found in graph", parentName)
	}

	childWrapper, childExists := g.nodeDict[childName]
	if !childExists {
		return fmt.Errorf("child node %s not found in graph", childName)
	}

	// Add the edge (includes cycle detection)
	if err := parentWrapper.AddChild(childWrapper, verifyFn); err != nil {
		return err
	}

	return nil
}

// GetNode retrieves a node by name.
func (g *Graph[T]) GetNode(name string) (T, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	wrapper, exists := g.nodeDict[name]
	if !exists {
		var zero T
		return zero, false
	}

	return wrapper.node, true
}

// GetAllNodes returns all nodes in the graph in no particular order.
func (g *Graph[T]) GetAllNodes() []T {
	g.mu.RLock()
	defer g.mu.RUnlock()

	nodes := make([]T, 0, len(g.nodeDict))
	for _, wrapper := range g.nodeDict {
		nodes = append(nodes, wrapper.node)
	}

	return nodes
}

// Topology returns all nodes in topological order (dependencies before dependents).
// Nodes with no dependencies come first.
func (g *Graph[T]) Topology() []T {
	g.mu.RLock()
	defer g.mu.RUnlock()

	// Get topology starting from root
	topology := g.root.Topology()

	// Remove the zero-value root node from results
	// (it's just a placeholder to connect top-level nodes)
	if len(topology) > 0 {
		// The root will be the last element in post-order traversal
		// Filter it out by checking for zero value
		var zero T
		filtered := make([]T, 0, len(topology))
		for _, node := range topology {
			if node != zero {
				filtered = append(filtered, node)
			}
		}
		return filtered
	}

	return topology
}

// GetChildren returns the direct children of a node (nodes that depend on it).
func (g *Graph[T]) GetChildren(parent T) ([]T, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	parentName := parent.GetName()
	parentWrapper, exists := g.nodeDict[parentName]
	if !exists {
		return nil, fmt.Errorf("node %s not found in graph", parentName)
	}

	children := make([]T, 0, len(parentWrapper.children))
	for childWrapper := range parentWrapper.children {
		children = append(children, childWrapper.node)
	}

	return children, nil
}

// GetEdgeFunc returns the verification function for the edge from parent to child.
func (g *Graph[T]) GetEdgeFunc(parent, child T) (EdgeFunc, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	parentName := parent.GetName()
	childName := child.GetName()

	parentWrapper, parentExists := g.nodeDict[parentName]
	if !parentExists {
		return nil, fmt.Errorf("parent node %s not found in graph", parentName)
	}

	childWrapper, childExists := g.nodeDict[childName]
	if !childExists {
		return nil, fmt.Errorf("child node %s not found in graph", childName)
	}

	verifyFn, hasEdge := parentWrapper.children[childWrapper]
	if !hasEdge {
		return nil, fmt.Errorf("no edge from %s to %s", parentName, childName)
	}

	return verifyFn, nil
}

// NodeCount returns the number of nodes in the graph.
func (g *Graph[T]) NodeCount() int {
	g.mu.RLock()
	defer g.mu.RUnlock()

	return len(g.nodeDict)
}
