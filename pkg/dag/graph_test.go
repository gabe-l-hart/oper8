package dag

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestNode is a simple node for testing
type TestNode struct {
	name string
}

func (tn TestNode) GetName() string {
	return tn.name
}

func TestNewGraph(t *testing.T) {
	g := NewGraph[TestNode]()

	require.NotNil(t, g)
	require.NotNil(t, g.root)
	require.NotNil(t, g.nodeDict)
	require.Equal(t, 0, g.NodeCount())
}

func TestGraph_AddNode(t *testing.T) {
	tests := []struct {
		name    string
		nodes   []TestNode
		wantErr bool
		errMsg  string
	}{
		{
			name:    "add single node",
			nodes:   []TestNode{{name: "node1"}},
			wantErr: false,
		},
		{
			name:    "add multiple nodes",
			nodes:   []TestNode{{name: "node1"}, {name: "node2"}, {name: "node3"}},
			wantErr: false,
		},
		{
			name:    "add duplicate node",
			nodes:   []TestNode{{name: "node1"}, {name: "node1"}},
			wantErr: true,
			errMsg:  "node node1 already exists in graph",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGraph[TestNode]()

			var err error
			for _, node := range tt.nodes {
				err = g.AddNode(node)
				if err != nil {
					break
				}
			}

			if tt.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
				require.Equal(t, len(tt.nodes), g.NodeCount())
			}
		})
	}
}

