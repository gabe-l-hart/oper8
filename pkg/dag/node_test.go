package dag

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_newNodeWrapper(t *testing.T) {
	node := TestNode{name: "test"}
	wrapper := newNodeWrapper(node)

	require.NotNil(t, wrapper)
	require.Equal(t, "test", wrapper.node.GetName())
	require.NotNil(t, wrapper.children)
	require.Empty(t, wrapper.children)
}

func TestNodeWrapper_GetNode(t *testing.T) {
	node := TestNode{name: "my-node"}
	wrapper := newNodeWrapper(node)

	got := wrapper.GetNode()
	require.Equal(t, "my-node", got.GetName())
}

func TestNodeWrapper_GetChildren(t *testing.T) {
	parent := newNodeWrapper(TestNode{name: "parent"})
	child1 := newNodeWrapper(TestNode{name: "child1"})
	child2 := newNodeWrapper(TestNode{name: "child2"})

	t.Run("no children", func(t *testing.T) {
		children := parent.GetChildren()
		require.Empty(t, children)
	})

	t.Run("with children", func(t *testing.T) {
		parent.AddChild(child1, nil)
		parent.AddChild(child2, nil)

		children := parent.GetChildren()
		require.Len(t, children, 2)
		require.Contains(t, children, child1)
		require.Contains(t, children, child2)
	})
}

func TestNodeWrapper_AddChild(t *testing.T) {
	t.Run("add child successfully", func(t *testing.T) {
		parent := newNodeWrapper(TestNode{name: "parent"})
		child := newNodeWrapper(TestNode{name: "child"})

		err := parent.AddChild(child, nil)
		require.NoError(t, err)

		children := parent.GetChildren()
		require.Len(t, children, 1)
		require.Contains(t, children, child)
	})

	t.Run("add child with verify function", func(t *testing.T) {
		parent := newNodeWrapper(TestNode{name: "parent"})
		child := newNodeWrapper(TestNode{name: "child"})

		called := false
		verifyFn := func(ctx context.Context) (bool, error) {
			called = true
			return true, nil
		}

		err := parent.AddChild(child, verifyFn)
		require.NoError(t, err)

		// Verify the function was stored
		children := parent.GetChildren()
		fn := children[child]
		require.NotNil(t, fn)

		// Call it
		success, err := fn(context.Background())
		require.NoError(t, err)
		require.True(t, success)
		require.True(t, called)
	})

	t.Run("detect direct cycle", func(t *testing.T) {
		nodeA := newNodeWrapper(TestNode{name: "A"})
		nodeB := newNodeWrapper(TestNode{name: "B"})

		// A -> B is OK
		err := nodeA.AddChild(nodeB, nil)
		require.NoError(t, err)

		// B -> A creates a cycle
		err = nodeB.AddChild(nodeA, nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "cycle")
		require.Contains(t, err.Error(), "B -> A")
	})

	t.Run("detect indirect cycle", func(t *testing.T) {
		nodeA := newNodeWrapper(TestNode{name: "A"})
		nodeB := newNodeWrapper(TestNode{name: "B"})
		nodeC := newNodeWrapper(TestNode{name: "C"})

		// A -> B -> C is OK
		nodeA.AddChild(nodeB, nil)
		nodeB.AddChild(nodeC, nil)

		// C -> A creates a cycle
		err := nodeC.AddChild(nodeA, nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "cycle")
	})

	t.Run("allow diamond pattern (not a cycle)", func(t *testing.T) {
		nodeA := newNodeWrapper(TestNode{name: "A"})
		nodeB := newNodeWrapper(TestNode{name: "B"})
		nodeC := newNodeWrapper(TestNode{name: "C"})
		nodeD := newNodeWrapper(TestNode{name: "D"})

		// Diamond: A -> B, A -> C, B -> D, C -> D
		// This is NOT a cycle (D is reachable from A via two paths)
		err := nodeA.AddChild(nodeB, nil)
		require.NoError(t, err)

		err = nodeA.AddChild(nodeC, nil)
		require.NoError(t, err)

		err = nodeB.AddChild(nodeD, nil)
		require.NoError(t, err)

		err = nodeC.AddChild(nodeD, nil)
		require.NoError(t, err)

		// Verify D is not a child of itself
		err = nodeD.AddChild(nodeD, nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "cycle")
	})
}

func TestNodeWrapper_dfs(t *testing.T) {
	t.Run("direct connection", func(t *testing.T) {
		nodeA := newNodeWrapper(TestNode{name: "A"})
		nodeB := newNodeWrapper(TestNode{name: "B"})

		nodeA.AddChild(nodeB, nil)

		// A can reach B
		require.True(t, nodeA.dfs(nodeB))
		// B cannot reach A
		require.False(t, nodeB.dfs(nodeA))
	})

	t.Run("indirect connection", func(t *testing.T) {
		nodeA := newNodeWrapper(TestNode{name: "A"})
		nodeB := newNodeWrapper(TestNode{name: "B"})
		nodeC := newNodeWrapper(TestNode{name: "C"})

		nodeA.AddChild(nodeB, nil)
		nodeB.AddChild(nodeC, nil)

		// A can reach C (via B)
		require.True(t, nodeA.dfs(nodeC))
		// C cannot reach A
		require.False(t, nodeC.dfs(nodeA))
	})

	t.Run("self reference", func(t *testing.T) {
		nodeA := newNodeWrapper(TestNode{name: "A"})

		// Node can reach itself
		require.True(t, nodeA.dfs(nodeA))
	})

	t.Run("no connection", func(t *testing.T) {
		nodeA := newNodeWrapper(TestNode{name: "A"})
		nodeB := newNodeWrapper(TestNode{name: "B"})

		// No edge between them
		require.False(t, nodeA.dfs(nodeB))
		require.False(t, nodeB.dfs(nodeA))
	})
}

