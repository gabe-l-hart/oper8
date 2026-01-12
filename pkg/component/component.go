package component

import (
	"context"
)

// Component represents an atomic grouping of Kubernetes resources.
// Components are the building blocks of an operator's reconciliation logic.
//
// Users typically embed BaseComponent and override methods:
//   - BuildChart: Define resources for this component
//   - Verify: Custom verification logic
//   - Deploy/Disable: Custom deployment/removal logic
type Component interface {
	// GetName returns the unique name of this component (for DAG node identification)
	GetName() string

	// BuildChart constructs the resource DAG for this component.
	// This is called during the render phase before deployment.
	BuildChart(ctx context.Context, sess Session) error

	// Deploy applies all resources in this component to the cluster.
	// Returns an error if deployment fails.
	Deploy(ctx context.Context, sess Session) error

	// Verify checks if the component is successfully deployed and ready.
	// Returns (success bool, error). If success is false, reconciliation may retry.
	Verify(ctx context.Context, sess Session) (bool, error)

	// Disable removes all resources in this component from the cluster.
	// Returns an error if removal fails.
	Disable(ctx context.Context, sess Session) error

	// IsDisabled returns true if this component should be skipped.
	IsDisabled() bool
}

// Session is a minimal interface for the session object.
// This avoids circular dependencies between component and session packages.
// The full Session implementation is in pkg/session.
type Session interface {
	// ID returns the unique identifier for this reconciliation
	ID() string

	// Name returns the CR instance name
	Name() string

	// Namespace returns the CR namespace
	Namespace() string

	// GetName returns the component name (for dag.Node interface)
	GetName() string
}

// BaseComponent provides a default implementation of Component.
// Users should embed this and override methods as needed.
type BaseComponent struct {
	name     string
	disabled bool
}

// NewBase creates a new BaseComponent.
func NewBase(name string, disabled bool) *BaseComponent {
	return &BaseComponent{
		name:     name,
		disabled: disabled,
	}
}

// GetName returns the component name (implements dag.Node).
func (b *BaseComponent) GetName() string {
	return b.name
}

// IsDisabled returns true if this component should be skipped.
func (b *BaseComponent) IsDisabled() bool {
	return b.disabled
}

// BuildChart is a no-op by default. Override this to add resources.
func (b *BaseComponent) BuildChart(ctx context.Context, sess Session) error {
	return nil
}

// Deploy is a no-op by default. Override this or use the default implementation
// that deploys all managed resources.
func (b *BaseComponent) Deploy(ctx context.Context, sess Session) error {
	return nil
}

// Verify returns true by default. Override this for custom verification.
func (b *BaseComponent) Verify(ctx context.Context, sess Session) (bool, error) {
	return true, nil
}

// Disable is a no-op by default. Override this for custom cleanup.
func (b *BaseComponent) Disable(ctx context.Context, sess Session) error {
	return nil
}
