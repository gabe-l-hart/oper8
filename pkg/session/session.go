package session

import (
	"context"
	"fmt"
	"sync"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/IBM/oper8/pkg/component"
	"github.com/IBM/oper8/pkg/dag"
	"github.com/IBM/oper8/pkg/deployer"
)

// ComponentNode wraps a Component to make it usable in the generic DAG.
// This is needed because Component is an interface and can't directly satisfy
// the comparable constraint required by dag.Graph.
type ComponentNode struct {
	comp component.Component
}

// GetName implements dag.Node.
func (cn *ComponentNode) GetName() string {
	return cn.comp.GetName()
}

// GetComponent returns the wrapped component.
func (cn *ComponentNode) GetComponent() component.Component {
	return cn.comp
}

// Session contains the state of a single reconciliation.
// It is shared across all components and controllers involved in the reconciliation.
//
// Key responsibilities:
//   - Hold immutable reconciliation context (CR manifest, config)
//   - Maintain the component dependency DAG
//   - Provide access to the DeployManager for K8s operations
//   - Offer utility methods for common operations
type Session struct {
	// Immutable fields (set at construction)
	id            string
	crManifest    *unstructured.Unstructured
	config        map[string]interface{}
	deployManager deployer.Manager

	// Mutable state (protected by mutex)
	graph      *dag.Graph[*ComponentNode]
	components map[string]component.Component // Maps name -> component
	mu         sync.RWMutex
}

// New creates a new Session for a reconciliation.
func New(
	id string,
	crManifest *unstructured.Unstructured,
	config map[string]interface{},
	deployManager deployer.Manager,
) *Session {
	if config == nil {
		config = make(map[string]interface{})
	}

	return &Session{
		id:            id,
		crManifest:    crManifest,
		config:        config,
		deployManager: deployManager,
		graph:         dag.NewGraph[*ComponentNode](),
		components:    make(map[string]component.Component),
	}
}

// Accessor methods for immutable fields

// ID returns the unique identifier for this reconciliation.
func (s *Session) ID() string {
	return s.id
}

// CRManifest returns the full Custom Resource manifest that triggered this reconciliation.
func (s *Session) CRManifest() *unstructured.Unstructured {
	return s.crManifest
}

// Name returns the CR instance name.
func (s *Session) Name() string {
	if s.crManifest == nil {
		return ""
	}
	return s.crManifest.GetName()
}

// Namespace returns the CR namespace.
func (s *Session) Namespace() string {
	if s.crManifest == nil {
		return ""
	}
	return s.crManifest.GetNamespace()
}

// Kind returns the CR kind.
func (s *Session) Kind() string {
	if s.crManifest == nil {
		return ""
	}
	return s.crManifest.GetKind()
}

// APIVersion returns the CR API version.
func (s *Session) APIVersion() string {
	if s.crManifest == nil {
		return ""
	}
	return s.crManifest.GetAPIVersion()
}

// Spec returns the spec section of the CR.
func (s *Session) Spec() map[string]interface{} {
	if s.crManifest == nil {
		return nil
	}

	spec, found, err := unstructured.NestedMap(s.crManifest.Object, "spec")
	if err != nil || !found {
		return make(map[string]interface{})
	}

	return spec
}

// Status returns the status section of the CR.
func (s *Session) Status() map[string]interface{} {
	if s.crManifest == nil {
		return nil
	}

	status, found, err := unstructured.NestedMap(s.crManifest.Object, "status")
	if err != nil || !found {
		return make(map[string]interface{})
	}

	return status
}

// Config returns the operator configuration.
func (s *Session) Config() map[string]interface{} {
	return s.config
}

// DeployManager returns the deploy manager for this session.
func (s *Session) DeployManager() deployer.Manager {
	return s.deployManager
}

// GetName implements the dag.Node interface (for sessions as nodes, if needed).
func (s *Session) GetName() string {
	return s.Name()
}

// Component management

// AddComponent registers a component with this session.
// The component is added to the dependency graph.
func (s *Session) AddComponent(comp component.Component) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	name := comp.GetName()
	if _, exists := s.components[name]; exists {
		return fmt.Errorf("component %s already exists", name)
	}

	node := &ComponentNode{comp: comp}
	if err := s.graph.AddNode(node); err != nil {
		return err
	}

	s.components[name] = comp
	return nil
}

// AddComponentDependency establishes that 'dependent' depends on 'upstream'.
// This means 'upstream' must be deployed and verified before 'dependent' can proceed.
//
// The optional verifyFn is called to verify the dependency is satisfied.
func (s *Session) AddComponentDependency(
	dependent component.Component,
	upstream component.Component,
	verifyFn dag.EdgeFunc,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Find the nodes for these components
	upstreamNode, upstreamExists := s.graph.GetNode(upstream.GetName())
	if !upstreamExists {
		return fmt.Errorf("upstream component %s not found", upstream.GetName())
	}

	dependentNode, dependentExists := s.graph.GetNode(dependent.GetName())
	if !dependentExists {
		return fmt.Errorf("dependent component %s not found", dependent.GetName())
	}

	// In DAG terms: upstream is the parent, dependent is the child
	return s.graph.AddEdge(upstreamNode, dependentNode, verifyFn)
}

// GetComponent retrieves a component by name.
func (s *Session) GetComponent(name string) (component.Component, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	comp, exists := s.components[name]
	return comp, exists
}

// GetAllComponents returns all registered components.
func (s *Session) GetAllComponents() []component.Component {
	s.mu.RLock()
	defer s.mu.RUnlock()

	components := make([]component.Component, 0, len(s.components))
	for _, comp := range s.components {
		components = append(components, comp)
	}
	return components
}

// GetComponentTopology returns components in topological order (dependencies first).
func (s *Session) GetComponentTopology() []component.Component {
	s.mu.RLock()
	defer s.mu.RUnlock()

	topology := s.graph.Topology()
	components := make([]component.Component, 0, len(topology))
	for _, node := range topology {
		components = append(components, node.comp)
	}
	return components
}

// Graph returns the component dependency graph (for direct access if needed).
func (s *Session) Graph() *dag.Graph[*ComponentNode] {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.graph
}

// Kubernetes operations (delegated to DeployManager)

// GetObjectCurrentState fetches the current state of a resource from the cluster.
func (s *Session) GetObjectCurrentState(ctx context.Context, ref deployer.ObjectRef) (*unstructured.Unstructured, error) {
	return s.deployManager.GetObjectCurrentState(ctx, ref)
}

// FilterObjectsCurrentState lists resources matching the filter criteria.
func (s *Session) FilterObjectsCurrentState(ctx context.Context, filter deployer.ObjectFilter) ([]*unstructured.Unstructured, error) {
	return s.deployManager.FilterObjectsCurrentState(ctx, filter)
}

// SetStatus updates the status subresource of the CR or another resource.
func (s *Session) SetStatus(ctx context.Context, ref deployer.ObjectRef, status map[string]interface{}) (bool, error) {
	return s.deployManager.SetStatus(ctx, ref, status)
}

// Utility methods

// GetScopedName creates an application-scoped name by prefixing with the CR name.
// For example, GetScopedName("database") might return "myapp-database".
func (s *Session) GetScopedName(suffix string) string {
	return fmt.Sprintf("%s-%s", s.Name(), suffix)
}

// GetTruncatedName ensures a name fits within Kubernetes limits (63 characters).
func (s *Session) GetTruncatedName(name string) string {
	const maxLen = 63
	if len(name) <= maxLen {
		return name
	}
	return name[:maxLen]
}
