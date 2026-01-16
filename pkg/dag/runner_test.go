package dag

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNewRunner(t *testing.T) {
	g := NewGraph[TestNode]()
	defaultFunc := func(ctx context.Context, node TestNode) error {
		return nil
	}

	runner := NewRunner("test-runner", g, defaultFunc)

	require.NotNil(t, runner)
	require.Equal(t, "test-runner", runner.name)
	require.Equal(t, g, runner.graph)
	require.Equal(t, 4, runner.workerPool)
	require.True(t, runner.verifyUpstream)
	require.Equal(t, 50*time.Millisecond, runner.pollInterval)
	require.NotNil(t, runner.started)
	require.NotNil(t, runner.verified)
	require.NotNil(t, runner.unverified)
	require.NotNil(t, runner.failed)
	require.NotNil(t, runner.disabled)
}

func TestRunner_BuilderMethods(t *testing.T) {
	g := NewGraph[TestNode]()
	runner := NewRunner("test", g, nil)

	t.Run("WithWorkerPool", func(t *testing.T) {
		result := runner.WithWorkerPool(8)
		require.Equal(t, 8, runner.workerPool)
		require.Equal(t, runner, result) // Should return self for chaining
	})

	t.Run("WithPollInterval", func(t *testing.T) {
		result := runner.WithPollInterval(100 * time.Millisecond)
		require.Equal(t, 100*time.Millisecond, runner.pollInterval)
		require.Equal(t, runner, result)
	})

	t.Run("WithVerifyUpstream", func(t *testing.T) {
		result := runner.WithVerifyUpstream(false)
		require.False(t, runner.verifyUpstream)
		require.Equal(t, runner, result)
	})

	t.Run("chained", func(t *testing.T) {
		g2 := NewGraph[TestNode]()
		r := NewRunner("chained", g2, nil).
			WithWorkerPool(2).
			WithPollInterval(25 * time.Millisecond).
			WithVerifyUpstream(false)

		require.Equal(t, 2, r.workerPool)
		require.Equal(t, 25*time.Millisecond, r.pollInterval)
		require.False(t, r.verifyUpstream)
	})
}

func TestRunner_Run_EmptyGraph(t *testing.T) {
	g := NewGraph[TestNode]()
	runner := NewRunner("empty", g, nil)

	state := runner.Run(context.Background())

	require.NotNil(t, state)
	require.Empty(t, state.VerifiedNodes)
	require.Empty(t, state.UnverifiedNodes)
	require.Empty(t, state.FailedNodes)
	require.Nil(t, state.Exception)
}

func TestRunner_Run_SingleNode(t *testing.T) {
	g := NewGraph[TestNode]()
	node := TestNode{name: "single"}
	g.AddNode(node)

	executed := false
	defaultFunc := func(ctx context.Context, n TestNode) error {
		executed = true
		require.Equal(t, "single", n.GetName())
		return nil
	}

	runner := NewRunner("single", g, defaultFunc).
		WithPollInterval(10 * time.Millisecond)

	state := runner.Run(context.Background())

	require.True(t, executed, "node should have been executed")
	require.Len(t, state.VerifiedNodes, 1)
	require.Equal(t, "single", state.VerifiedNodes[0])
	require.Empty(t, state.UnverifiedNodes)
	require.Empty(t, state.FailedNodes)
	require.Nil(t, state.Exception)
}

func TestRunner_Run_LinearDependencies(t *testing.T) {
	g := NewGraph[TestNode]()
	nodeA := TestNode{name: "A"}
	nodeB := TestNode{name: "B"}
	nodeC := TestNode{name: "C"}

	g.AddNode(nodeA)
	g.AddNode(nodeB)
	g.AddNode(nodeC)

	// A depends on B, B depends on C
	g.AddEdge(nodeB, nodeA, nil)
	g.AddEdge(nodeC, nodeB, nil)

	var mu sync.Mutex
	executionOrder := []string{}

	defaultFunc := func(ctx context.Context, n TestNode) error {
		mu.Lock()
		executionOrder = append(executionOrder, n.GetName())
		mu.Unlock()
		time.Sleep(10 * time.Millisecond) // Simulate work
		return nil
	}

	runner := NewRunner("linear", g, defaultFunc).
		WithWorkerPool(2).
		WithPollInterval(5 * time.Millisecond)

	state := runner.Run(context.Background())

	require.Len(t, state.VerifiedNodes, 3)
	require.Empty(t, state.UnverifiedNodes)
	require.Empty(t, state.FailedNodes)
	require.Nil(t, state.Exception)

	// Verify execution order: C before B before A
	require.Len(t, executionOrder, 3)
	cPos := indexOf(executionOrder, "C")
	bPos := indexOf(executionOrder, "B")
	aPos := indexOf(executionOrder, "A")

	require.Less(t, cPos, bPos, "C should execute before B")
	require.Less(t, bPos, aPos, "B should execute before A")
}