func TestNodeWrapper_Topology(t *testing.T) {
	t.Run("single node", func(t *testing.T) {
		node := newNodeWrapper(TestNode{name: "A"})
		topology := node.Topology()

		require.Len(t, topology, 1)
		require.Equal(t, "A", topology[0].GetName())
	})

	t.Run("linear chain", func(t *testing.T) {
		nodeA := newNodeWrapper(TestNode{name: "A"})
		nodeB := newNodeWrapper(TestNode{name: "B"})
		nodeC := newNodeWrapper(TestNode{name: "C"})

		// A -> B -> C
		nodeA.AddChild(nodeB, nil)
		nodeB.AddChild(nodeC, nil)

		topology := nodeA.Topology()

		require.Len(t, topology, 3)
		// Post-order: C, B, A
		require.Equal(t, "C", topology[0].GetName())
		require.Equal(t, "B", topology[1].GetName())
		require.Equal(t, "A", topology[2].GetName())
	})

	t.Run("multiple children", func(t *testing.T) {
		nodeA := newNodeWrapper(TestNode{name: "A"})
		nodeB := newNodeWrapper(TestNode{name: "B"})
		nodeC := newNodeWrapper(TestNode{name: "C"})

		// A -> B, A -> C
		nodeA.AddChild(nodeB, nil)
		nodeA.AddChild(nodeC, nil)

		topology := nodeA.Topology()

		require.Len(t, topology, 3)
		// Post-order, with B and C before A
		// The exact order of B and C depends on sorting
		positions := make(map[string]int)
		for i, node := range topology {
			positions[node.GetName()] = i
		}

		// A should be last
		require.Equal(t, 2, positions["A"])
		// B and C should be before A
		require.Less(t, positions["B"], positions["A"])
		require.Less(t, positions["C"], positions["A"])
	})

	t.Run("diamond pattern", func(t *testing.T) {
		nodeA := newNodeWrapper(TestNode{name: "A"})
		nodeB := newNodeWrapper(TestNode{name: "B"})
		nodeC := newNodeWrapper(TestNode{name: "C"})
		nodeD := newNodeWrapper(TestNode{name: "D"})

		// A -> B -> D
		// A -> C -> D (diamond)
		nodeA.AddChild(nodeB, nil)
		nodeA.AddChild(nodeC, nil)
		nodeB.AddChild(nodeD, nil)
		nodeC.AddChild(nodeD, nil)

		topology := nodeA.Topology()

		// All 4 nodes should be present
		require.Len(t, topology, 4)

		positions := make(map[string]int)
		for i, node := range topology {
			positions[node.GetName()] = i
		}

		// D should come before B and C (it's their child)
		require.Less(t, positions["D"], positions["B"])
		require.Less(t, positions["D"], positions["C"])
		// B and C should come before A
		require.Less(t, positions["B"], positions["A"])
		require.Less(t, positions["C"], positions["A"])
		// A should be last
		require.Equal(t, 3, positions["A"])
	})

	t.Run("deterministic ordering", func(t *testing.T) {
		// Test that sorting provides deterministic results
		nodeA := newNodeWrapper(TestNode{name: "A"})
		nodeB := newNodeWrapper(TestNode{name: "B"})
		nodeC := newNodeWrapper(TestNode{name: "C"})
		nodeD := newNodeWrapper(TestNode{name: "D"})

		// A has children B, C, D (added in that order)
		nodeA.AddChild(nodeB, nil)
		nodeA.AddChild(nodeC, nil)
		nodeA.AddChild(nodeD, nil)

		topology1 := nodeA.Topology()
		topology2 := nodeA.Topology()

		// Should be identical
		require.Equal(t, len(topology1), len(topology2))
		for i := range topology1 {
			require.Equal(t, topology1[i].GetName(), topology2[i].GetName())
		}

		// Verify alphabetical ordering for children at same level
		// B, C, D should come before A, and in alphabetical order
		require.Equal(t, "B", topology1[0].GetName())
		require.Equal(t, "C", topology1[1].GetName())
		require.Equal(t, "D", topology1[2].GetName())
		require.Equal(t, "A", topology1[3].GetName())
	})

	t.Run("shared dependency visited once", func(t *testing.T) {
		nodeA := newNodeWrapper(TestNode{name: "A"})
		nodeB := newNodeWrapper(TestNode{name: "B"})
		nodeC := newNodeWrapper(TestNode{name: "C"})
		nodeD := newNodeWrapper(TestNode{name: "D"})

		// B -> D, C -> D (D is shared)
		// A -> B, A -> C
		nodeB.AddChild(nodeD, nil)
		nodeC.AddChild(nodeD, nil)
		nodeA.AddChild(nodeB, nil)
		nodeA.AddChild(nodeC, nil)

		topology := nodeA.Topology()

		// Each node should appear exactly once
		names := make(map[string]int)
		for _, node := range topology {
			names[node.GetName()]++
		}

		require.Equal(t, 1, names["A"])
		require.Equal(t, 1, names["B"])
		require.Equal(t, 1, names["C"])
		require.Equal(t, 1, names["D"])
	})
}
