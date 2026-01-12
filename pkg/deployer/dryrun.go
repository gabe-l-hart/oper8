package deployer

import (
	"context"
	"fmt"
	"io"
	"sync"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/watch"
	"sigs.k8s.io/yaml"
)

// DryRunDeployManager implements Manager for local development and testing.
// It tracks operations in memory without interacting with a real cluster.
// Optionally, it can output YAML to a writer for manifest generation.
type DryRunDeployManager struct {
	deployed     []*unstructured.Unstructured
	disabled     []*unstructured.Unstructured
	statusMap    map[string]map[string]interface{} // key: namespace/name/kind, value: status
	outputWriter io.Writer
	mu           sync.RWMutex
}

// NewDryRunDeployManager creates a new dry-run deploy manager.
func NewDryRunDeployManager() *DryRunDeployManager {
	return &DryRunDeployManager{
		deployed:  []*unstructured.Unstructured{},
		disabled:  []*unstructured.Unstructured{},
		statusMap: make(map[string]map[string]interface{}),
	}
}

// NewDryRunDeployManagerWithOutput creates a dry-run manager that writes YAML to the given writer.
func NewDryRunDeployManagerWithOutput(w io.Writer) *DryRunDeployManager {
	dm := NewDryRunDeployManager()
	dm.outputWriter = w
	return dm
}

// Deploy tracks the deployment in memory and optionally outputs YAML.
func (d *DryRunDeployManager) Deploy(ctx context.Context, resources []*unstructured.Unstructured, opts DeployOptions) (bool, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if len(resources) == 0 {
		return false, nil
	}

	// Add to deployed list
	d.deployed = append(d.deployed, resources...)

	// Write YAML if output writer is configured
	if d.outputWriter != nil {
		for _, res := range resources {
			yamlBytes, err := yaml.Marshal(res.Object)
			if err != nil {
				return false, fmt.Errorf("failed to marshal resource to YAML: %w", err)
			}

			// Write YAML document separator and content
			if _, err := fmt.Fprintf(d.outputWriter, "---\n%s\n", yamlBytes); err != nil {
				return false, fmt.Errorf("failed to write YAML: %w", err)
			}
		}
	}

	return true, nil
}

// Disable tracks the removal in memory.
func (d *DryRunDeployManager) Disable(ctx context.Context, resources []*unstructured.Unstructured) (bool, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if len(resources) == 0 {
		return false, nil
	}

	d.disabled = append(d.disabled, resources...)
	return true, nil
}

// GetObjectCurrentState retrieves a previously deployed resource from memory.
func (d *DryRunDeployManager) GetObjectCurrentState(ctx context.Context, ref ObjectRef) (*unstructured.Unstructured, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	// Search through deployed resources
	for _, res := range d.deployed {
		if d.matchesRef(res, ref) {
			// Return a deep copy to prevent mutations
			return res.DeepCopy(), nil
		}
	}

	// Not found
	return nil, nil
}

// FilterObjectsCurrentState filters deployed resources by criteria.
func (d *DryRunDeployManager) FilterObjectsCurrentState(ctx context.Context, filter ObjectFilter) ([]*unstructured.Unstructured, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var results []*unstructured.Unstructured

	for _, res := range d.deployed {
		if d.matchesFilter(res, filter) {
			results = append(results, res.DeepCopy())
		}
	}

	return results, nil
}

// SetStatus tracks status updates in memory.
func (d *DryRunDeployManager) SetStatus(ctx context.Context, ref ObjectRef, status map[string]interface{}) (bool, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	key := d.makeKey(ref.Namespace, ref.Name, ref.Kind)
	d.statusMap[key] = status

	return true, nil
}

// WatchObjects is not implemented for dry-run mode.
func (d *DryRunDeployManager) WatchObjects(ctx context.Context, filter ObjectFilter) (watch.Interface, error) {
	return nil, fmt.Errorf("watch is not supported in dry-run mode")
}

// GetDeployedResources returns all deployed resources (for testing/inspection).
func (d *DryRunDeployManager) GetDeployedResources() []*unstructured.Unstructured {
	d.mu.RLock()
	defer d.mu.RUnlock()

	// Return a deep copy to prevent external mutations
	result := make([]*unstructured.Unstructured, len(d.deployed))
	for i, res := range d.deployed {
		result[i] = res.DeepCopy()
	}

	return result
}

// GetDisabledResources returns all disabled resources (for testing/inspection).
func (d *DryRunDeployManager) GetDisabledResources() []*unstructured.Unstructured {
	d.mu.RLock()
	defer d.mu.RUnlock()

	result := make([]*unstructured.Unstructured, len(d.disabled))
	for i, res := range d.disabled {
		result[i] = res.DeepCopy()
	}

	return result
}

// GetStatus returns the status for a resource (for testing/inspection).
func (d *DryRunDeployManager) GetStatus(ref ObjectRef) (map[string]interface{}, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	key := d.makeKey(ref.Namespace, ref.Name, ref.Kind)
	status, exists := d.statusMap[key]
	return status, exists
}

// Reset clears all tracked data (for testing).
func (d *DryRunDeployManager) Reset() {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.deployed = []*unstructured.Unstructured{}
	d.disabled = []*unstructured.Unstructured{}
	d.statusMap = make(map[string]map[string]interface{})
}

// Helper methods

func (d *DryRunDeployManager) matchesRef(res *unstructured.Unstructured, ref ObjectRef) bool {
	return res.GetKind() == ref.Kind &&
		res.GetAPIVersion() == ref.APIVersion &&
		res.GetName() == ref.Name &&
		res.GetNamespace() == ref.Namespace
}

func (d *DryRunDeployManager) matchesFilter(res *unstructured.Unstructured, filter ObjectFilter) bool {
	// Match Kind
	if filter.Kind != "" && res.GetKind() != filter.Kind {
		return false
	}

	// Match APIVersion
	if filter.APIVersion != "" && res.GetAPIVersion() != filter.APIVersion {
		return false
	}

	// Match Namespace
	if filter.Namespace != "" && res.GetNamespace() != filter.Namespace {
		return false
	}

	// TODO: Implement label selector matching if needed
	// For now, we skip label/field selector matching in dry-run mode

	return true
}

func (d *DryRunDeployManager) makeKey(namespace, name, kind string) string {
	return fmt.Sprintf("%s/%s/%s", namespace, name, kind)
}