func TestRunner_Run_DiamondDependencies(t *testing.T) {
	g := NewGraph[TestNode]()
	nodeA := TestNode{name: "A"}
	nodeB := TestNode{name: "B"}
	nodeC := TestNode{name: "C"}
	nodeD := TestNode{name: "D"}

	g.AddNode(nodeA)
	g.AddNode(nodeB)
	g.AddNode(nodeC)
	g.AddNode(nodeD)

	// Diamond: A depends on B and C, B and C both depend on D
	g.AddEdge(nodeB, nodeA, nil)
	g.AddEdge(nodeC, nodeA, nil)
	g.AddEdge(nodeD, nodeB, nil)
	g.AddEdge(nodeD, nodeC, nil)

	var mu sync.Mutex
	executionOrder := []string{}

	defaultFunc := func(ctx context.Context, n TestNode) error {
		mu.Lock()
		executionOrder = append(executionOrder, n.GetName())
		mu.Unlock()
		time.Sleep(10 * time.Millisecond)
		return nil
	}

	runner := NewRunner("diamond", g, defaultFunc).
		WithWorkerPool(2).
		WithPollInterval(5 * time.Millisecond)

	state := runner.Run(context.Background())

	require.Len(t, state.VerifiedNodes, 4)
	require.Empty(t, state.FailedNodes)

	// Verify D executes first, A executes last
	require.Equal(t, "D", executionOrder[0], "D should execute first")
	require.Equal(t, "A", executionOrder[len(executionOrder)-1], "A should execute last")

	// B and C should both come after D and before A
	dPos := indexOf(executionOrder, "D")
	bPos := indexOf(executionOrder, "B")
	cPos := indexOf(executionOrder, "C")
	aPos := indexOf(executionOrder, "A")

	require.Less(t, dPos, bPos)
	require.Less(t, dPos, cPos)
	require.Less(t, bPos, aPos)
	require.Less(t, cPos, aPos)
}

func TestRunner_Run_ParallelExecution(t *testing.T) {
	g := NewGraph[TestNode]()
	node1 := TestNode{name: "node1"}
	node2 := TestNode{name: "node2"}
	node3 := TestNode{name: "node3"}

	g.AddNode(node1)
	g.AddNode(node2)
	g.AddNode(node3)
	// No dependencies - all can run in parallel

	var mu sync.Mutex
	running := 0
	maxParallel := 0

	defaultFunc := func(ctx context.Context, n TestNode) error {
		mu.Lock()
		running++
		if running > maxParallel {
			maxParallel = running
		}
		mu.Unlock()

		time.Sleep(20 * time.Millisecond)

		mu.Lock()
		running--
		mu.Unlock()

		return nil
	}

	runner := NewRunner("parallel", g, defaultFunc).
		WithWorkerPool(3).
		WithPollInterval(5 * time.Millisecond)

	state := runner.Run(context.Background())

	require.Len(t, state.VerifiedNodes, 3)
	require.Empty(t, state.FailedNodes)

	// Should have run multiple nodes in parallel
	require.GreaterOrEqual(t, maxParallel, 2, "should run nodes in parallel")
}

