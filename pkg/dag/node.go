package dag

import (
	"context"
	"fmt"
	"sort"
)

// Node represents a node in the DAG.
// Types implementing Node can be used in a generic Graph[T Node].
type Node interface {
	GetName() string
	comparable
}

// EdgeFunc is a function that verifies a dependency is satisfied.
// It returns (satisfied bool, error). If satisfied is false, the dependent
// node cannot proceed. If error is non-nil, it indicates a verification failure.
type EdgeFunc func(context.Context) (bool, error)

// NodeWrapper wraps a node with its outgoing edges (children).
// This internal type manages the graph structure.
type NodeWrapper[T Node] struct {
	node     T
	children map[*NodeWrapper[T]]EdgeFunc
}

// newNodeWrapper creates a new NodeWrapper for the given node.
func newNodeWrapper[T Node](node T) *NodeWrapper[T] {
	return &NodeWrapper[T]{
		node:     node,
		children: make(map[*NodeWrapper[T]]EdgeFunc),
	}
}

// GetNode returns the wrapped node.
func (nw *NodeWrapper[T]) GetNode() T {
	return nw.node
}

// GetChildren returns all child node wrappers and their edge functions.
func (nw *NodeWrapper[T]) GetChildren() map[*NodeWrapper[T]]EdgeFunc {
	return nw.children
}

// AddChild adds a child node with an optional verification function.
// Returns an error if adding this child would create a cycle.
func (nw *NodeWrapper[T]) AddChild(child *NodeWrapper[T], verifyFn EdgeFunc) error {
	// Check for cycles: if child already has a path to self, adding this edge creates a cycle
	if child.dfs(nw) {
		return fmt.Errorf("unable to add cyclic dependency: adding %s -> %s would create a cycle",
			nw.node.GetName(), child.node.GetName())
	}

	nw.children[child] = verifyFn
	return nil
}

// dfs performs depth-first search to check if there's a path from self to target.
// Used for cycle detection.
func (nw *NodeWrapper[T]) dfs(target *NodeWrapper[T]) bool {
	if nw == target {
		return true
	}

	for child := range nw.children {
		if child.dfs(target) {
			return true
		}
	}

	return false
}

// Topology returns the nodes in topological order using post-order DFS.
// Nodes with no dependencies come first, and nodes depending on others come after.
func (nw *NodeWrapper[T]) Topology() []T {
	visited := make(map[*NodeWrapper[T]]bool)
	var result []T

	var visit func(*NodeWrapper[T])
	visit = func(n *NodeWrapper[T]) {
		// Visit children first (post-order)
		// Sort children by name for deterministic ordering
		var sortedChildren []*NodeWrapper[T]
		for child := range n.children {
			sortedChildren = append(sortedChildren, child)
		}
		sort.Slice(sortedChildren, func(i, j int) bool {
			return sortedChildren[i].node.GetName() < sortedChildren[j].node.GetName()
		})

		for _, child := range sortedChildren {
			if !visited[child] {
				visit(child)
			}
		}

		// Add this node if not already visited
		if !visited[n] {
			result = append(result, n.node)
			visited[n] = true
		}
	}

	visit(nw)
	return result
}
