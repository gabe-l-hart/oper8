package dag

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// NodeFunc is a function that processes a node. It returns an error if the node
// processing fails. The error type determines whether execution should halt.
type NodeFunc[T Node] func(context.Context, T) error

// Runner executes a DAG using a worker pool pattern with goroutines.
// It processes nodes in topological order, respecting dependencies.
type Runner[T Node] struct {
	name           string
	graph          *Graph[T]
	defaultFunc    NodeFunc[T]
	workerPool     int
	verifyUpstream bool
	pollInterval   time.Duration

	// State tracking
	mu          sync.RWMutex
	started     map[string]bool
	verified    map[string]bool
	unverified  map[string]bool
	failed      map[string]bool
	disabled    map[string]bool
	exception   error
}

// NewRunner creates a new DAG runner.
func NewRunner[T Node](name string, graph *Graph[T], defaultFunc NodeFunc[T]) *Runner[T] {
	return &Runner[T]{
		name:           name,
		graph:          graph,
		defaultFunc:    defaultFunc,
		workerPool:     4, // Default to 4 workers
		verifyUpstream: true,
		pollInterval:   50 * time.Millisecond,
		started:        make(map[string]bool),
		verified:       make(map[string]bool),
		unverified:     make(map[string]bool),
		failed:         make(map[string]bool),
		disabled:       make(map[string]bool),
	}
}

// WithWorkerPool sets the number of goroutines in the worker pool.
func (r *Runner[T]) WithWorkerPool(n int) *Runner[T] {
	r.workerPool = n
	return r
}

// WithPollInterval sets the interval for checking ready nodes.
func (r *Runner[T]) WithPollInterval(d time.Duration) *Runner[T] {
	r.pollInterval = d
	return r
}

// WithVerifyUpstream sets whether to verify upstream dependencies.
func (r *Runner[T]) WithVerifyUpstream(verify bool) *Runner[T] {
	r.verifyUpstream = verify
	return r
}

// Run executes the DAG and returns the completion state.
func (r *Runner[T]) Run(ctx context.Context) *CompletionState {
	topology := r.graph.Topology()
	if len(topology) == 0 {
		return NewCompletionState()
	}

	// Create channels
	workChan := make(chan T, r.workerPool)
	errChan := make(chan error, 1)

	// Create cancellable context
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < r.workerPool; i++ {
		wg.Add(1)
		go r.worker(ctx, workChan, errChan, &wg)
	}

	// Start scheduler
	go r.scheduler(ctx, topology, workChan, errChan, cancel)

	// Wait for all workers to finish
	wg.Wait()

	// Check for fatal errors
	select {
	case err := <-errChan:
		r.mu.Lock()
		r.exception = err
		r.mu.Unlock()
	default:
	}

	// Build completion state
	return r.buildCompletionState(topology)
}

// worker processes nodes from the work channel.
func (r *Runner[T]) worker(ctx context.Context, work <-chan T, errs chan<- error, wg *sync.WaitGroup) {
	defer wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case node, ok := <-work:
			if !ok {
				return
			}

			if err := r.executeNode(ctx, node); err != nil {
				// Check if error is fatal
				if isFatalError(err) {
					select {
					case errs <- err:
					default:
					}
					return
				}
				// Non-fatal: mark as unverified
				r.markUnverified(node)
			} else {
				r.markVerified(node)
			}
		}
	}
}

// scheduler feeds ready nodes to workers.
func (r *Runner[T]) scheduler(ctx context.Context, topology []T, work chan<- T, errs <-chan error, cancel context.CancelFunc) {
	ticker := time.NewTicker(r.pollInterval)
	defer ticker.Stop()
	defer close(work)

	for {
		select {
		case <-ctx.Done():
			return
		case err := <-errs:
			if isFatalError(err) {
				cancel()
				return
			}
		case <-ticker.C:
			ready := r.getReadyNodes(ctx, topology)

			if len(ready) == 0 && r.allStartedOrSkipped(topology) {
				// All nodes either started or can't start
				return
			}

			for _, node := range ready {
				r.markStarted(node)
				select {
				case work <- node:
				case <-ctx.Done():
					return
				}
			}
		}
	}
}