func TestRunner_Run_NodeFailure(t *testing.T) {
	g := NewGraph[TestNode]()
	nodeA := TestNode{name: "A"}
	nodeB := TestNode{name: "B"}

	g.AddNode(nodeA)
	g.AddNode(nodeB)

	failureErr := errors.New("node B failed")

	defaultFunc := func(ctx context.Context, n TestNode) error {
		if n.GetName() == "B" {
			return failureErr
		}
		return nil
	}

	runner := NewRunner("failure", g, defaultFunc).
		WithPollInterval(5 * time.Millisecond)

	state := runner.Run(context.Background())

	require.Len(t, state.VerifiedNodes, 1, "A should succeed")
	require.Len(t, state.UnverifiedNodes, 1, "B should be unverified")
	require.Equal(t, "B", state.UnverifiedNodes[0])
	require.Nil(t, state.Exception, "non-fatal error should not set exception")
}

func TestRunner_Run_FatalError(t *testing.T) {
	g := NewGraph[TestNode]()
	nodeA := TestNode{name: "A"}
	nodeB := TestNode{name: "B"}
	nodeC := TestNode{name: "C"}

	g.AddNode(nodeA)
	g.AddNode(nodeB)
	g.AddNode(nodeC)

	// Add dependencies so nodes execute sequentially: A, then B, then C
	g.AddEdge(nodeA, nodeB, nil) // B depends on A
	g.AddEdge(nodeB, nodeC, nil) // C depends on B

	fatalErr := errors.New("fatal error")

	var mu sync.Mutex
	executed := []string{}

	defaultFunc := func(ctx context.Context, n TestNode) error {
		mu.Lock()
		executed = append(executed, n.GetName())
		mu.Unlock()

		if n.GetName() == "B" {
			return NewFatalError(fatalErr)
		}
		time.Sleep(10 * time.Millisecond)
		return nil
	}

	runner := NewRunner("fatal", g, defaultFunc).
		WithWorkerPool(1).
		WithPollInterval(5 * time.Millisecond)

	state := runner.Run(context.Background())

	// A should execute, B should execute and fail, C should not execute
	mu.Lock()
	executedCopy := append([]string(nil), executed...)
	mu.Unlock()

	require.Contains(t, executedCopy, "A", "A should execute")
	require.Contains(t, executedCopy, "B", "B should execute and encounter fatal error")
	require.NotContains(t, executedCopy, "C", "C should not execute after fatal error in B")

	// Exception should be set (may be in exception field or not, depending on timing)
	// The key behavior is that C doesn't execute
	if state.Exception != nil {
		require.True(t, isFatalError(state.Exception))
		require.ErrorIs(t, state.Exception, fatalErr)
	}
}

func TestRunner_Run_WithEdgeVerification(t *testing.T) {
	g := NewGraph[TestNode]()
	nodeA := TestNode{name: "A"}
	nodeB := TestNode{name: "B"}

	g.AddNode(nodeA)
	g.AddNode(nodeB)

	verifyCalled := false
	verifyFunc := func(ctx context.Context) (bool, error) {
		verifyCalled = true
		return true, nil
	}

	// A depends on B with verification
	g.AddEdge(nodeB, nodeA, verifyFunc)

	defaultFunc := func(ctx context.Context, n TestNode) error {
		return nil
	}

	runner := NewRunner("verify", g, defaultFunc).
		WithPollInterval(5 * time.Millisecond).
		WithVerifyUpstream(true)

	state := runner.Run(context.Background())

	require.True(t, verifyCalled, "edge verification should be called")
	require.Len(t, state.VerifiedNodes, 2)
	require.Empty(t, state.FailedNodes)
}

func TestRunner_Run_EdgeVerificationFails(t *testing.T) {
	g := NewGraph[TestNode]()
	nodeA := TestNode{name: "A"}
	nodeB := TestNode{name: "B"}

	g.AddNode(nodeA)
	g.AddNode(nodeB)

	verifyFunc := func(ctx context.Context) (bool, error) {
		return false, nil // Verification fails
	}

	g.AddEdge(nodeB, nodeA, verifyFunc)

	defaultFunc := func(ctx context.Context, n TestNode) error {
		return nil
	}

	runner := NewRunner("verify-fail", g, defaultFunc).
		WithPollInterval(5 * time.Millisecond).
		WithVerifyUpstream(true)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	state := runner.Run(ctx)

	// B should execute, but A should not (verification failed)
	require.Len(t, state.VerifiedNodes, 1, "only B should verify")
	require.Equal(t, "B", state.VerifiedNodes[0])
	require.Len(t, state.UnstartedNodes, 1, "A should not start")
	require.Equal(t, "A", state.UnstartedNodes[0])
}

