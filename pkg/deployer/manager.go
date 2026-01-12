package deployer

import (
	"context"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/watch"
)

// Manager is the interface for Kubernetes resource operations.
// Implementations include KubernetesDeployManager (real cluster) and
// DryRunDeployManager (in-memory simulation).
type Manager interface {
	// Deploy applies resources to the cluster (or simulates it in dry-run mode).
	// Returns (changed bool, error). Changed indicates if any modifications were made.
	Deploy(ctx context.Context, resources []*unstructured.Unstructured, opts DeployOptions) (bool, error)

	// Disable removes resources from the cluster (or simulates removal).
	// Returns (changed bool, error).
	Disable(ctx context.Context, resources []*unstructured.Unstructured) (bool, error)

	// GetObjectCurrentState fetches the current state of a resource.
	// Returns (resource, error). Resource is nil if not found.
	GetObjectCurrentState(ctx context.Context, ref ObjectRef) (*unstructured.Unstructured, error)

	// FilterObjectsCurrentState lists resources matching the filter criteria.
	FilterObjectsCurrentState(ctx context.Context, filter ObjectFilter) ([]*unstructured.Unstructured, error)

	// SetStatus updates the status subresource of a resource.
	// Returns (changed bool, error).
	SetStatus(ctx context.Context, ref ObjectRef, status map[string]interface{}) (bool, error)

	// WatchObjects returns a watch interface for resources matching the filter.
	WatchObjects(ctx context.Context, filter ObjectFilter) (watch.Interface, error)
}

// DeployOptions configures how resources are deployed.
type DeployOptions struct {
	// ManageOwnerReferences determines if owner references should be added
	ManageOwnerReferences bool

	// Method specifies how to apply the resource
	Method DeployMethod

	// FieldManager identifies who is managing these fields (for server-side apply)
	FieldManager string
}

// DeployMethod specifies the strategy for applying resources.
type DeployMethod int

const (
	// DeployMethodDefault uses the default strategy (server-side apply with fallback to replace if needed)
	DeployMethodDefault DeployMethod = iota

	// DeployMethodUpdate uses client-side update (strategic merge patch)
	DeployMethodUpdate

	// DeployMethodReplace uses PUT to replace the entire resource
	DeployMethodReplace
)

// ObjectRef identifies a specific Kubernetes resource.
type ObjectRef struct {
	// APIVersion is the group/version (e.g., "apps/v1")
	APIVersion string

	// Kind is the resource kind (e.g., "Deployment")
	Kind string

	// Namespace is the namespace (empty for cluster-scoped resources)
	Namespace string

	// Name is the resource name
	Name string
}

// ObjectFilter specifies criteria for listing or watching resources.
type ObjectFilter struct {
	// APIVersion is the group/version (e.g., "v1", "apps/v1")
	APIVersion string

	// Kind is the resource kind (e.g., "Pod", "Deployment")
	Kind string

	// Namespace is the namespace (empty for cluster-scoped or all namespaces)
	Namespace string

	// LabelSelector filters by labels (e.g., "app=myapp,env=prod")
	LabelSelector string

	// FieldSelector filters by fields (e.g., "metadata.name=myname")
	FieldSelector string

	// ResourceVersion for watch operations
	ResourceVersion string
}