// executeNode runs the default function on a node.
func (r *Runner[T]) executeNode(ctx context.Context, node T) error {
	if r.defaultFunc == nil {
		return nil
	}
	return r.defaultFunc(ctx, node)
}

// getReadyNodes returns nodes that are ready to execute (dependencies satisfied).
func (r *Runner[T]) getReadyNodes(ctx context.Context, topology []T) []T {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var ready []T

	for _, node := range topology {
		name := node.GetName()

		// Skip if already started, disabled, or failed
		if r.started[name] || r.disabled[name] || r.failed[name] {
			continue
		}

		// Check if all dependencies are satisfied
		if r.dependenciesSatisfied(ctx, node) {
			ready = append(ready, node)
		}
	}

	return ready
}

// dependenciesSatisfied checks if all upstream dependencies of a node are satisfied.
func (r *Runner[T]) dependenciesSatisfied(ctx context.Context, node T) bool {
	// Find all nodes that this node depends on (nodes that have this node as a child)
	allNodes := r.graph.GetAllNodes()

	for _, potentialParent := range allNodes {
		children, err := r.graph.GetChildren(potentialParent)
		if err != nil {
			continue
		}

		// Check if this node is a child of potentialParent
		isChild := false
		for _, child := range children {
			if child.GetName() == node.GetName() {
				isChild = true
				break
			}
		}

		if !isChild {
			continue
		}

		// This node depends on potentialParent - check if it's satisfied
		parentName := potentialParent.GetName()

		// Parent must be verified
		if !r.verified[parentName] {
			return false
		}

		// If verify upstream is enabled, run the edge verification function
		if r.verifyUpstream {
			edgeFunc, err := r.graph.GetEdgeFunc(potentialParent, node)
			if err == nil && edgeFunc != nil {
				satisfied, err := edgeFunc(ctx)
				if err != nil || !satisfied {
					return false
				}
			}
		}
	}

	return true
}

// allStartedOrSkipped returns true if all nodes have either started or can't start.
func (r *Runner[T]) allStartedOrSkipped(topology []T) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, node := range topology {
		name := node.GetName()
		if !r.started[name] && !r.disabled[name] && !r.failed[name] {
			// This node hasn't started - check if it ever can
			if r.dependenciesSatisfied(context.Background(), node) {
				return false // Node is ready but hasn't started yet
			}
		}
	}

	return true
}

// State management methods
func (r *Runner[T]) markStarted(node T) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.started[node.GetName()] = true
}

func (r *Runner[T]) markVerified(node T) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.verified[node.GetName()] = true
}

func (r *Runner[T]) markUnverified(node T) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.unverified[node.GetName()] = true
}

func (r *Runner[T]) markFailed(node T) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.failed[node.GetName()] = true
}

func (r *Runner[T]) markDisabled(node T) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.disabled[node.GetName()] = true
}

// buildCompletionState constructs the final completion state.
func (r *Runner[T]) buildCompletionState(topology []T) *CompletionState {
	r.mu.RLock()
	defer r.mu.RUnlock()

	state := NewCompletionState()

	for _, node := range topology {
		name := node.GetName()

		if r.verified[name] {
			state.VerifiedNodes = append(state.VerifiedNodes, name)
		} else if r.unverified[name] {
			state.UnverifiedNodes = append(state.UnverifiedNodes, name)
		} else if r.failed[name] {
			state.FailedNodes = append(state.FailedNodes, name)
		} else if r.disabled[name] {
			state.DisabledNodes = append(state.DisabledNodes, name)
		} else if !r.started[name] {
			state.UnstartedNodes = append(state.UnstartedNodes, name)
		}
	}

	state.Exception = r.exception

	return state
}

// FatalError wraps an error to indicate it should halt DAG execution.
type FatalError struct {
	Err error
}

func (e *FatalError) Error() string {
	return fmt.Sprintf("fatal: %v", e.Err)
}

func (e *FatalError) Unwrap() error {
	return e.Err
}

// NewFatalError creates a new fatal error.
func NewFatalError(err error) error {
	return &FatalError{Err: err}
}

// isFatalError checks if an error is fatal.
func isFatalError(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(*FatalError)
	return ok
}