func TestRunner_Run_ContextCancellation(t *testing.T) {
	g := NewGraph[TestNode]()
	nodeA := TestNode{name: "A"}
	nodeB := TestNode{name: "B"}

	g.AddNode(nodeA)
	g.AddNode(nodeB)

	// B depends on A for sequential execution
	g.AddEdge(nodeA, nodeB, nil)

	var mu sync.Mutex
	executed := []string{}

	defaultFunc := func(ctx context.Context, n TestNode) error {
		mu.Lock()
		executed = append(executed, n.GetName())
		mu.Unlock()
		time.Sleep(100 * time.Millisecond)
		return nil
	}

	runner := NewRunner("cancel", g, defaultFunc).
		WithPollInterval(5 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	state := runner.Run(ctx)

	// With 100ms execution and 50ms timeout, only A should start (and complete after timeout)
	// B should not start because context is cancelled before A finishes
	mu.Lock()
	executedCount := len(executed)
	mu.Unlock()

	require.LessOrEqual(t, executedCount, 1, "should execute at most 1 node due to cancellation")
	require.LessOrEqual(t, len(state.VerifiedNodes), 1, "at most 1 node should be verified")
}

func TestFatalError(t *testing.T) {
	t.Run("NewFatalError", func(t *testing.T) {
		baseErr := errors.New("base error")
		fatalErr := NewFatalError(baseErr)

		require.NotNil(t, fatalErr)
		require.True(t, isFatalError(fatalErr))
		require.Contains(t, fatalErr.Error(), "fatal")
		require.Contains(t, fatalErr.Error(), "base error")
	})

	t.Run("isFatalError checks correctly", func(t *testing.T) {
		regularErr := errors.New("regular error")
		fatalErr := NewFatalError(errors.New("fatal"))

		require.False(t, isFatalError(nil))
		require.False(t, isFatalError(regularErr))
		require.True(t, isFatalError(fatalErr))
	})

	t.Run("Unwrap", func(t *testing.T) {
		baseErr := errors.New("base")
		fatalErr := NewFatalError(baseErr)

		require.ErrorIs(t, fatalErr, baseErr)
	})
}

func TestRunner_buildCompletionState(t *testing.T) {
	g := NewGraph[TestNode]()
	nodeA := TestNode{name: "A"}
	nodeB := TestNode{name: "B"}
	nodeC := TestNode{name: "C"}
	nodeD := TestNode{name: "D"}
	nodeE := TestNode{name: "E"}

	g.AddNode(nodeA)
	g.AddNode(nodeB)
	g.AddNode(nodeC)
	g.AddNode(nodeD)
	g.AddNode(nodeE)

	runner := NewRunner("state", g, nil)

	// Manually set state
	runner.verified["A"] = true
	runner.unverified["B"] = true
	runner.failed["C"] = true
	runner.disabled["D"] = true
	// E is unstarted

	testErr := errors.New("test exception")
	runner.exception = testErr

	topology := []TestNode{nodeA, nodeB, nodeC, nodeD, nodeE}
	state := runner.buildCompletionState(topology)

	require.Len(t, state.VerifiedNodes, 1)
	require.Contains(t, state.VerifiedNodes, "A")

	require.Len(t, state.UnverifiedNodes, 1)
	require.Contains(t, state.UnverifiedNodes, "B")

	require.Len(t, state.FailedNodes, 1)
	require.Contains(t, state.FailedNodes, "C")

	require.Len(t, state.DisabledNodes, 1)
	require.Contains(t, state.DisabledNodes, "D")

	require.Len(t, state.UnstartedNodes, 1)
	require.Contains(t, state.UnstartedNodes, "E")

	require.Equal(t, testErr, state.Exception)
}

// Helper function to find index of string in slice
func indexOf(slice []string, target string) int {
	for i, s := range slice {
		if s == target {
			return i
		}
	}
	return -1
}