func TestGraph_AddEdge(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(*Graph[TestNode]) (parent, child TestNode)
		wantErr    bool
		errContains string
	}{
		{
			name: "add edge between existing nodes",
			setup: func(g *Graph[TestNode]) (TestNode, TestNode) {
				parent := TestNode{name: "parent"}
				child := TestNode{name: "child"}
				g.AddNode(parent)
				g.AddNode(child)
				return parent, child
			},
			wantErr: false,
		},
		{
			name: "parent not found",
			setup: func(g *Graph[TestNode]) (TestNode, TestNode) {
				child := TestNode{name: "child"}
				g.AddNode(child)
				return TestNode{name: "missing"}, child
			},
			wantErr:     true,
			errContains: "parent node missing not found",
		},
		{
			name: "child not found",
			setup: func(g *Graph[TestNode]) (TestNode, TestNode) {
				parent := TestNode{name: "parent"}
				g.AddNode(parent)
				return parent, TestNode{name: "missing"}
			},
			wantErr:     true,
			errContains: "child node missing not found",
		},
		{
			name: "cycle detection - direct cycle",
			setup: func(g *Graph[TestNode]) (TestNode, TestNode) {
				nodeA := TestNode{name: "A"}
				nodeB := TestNode{name: "B"}
				g.AddNode(nodeA)
				g.AddNode(nodeB)
				// A -> B, then try B -> A (cycle)
				g.AddEdge(nodeA, nodeB, nil)
				return nodeB, nodeA
			},
			wantErr:     true,
			errContains: "cycle",
		},
		{
			name: "cycle detection - indirect cycle",
			setup: func(g *Graph[TestNode]) (TestNode, TestNode) {
				nodeA := TestNode{name: "A"}
				nodeB := TestNode{name: "B"}
				nodeC := TestNode{name: "C"}
				g.AddNode(nodeA)
				g.AddNode(nodeB)
				g.AddNode(nodeC)
				// A -> B -> C, then try C -> A (cycle)
				g.AddEdge(nodeA, nodeB, nil)
				g.AddEdge(nodeB, nodeC, nil)
				return nodeC, nodeA
			},
			wantErr:     true,
			errContains: "cycle",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGraph[TestNode]()
			parent, child := tt.setup(g)

			err := g.AddEdge(parent, child, nil)

			if tt.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errContains)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestGraph_AddEdge_WithVerifyFunction(t *testing.T) {
	g := NewGraph[TestNode]()
	parent := TestNode{name: "parent"}
	child := TestNode{name: "child"}

	g.AddNode(parent)
	g.AddNode(child)

	called := false
	verifyFn := func(ctx context.Context) (bool, error) {
		called = true
		return true, nil
	}

	err := g.AddEdge(parent, child, verifyFn)
	require.NoError(t, err)

	// Verify the edge function was stored
	edgeFn, err := g.GetEdgeFunc(parent, child)
	require.NoError(t, err)
	require.NotNil(t, edgeFn)

	// Call it to verify it's the right function
	success, err := edgeFn(context.Background())
	require.NoError(t, err)
	require.True(t, success)
	require.True(t, called)
}

func TestGraph_GetNode(t *testing.T) {
	g := NewGraph[TestNode]()
	node := TestNode{name: "test-node"}
	g.AddNode(node)

	t.Run("node exists", func(t *testing.T) {
		found, exists := g.GetNode("test-node")
		require.True(t, exists)
		require.Equal(t, "test-node", found.GetName())
	})

	t.Run("node does not exist", func(t *testing.T) {
		found, exists := g.GetNode("missing")
		require.False(t, exists)
		require.Equal(t, "", found.GetName()) // zero value
	})
}

func TestGraph_GetAllNodes(t *testing.T) {
	g := NewGraph[TestNode]()

	t.Run("empty graph", func(t *testing.T) {
		nodes := g.GetAllNodes()
		require.Empty(t, nodes)
	})

	t.Run("multiple nodes", func(t *testing.T) {
		g.AddNode(TestNode{name: "node1"})
		g.AddNode(TestNode{name: "node2"})
		g.AddNode(TestNode{name: "node3"})

		nodes := g.GetAllNodes()
		require.Len(t, nodes, 3)

		// Verify all nodes are present
		names := make(map[string]bool)
		for _, node := range nodes {
			names[node.GetName()] = true
		}
		require.True(t, names["node1"])
		require.True(t, names["node2"])
		require.True(t, names["node3"])
	})
}

func TestGraph_Topology(t *testing.T) {
	t.Run("empty graph", func(t *testing.T) {
		g := NewGraph[TestNode]()
		topology := g.Topology()
		require.Empty(t, topology)
	})

	t.Run("single node", func(t *testing.T) {
		g := NewGraph[TestNode]()
		node := TestNode{name: "node1"}
		g.AddNode(node)

		topology := g.Topology()
		require.Len(t, topology, 1)
		require.Equal(t, "node1", topology[0].GetName())
	})

	t.Run("linear dependency chain", func(t *testing.T) {
		g := NewGraph[TestNode]()
		nodeA := TestNode{name: "A"}
		nodeB := TestNode{name: "B"}
		nodeC := TestNode{name: "C"}

		g.AddNode(nodeA)
		g.AddNode(nodeB)
		g.AddNode(nodeC)

		// A -> B -> C (A depends on B, B depends on C)
		g.AddEdge(nodeB, nodeA, nil)
		g.AddEdge(nodeC, nodeB, nil)

		topology := g.Topology()
		require.Len(t, topology, 3)

		// C should come before B, B should come before A
		positions := make(map[string]int)
		for i, node := range topology {
			positions[node.GetName()] = i
			t.Logf("Position %d: %s", i, node.GetName())
		}

		require.Less(t, positions["C"], positions["B"], "C should come before B")
		require.Less(t, positions["B"], positions["A"], "B should come before A")
	})

	t.Run("diamond dependency", func(t *testing.T) {
		g := NewGraph[TestNode]()
		nodeA := TestNode{name: "A"}
		nodeB := TestNode{name: "B"}
		nodeC := TestNode{name: "C"}
		nodeD := TestNode{name: "D"}

		g.AddNode(nodeA)
		g.AddNode(nodeB)
		g.AddNode(nodeC)
		g.AddNode(nodeD)

		// Diamond: D -> B, D -> C, B -> A, C -> A
		g.AddEdge(nodeD, nodeB, nil)
		g.AddEdge(nodeD, nodeC, nil)
		g.AddEdge(nodeB, nodeA, nil)
		g.AddEdge(nodeC, nodeA, nil)

		topology := g.Topology()
		require.Len(t, topology, 4)

		positions := make(map[string]int)
		for i, node := range topology {
			positions[node.GetName()] = i
		}

		// D should come before B and C
		require.Less(t, positions["D"], positions["B"])
		require.Less(t, positions["D"], positions["C"])
		// B and C should come before A
		require.Less(t, positions["B"], positions["A"])
		require.Less(t, positions["C"], positions["A"])
	})
}

func TestGraph_GetChildren(t *testing.T) {
	g := NewGraph[TestNode]()
	parent := TestNode{name: "parent"}
	child1 := TestNode{name: "child1"}
	child2 := TestNode{name: "child2"}

	g.AddNode(parent)
	g.AddNode(child1)
	g.AddNode(child2)

	g.AddEdge(parent, child1, nil)
	g.AddEdge(parent, child2, nil)

	t.Run("get children of existing node", func(t *testing.T) {
		children, err := g.GetChildren(parent)
		require.NoError(t, err)
		require.Len(t, children, 2)

		names := make(map[string]bool)
		for _, child := range children {
			names[child.GetName()] = true
		}
		require.True(t, names["child1"])
		require.True(t, names["child2"])
	})

	t.Run("get children of node with no children", func(t *testing.T) {
		children, err := g.GetChildren(child1)
		require.NoError(t, err)
		require.Empty(t, children)
	})

	t.Run("get children of non-existent node", func(t *testing.T) {
		missing := TestNode{name: "missing"}
		children, err := g.GetChildren(missing)
		require.Error(t, err)
		require.Nil(t, children)
		require.Contains(t, err.Error(), "not found")
	})
}

func TestGraph_GetEdgeFunc(t *testing.T) {
	g := NewGraph[TestNode]()
	parent := TestNode{name: "parent"}
	child := TestNode{name: "child"}

	g.AddNode(parent)
	g.AddNode(child)

	t.Run("edge with no function", func(t *testing.T) {
		g.AddEdge(parent, child, nil)

		fn, err := g.GetEdgeFunc(parent, child)
		require.NoError(t, err)
		require.Nil(t, fn)
	})

	t.Run("edge with function", func(t *testing.T) {
		g2 := NewGraph[TestNode]()
		parent2 := TestNode{name: "p"}
		child2 := TestNode{name: "c"}
		g2.AddNode(parent2)
		g2.AddNode(child2)

		verifyFn := func(ctx context.Context) (bool, error) {
			return true, nil
		}
		g2.AddEdge(parent2, child2, verifyFn)

		fn, err := g2.GetEdgeFunc(parent2, child2)
		require.NoError(t, err)
		require.NotNil(t, fn)
	})

	t.Run("no edge between nodes", func(t *testing.T) {
		g3 := NewGraph[TestNode]()
		node1 := TestNode{name: "n1"}
		node2 := TestNode{name: "n2"}
		g3.AddNode(node1)
		g3.AddNode(node2)
		// No edge added

		fn, err := g3.GetEdgeFunc(node1, node2)
		require.Error(t, err)
		require.Nil(t, fn)
		require.Contains(t, err.Error(), "no edge")
	})

	t.Run("parent not found", func(t *testing.T) {
		g4 := NewGraph[TestNode]()
		child4 := TestNode{name: "c"}
		g4.AddNode(child4)

		fn, err := g4.GetEdgeFunc(TestNode{name: "missing"}, child4)
		require.Error(t, err)
		require.Nil(t, fn)
	})

	t.Run("child not found", func(t *testing.T) {
		g5 := NewGraph[TestNode]()
		parent5 := TestNode{name: "p"}
		g5.AddNode(parent5)

		fn, err := g5.GetEdgeFunc(parent5, TestNode{name: "missing"})
		require.Error(t, err)
		require.Nil(t, fn)
	})
}

func TestGraph_NodeCount(t *testing.T) {
	g := NewGraph[TestNode]()

	require.Equal(t, 0, g.NodeCount())

	g.AddNode(TestNode{name: "node1"})
	require.Equal(t, 1, g.NodeCount())

	g.AddNode(TestNode{name: "node2"})
	require.Equal(t, 2, g.NodeCount())

	g.AddNode(TestNode{name: "node3"})
	require.Equal(t, 3, g.NodeCount())
}

func TestGraph_ThreadSafety(t *testing.T) {
	// This test verifies that the graph can handle concurrent operations
	g := NewGraph[TestNode]()

	// Add nodes concurrently
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			node := TestNode{name: string(rune('A' + id))}
			g.AddNode(node)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify all nodes were added
	require.Equal(t, 10, g.NodeCount())
}
