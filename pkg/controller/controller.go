package controller

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/IBM/oper8/pkg/dag"
	"github.com/IBM/oper8/pkg/session"
)

// Controller manages the reconciliation for a specific CustomResource type.
// It defines the GVK (GroupVersionKind) it handles and orchestrates components.
//
// Users typically embed BaseController and override methods:
//   - SetupComponents: Define the component DAG for reconciliation
//   - FinalizeComponents: Handle CR deletion/cleanup
//   - AfterDeploy/AfterVerify: Lifecycle hooks for custom logic
type Controller interface {
	// GVK returns the GroupVersionKind this controller manages
	GVK() schema.GroupVersionKind

	// SetupComponents builds the component DAG for reconciliation.
	// This is called at the start of reconciliation to define what needs to be deployed.
	SetupComponents(ctx context.Context, sess *session.Session) error

	// FinalizeComponents handles cleanup when the CR is being deleted.
	// This is called when the CR has a deletion timestamp.
	FinalizeComponents(ctx context.Context, sess *session.Session) error

	// Lifecycle hooks (optional, called at key points during reconciliation)

	// AfterDeploy is called after all components have been deployed successfully.
	AfterDeploy(ctx context.Context, sess *session.Session, state *dag.CompletionState) (bool, error)

	// AfterDeployUnsuccessful is called when deployment doesn't complete successfully.
	AfterDeployUnsuccessful(ctx context.Context, sess *session.Session, failed bool, state *dag.CompletionState) (bool, error)

	// AfterVerify is called after all components have been verified successfully.
	AfterVerify(ctx context.Context, sess *session.Session, verifyState, deployState *dag.CompletionState) (bool, error)

	// AfterVerifyUnsuccessful is called when verification doesn't complete successfully.
	AfterVerifyUnsuccessful(ctx context.Context, sess *session.Session, failed bool, verifyState, deployState *dag.CompletionState) (bool, error)
}

// BaseController provides default implementations for Controller interface.
// Users should embed this and override methods as needed.
type BaseController struct {
	gvk            schema.GroupVersionKind
	configDefaults map[string]interface{}
}

// NewBase creates a new BaseController.
func NewBase(gvk schema.GroupVersionKind) *BaseController {
	return &BaseController{
		gvk:            gvk,
		configDefaults: make(map[string]interface{}),
	}
}

// NewBaseWithDefaults creates a new BaseController with config defaults.
func NewBaseWithDefaults(gvk schema.GroupVersionKind, defaults map[string]interface{}) *BaseController {
	return &BaseController{
		gvk:            gvk,
		configDefaults: defaults,
	}
}

// GVK returns the GroupVersionKind this controller manages.
func (bc *BaseController) GVK() schema.GroupVersionKind {
	return bc.gvk
}

// ConfigDefaults returns the default configuration values.
func (bc *BaseController) ConfigDefaults() map[string]interface{} {
	return bc.configDefaults
}

// SetupComponents is a no-op by default. Override this to define components.
func (bc *BaseController) SetupComponents(ctx context.Context, sess *session.Session) error {
	// Override this method to add components
	return nil
}

// FinalizeComponents is a no-op by default. Override this for custom cleanup.
func (bc *BaseController) FinalizeComponents(ctx context.Context, sess *session.Session) error {
	// Override this method for custom finalization logic
	return nil
}

// AfterDeploy is a no-op hook by default. Override for custom post-deploy logic.
func (bc *BaseController) AfterDeploy(ctx context.Context, sess *session.Session, state *dag.CompletionState) (bool, error) {
	return true, nil
}

// AfterDeployUnsuccessful is a no-op hook by default.
func (bc *BaseController) AfterDeployUnsuccessful(ctx context.Context, sess *session.Session, failed bool, state *dag.CompletionState) (bool, error) {
	return true, nil
}

// AfterVerify is a no-op hook by default. Override for custom post-verify logic.
func (bc *BaseController) AfterVerify(ctx context.Context, sess *session.Session, verifyState, deployState *dag.CompletionState) (bool, error) {
	return true, nil
}

// AfterVerifyUnsuccessful is a no-op hook by default.
func (bc *BaseController) AfterVerifyUnsuccessful(ctx context.Context, sess *session.Session, failed bool, verifyState, deployState *dag.CompletionState) (bool, error) {
	return true, nil
}
