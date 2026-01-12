package rollout

import (
	"context"
	"fmt"

	"github.com/IBM/oper8/pkg/controller"
	"github.com/IBM/oper8/pkg/dag"
	"github.com/IBM/oper8/pkg/session"
)

// ExecuteReconciliation runs the full reconciliation flow for a controller.
//
// The flow is:
//  1. Setup: Controller.SetupComponents() builds the component DAG
//  2. Phase 1 - Deploy: Walk DAG, calling BuildChart() and Deploy() for each component
//  3. Hook: Controller.AfterDeploy() or AfterDeployUnsuccessful()
//  4. Phase 2 - Verify: Walk DAG, calling Verify() for each component
//  5. Hook: Controller.AfterVerify() or AfterVerifyUnsuccessful()
//
// Returns the completion state of the deployment and verification phases.
func ExecuteReconciliation(
	ctx context.Context,
	ctrl controller.Controller,
	sess *session.Session,
) (*dag.CompletionState, error) {
	// Phase 1: Setup components
	if err := ctrl.SetupComponents(ctx, sess); err != nil {
		return nil, fmt.Errorf("failed to setup components: %w", err)
	}

	// Phase 2: Deploy components
	deployState, err := deployComponents(ctx, sess)
	if err != nil {
		return deployState, err
	}

	// Phase 3: After-deploy hook
	if deployState.IsSuccessful() {
		if _, err := ctrl.AfterDeploy(ctx, sess, deployState); err != nil {
			return deployState, fmt.Errorf("after_deploy hook failed: %w", err)
		}
	} else {
		failed := deployState.HasFailures()
		if _, err := ctrl.AfterDeployUnsuccessful(ctx, sess, failed, deployState); err != nil {
			return deployState, fmt.Errorf("after_deploy_unsuccessful hook failed: %w", err)
		}

		// If deployment failed, don't proceed to verification
		if failed {
			return deployState, nil
		}
	}

	// Phase 4: Verify components
	verifyState, err := verifyComponents(ctx, sess, deployState)
	if err != nil {
		return verifyState, err
	}

	// Phase 5: After-verify hook
	if verifyState.IsSuccessful() {
		if _, err := ctrl.AfterVerify(ctx, sess, verifyState, deployState); err != nil {
			return verifyState, fmt.Errorf("after_verify hook failed: %w", err)
		}
	} else {
		failed := verifyState.HasFailures()
		if _, err := ctrl.AfterVerifyUnsuccessful(ctx, sess, failed, verifyState, deployState); err != nil {
			return verifyState, fmt.Errorf("after_verify_unsuccessful hook failed: %w", err)
		}
	}

	return verifyState, nil
}

// deployComponents walks the component DAG and deploys each component in topological order.
func deployComponents(ctx context.Context, sess *session.Session) (*dag.CompletionState, error) {
	// Create a runner to execute the DAG
	graph := sess.Graph()

	runner := dag.NewRunner("deploy", graph, deployComponentFunc(sess)).
		WithWorkerPool(4). // TODO: Make configurable
		WithVerifyUpstream(true)

	return runner.Run(ctx), nil
}

// deployComponentFunc returns a NodeFunc that deploys a component node.
func deployComponentFunc(sess *session.Session) dag.NodeFunc[*session.ComponentNode] {
	return func(ctx context.Context, node *session.ComponentNode) error {
		comp := node.GetComponent()

		// Skip disabled components
		if comp.IsDisabled() {
			return nil
		}

		// Build the chart (render resources)
		if err := comp.BuildChart(ctx, sess); err != nil {
			return dag.NewFatalError(fmt.Errorf("failed to build chart for %s: %w", comp.GetName(), err))
		}

		// Deploy the component
		if err := comp.Deploy(ctx, sess); err != nil {
			// Determine if error is fatal based on error type
			// For now, treat all deploy errors as fatal
			return dag.NewFatalError(fmt.Errorf("failed to deploy %s: %w", comp.GetName(), err))
		}

		return nil
	}
}

// verifyComponents walks the component DAG and verifies each component.
func verifyComponents(ctx context.Context, sess *session.Session, deployState *dag.CompletionState) (*dag.CompletionState, error) {
	// Only verify components that were successfully deployed
	verifiedInDeploy := make(map[string]bool)
	for _, name := range deployState.VerifiedNodes {
		verifiedInDeploy[name] = true
	}

	graph := sess.Graph()

	runner := dag.NewRunner("verify", graph, verifyComponentFunc(sess, verifiedInDeploy)).
		WithWorkerPool(4). // TODO: Make configurable
		WithVerifyUpstream(true)

	return runner.Run(ctx), nil
}

// verifyComponentFunc returns a NodeFunc that verifies a component node.
func verifyComponentFunc(sess *session.Session, deployedNodes map[string]bool) dag.NodeFunc[*session.ComponentNode] {
	return func(ctx context.Context, node *session.ComponentNode) error {
		comp := node.GetComponent()

		// Skip if not deployed or disabled
		if !deployedNodes[comp.GetName()] || comp.IsDisabled() {
			return nil
		}

		// Verify the component
		success, err := comp.Verify(ctx, sess)
		if err != nil {
			// Verification errors are typically non-fatal (will retry)
			return fmt.Errorf("failed to verify %s: %w", comp.GetName(), err)
		}

		if !success {
			// Verification failed but no error - treat as non-fatal
			return fmt.Errorf("verification failed for %s", comp.GetName())
		}

		return nil
	}
}
