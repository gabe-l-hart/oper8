package deployer

import (
	"bytes"
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestNewDryRunDeployManager(t *testing.T) {
	dm := NewDryRunDeployManager()

	require.NotNil(t, dm)
	require.NotNil(t, dm.deployed)
	require.NotNil(t, dm.disabled)
	require.NotNil(t, dm.statusMap)
	require.Nil(t, dm.outputWriter)
	require.Empty(t, dm.deployed)
	require.Empty(t, dm.disabled)
	require.Empty(t, dm.statusMap)
}

func TestNewDryRunDeployManagerWithOutput(t *testing.T) {
	var buf bytes.Buffer
	dm := NewDryRunDeployManagerWithOutput(&buf)

	require.NotNil(t, dm)
	require.NotNil(t, dm.outputWriter)
	require.Equal(t, &buf, dm.outputWriter)
}

func TestDryRunDeployManager_Deploy(t *testing.T) {
	t.Run("deploy single resource", func(t *testing.T) {
		dm := NewDryRunDeployManager()
		res := createTestResource("v1", "ConfigMap", "default", "test-cm")

		changed, err := dm.Deploy(context.Background(), []*unstructured.Unstructured{res}, DeployOptions{})

		require.NoError(t, err)
		require.True(t, changed)

		deployed := dm.GetDeployedResources()
		require.Len(t, deployed, 1)
		require.Equal(t, "ConfigMap", deployed[0].GetKind())
		require.Equal(t, "test-cm", deployed[0].GetName())
	})

	t.Run("deploy multiple resources", func(t *testing.T) {
		dm := NewDryRunDeployManager()
		res1 := createTestResource("v1", "ConfigMap", "default", "cm1")
		res2 := createTestResource("v1", "Secret", "default", "secret1")
		res3 := createTestResource("apps/v1", "Deployment", "default", "deploy1")

		changed, err := dm.Deploy(context.Background(), []*unstructured.Unstructured{res1, res2, res3}, DeployOptions{})

		require.NoError(t, err)
		require.True(t, changed)

		deployed := dm.GetDeployedResources()
		require.Len(t, deployed, 3)
	})

	t.Run("deploy empty list", func(t *testing.T) {
		dm := NewDryRunDeployManager()

		changed, err := dm.Deploy(context.Background(), []*unstructured.Unstructured{}, DeployOptions{})

		require.NoError(t, err)
		require.False(t, changed)
		require.Empty(t, dm.GetDeployedResources())
	})

	t.Run("deploy with YAML output", func(t *testing.T) {
		var buf bytes.Buffer
		dm := NewDryRunDeployManagerWithOutput(&buf)
		res := createTestResource("v1", "ConfigMap", "default", "test-cm")

		changed, err := dm.Deploy(context.Background(), []*unstructured.Unstructured{res}, DeployOptions{})

		require.NoError(t, err)
		require.True(t, changed)

		output := buf.String()
		require.Contains(t, output, "---")
		require.Contains(t, output, "kind: ConfigMap")
		require.Contains(t, output, "name: test-cm")
		require.Contains(t, output, "namespace: default")
	})

	t.Run("deploy multiple with YAML output", func(t *testing.T) {
		var buf bytes.Buffer
		dm := NewDryRunDeployManagerWithOutput(&buf)
		res1 := createTestResource("v1", "ConfigMap", "default", "cm1")
		res2 := createTestResource("v1", "Secret", "default", "secret1")

		changed, err := dm.Deploy(context.Background(), []*unstructured.Unstructured{res1, res2}, DeployOptions{})

		require.NoError(t, err)
		require.True(t, changed)

		output := buf.String()
		// Should have two document separators (one per resource)
		require.Equal(t, 2, strings.Count(output, "---"))
		require.Contains(t, output, "kind: ConfigMap")
		require.Contains(t, output, "kind: Secret")
	})
}

func TestDryRunDeployManager_Disable(t *testing.T) {
	t.Run("disable single resource", func(t *testing.T) {
		dm := NewDryRunDeployManager()
		res := createTestResource("v1", "ConfigMap", "default", "test-cm")

		changed, err := dm.Disable(context.Background(), []*unstructured.Unstructured{res})

		require.NoError(t, err)
		require.True(t, changed)

		disabled := dm.GetDisabledResources()
		require.Len(t, disabled, 1)
		require.Equal(t, "ConfigMap", disabled[0].GetKind())
		require.Equal(t, "test-cm", disabled[0].GetName())
	})

	t.Run("disable multiple resources", func(t *testing.T) {
		dm := NewDryRunDeployManager()
		res1 := createTestResource("v1", "ConfigMap", "default", "cm1")
		res2 := createTestResource("v1", "Secret", "default", "secret1")

		changed, err := dm.Disable(context.Background(), []*unstructured.Unstructured{res1, res2})

		require.NoError(t, err)
		require.True(t, changed)

		disabled := dm.GetDisabledResources()
		require.Len(t, disabled, 2)
	})

	t.Run("disable empty list", func(t *testing.T) {
		dm := NewDryRunDeployManager()

		changed, err := dm.Disable(context.Background(), []*unstructured.Unstructured{})

		require.NoError(t, err)
		require.False(t, changed)
		require.Empty(t, dm.GetDisabledResources())
	})
}

func TestDryRunDeployManager_GetObjectCurrentState(t *testing.T) {
	t.Run("get existing resource", func(t *testing.T) {
		dm := NewDryRunDeployManager()
		res := createTestResource("v1", "ConfigMap", "default", "test-cm")
		dm.Deploy(context.Background(), []*unstructured.Unstructured{res}, DeployOptions{})

		ref := ObjectRef{
			APIVersion: "v1",
			Kind:       "ConfigMap",
			Namespace:  "default",
			Name:       "test-cm",
		}

		result, err := dm.GetObjectCurrentState(context.Background(), ref)

		require.NoError(t, err)
		require.NotNil(t, result)
		require.Equal(t, "ConfigMap", result.GetKind())
		require.Equal(t, "test-cm", result.GetName())
	})

	t.Run("get non-existent resource", func(t *testing.T) {
		dm := NewDryRunDeployManager()

		ref := ObjectRef{
			APIVersion: "v1",
			Kind:       "ConfigMap",
			Namespace:  "default",
			Name:       "missing",
		}

		result, err := dm.GetObjectCurrentState(context.Background(), ref)

		require.NoError(t, err)
		require.Nil(t, result)
	})

	t.Run("get returns deep copy", func(t *testing.T) {
		dm := NewDryRunDeployManager()
		res := createTestResource("v1", "ConfigMap", "default", "test-cm")
		dm.Deploy(context.Background(), []*unstructured.Unstructured{res}, DeployOptions{})

		ref := ObjectRef{
			APIVersion: "v1",
			Kind:       "ConfigMap",
			Namespace:  "default",
			Name:       "test-cm",
		}

		result1, _ := dm.GetObjectCurrentState(context.Background(), ref)
		result2, _ := dm.GetObjectCurrentState(context.Background(), ref)

		// Should be different pointers (deep copies)
		require.NotSame(t, result1, result2)

		// But same content
		require.Equal(t, result1.GetKind(), result2.GetKind())
		require.Equal(t, result1.GetName(), result2.GetName())
	})
}

func TestDryRunDeployManager_FilterObjectsCurrentState(t *testing.T) {
	t.Run("filter by kind", func(t *testing.T) {
		dm := NewDryRunDeployManager()
		cm1 := createTestResource("v1", "ConfigMap", "default", "cm1")
		cm2 := createTestResource("v1", "ConfigMap", "default", "cm2")
		secret := createTestResource("v1", "Secret", "default", "secret1")
		dm.Deploy(context.Background(), []*unstructured.Unstructured{cm1, cm2, secret}, DeployOptions{})

		filter := ObjectFilter{
			Kind: "ConfigMap",
		}

		results, err := dm.FilterObjectsCurrentState(context.Background(), filter)

		require.NoError(t, err)
		require.Len(t, results, 2)
		for _, res := range results {
			require.Equal(t, "ConfigMap", res.GetKind())
		}
	})

	t.Run("filter by namespace", func(t *testing.T) {
		dm := NewDryRunDeployManager()
		res1 := createTestResource("v1", "ConfigMap", "default", "cm1")
		res2 := createTestResource("v1", "ConfigMap", "kube-system", "cm2")
		res3 := createTestResource("v1", "ConfigMap", "default", "cm3")
		dm.Deploy(context.Background(), []*unstructured.Unstructured{res1, res2, res3}, DeployOptions{})

		filter := ObjectFilter{
			Namespace: "default",
		}

		results, err := dm.FilterObjectsCurrentState(context.Background(), filter)

		require.NoError(t, err)
		require.Len(t, results, 2)
		for _, res := range results {
			require.Equal(t, "default", res.GetNamespace())
		}
	})

	t.Run("filter by apiVersion", func(t *testing.T) {
		dm := NewDryRunDeployManager()
		res1 := createTestResource("v1", "ConfigMap", "default", "cm1")
		res2 := createTestResource("apps/v1", "Deployment", "default", "deploy1")
		dm.Deploy(context.Background(), []*unstructured.Unstructured{res1, res2}, DeployOptions{})

		filter := ObjectFilter{
			APIVersion: "apps/v1",
		}

		results, err := dm.FilterObjectsCurrentState(context.Background(), filter)

		require.NoError(t, err)
		require.Len(t, results, 1)
		require.Equal(t, "apps/v1", results[0].GetAPIVersion())
	})

	t.Run("filter by multiple criteria", func(t *testing.T) {
		dm := NewDryRunDeployManager()
		cm1 := createTestResource("v1", "ConfigMap", "default", "cm1")
		cm2 := createTestResource("v1", "ConfigMap", "kube-system", "cm2")
		secret := createTestResource("v1", "Secret", "default", "secret1")
		dm.Deploy(context.Background(), []*unstructured.Unstructured{cm1, cm2, secret}, DeployOptions{})

		filter := ObjectFilter{
			APIVersion: "v1",
			Kind:       "ConfigMap",
			Namespace:  "default",
		}

		results, err := dm.FilterObjectsCurrentState(context.Background(), filter)

		require.NoError(t, err)
		require.Len(t, results, 1)
		require.Equal(t, "ConfigMap", results[0].GetKind())
		require.Equal(t, "default", results[0].GetNamespace())
		require.Equal(t, "cm1", results[0].GetName())
	})

	t.Run("filter with no matches", func(t *testing.T) {
		dm := NewDryRunDeployManager()
		res := createTestResource("v1", "ConfigMap", "default", "cm1")
		dm.Deploy(context.Background(), []*unstructured.Unstructured{res}, DeployOptions{})

		filter := ObjectFilter{
			Kind: "Secret",
		}

		results, err := dm.FilterObjectsCurrentState(context.Background(), filter)

		require.NoError(t, err)
		require.Empty(t, results)
	})

	t.Run("empty filter returns all", func(t *testing.T) {
		dm := NewDryRunDeployManager()
		res1 := createTestResource("v1", "ConfigMap", "default", "cm1")
		res2 := createTestResource("v1", "Secret", "default", "secret1")
		dm.Deploy(context.Background(), []*unstructured.Unstructured{res1, res2}, DeployOptions{})

		filter := ObjectFilter{} // Empty filter

		results, err := dm.FilterObjectsCurrentState(context.Background(), filter)

		require.NoError(t, err)
		require.Len(t, results, 2)
	})
}

func TestDryRunDeployManager_SetStatus(t *testing.T) {
	t.Run("set status for resource", func(t *testing.T) {
		dm := NewDryRunDeployManager()
		ref := ObjectRef{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
			Namespace:  "default",
			Name:       "test-deploy",
		}

		status := map[string]interface{}{
			"replicas":      int64(3),
			"readyReplicas": int64(3),
		}

		changed, err := dm.SetStatus(context.Background(), ref, status)

		require.NoError(t, err)
		require.True(t, changed)

		retrievedStatus, exists := dm.GetStatus(ref)
		require.True(t, exists)
		require.Equal(t, int64(3), retrievedStatus["replicas"])
		require.Equal(t, int64(3), retrievedStatus["readyReplicas"])
	})

	t.Run("update existing status", func(t *testing.T) {
		dm := NewDryRunDeployManager()
		ref := ObjectRef{
			APIVersion: "v1",
			Kind:       "Pod",
			Namespace:  "default",
			Name:       "test-pod",
		}

		status1 := map[string]interface{}{
			"phase": "Pending",
		}

		dm.SetStatus(context.Background(), ref, status1)

		status2 := map[string]interface{}{
			"phase": "Running",
		}

		changed, err := dm.SetStatus(context.Background(), ref, status2)

		require.NoError(t, err)
		require.True(t, changed)

		retrievedStatus, exists := dm.GetStatus(ref)
		require.True(t, exists)
		require.Equal(t, "Running", retrievedStatus["phase"])
	})
}

func TestDryRunDeployManager_GetStatus(t *testing.T) {
	t.Run("get existing status", func(t *testing.T) {
		dm := NewDryRunDeployManager()
		ref := ObjectRef{
			APIVersion: "v1",
			Kind:       "Pod",
			Namespace:  "default",
			Name:       "test-pod",
		}

		status := map[string]interface{}{
			"phase": "Running",
		}

		dm.SetStatus(context.Background(), ref, status)

		retrievedStatus, exists := dm.GetStatus(ref)

		require.True(t, exists)
		require.Equal(t, "Running", retrievedStatus["phase"])
	})

	t.Run("get non-existent status", func(t *testing.T) {
		dm := NewDryRunDeployManager()
		ref := ObjectRef{
			APIVersion: "v1",
			Kind:       "Pod",
			Namespace:  "default",
			Name:       "missing",
		}

		status, exists := dm.GetStatus(ref)

		require.False(t, exists)
		require.Nil(t, status)
	})
}

func TestDryRunDeployManager_WatchObjects(t *testing.T) {
	dm := NewDryRunDeployManager()
	filter := ObjectFilter{
		Kind: "Pod",
	}

	watcher, err := dm.WatchObjects(context.Background(), filter)

	require.Error(t, err)
	require.Nil(t, watcher)
	require.Contains(t, err.Error(), "not supported")
}

func TestDryRunDeployManager_Reset(t *testing.T) {
	dm := NewDryRunDeployManager()

	// Add some data
	res1 := createTestResource("v1", "ConfigMap", "default", "cm1")
	res2 := createTestResource("v1", "Secret", "default", "secret1")
	dm.Deploy(context.Background(), []*unstructured.Unstructured{res1}, DeployOptions{})
	dm.Disable(context.Background(), []*unstructured.Unstructured{res2})
	dm.SetStatus(context.Background(), ObjectRef{
		APIVersion: "v1",
		Kind:       "Pod",
		Namespace:  "default",
		Name:       "pod1",
	}, map[string]interface{}{"phase": "Running"})

	// Reset
	dm.Reset()

	// Verify everything is cleared
	require.Empty(t, dm.GetDeployedResources())
	require.Empty(t, dm.GetDisabledResources())

	status, exists := dm.GetStatus(ObjectRef{
		APIVersion: "v1",
		Kind:       "Pod",
		Namespace:  "default",
		Name:       "pod1",
	})
	require.False(t, exists)
	require.Nil(t, status)
}

func TestDryRunDeployManager_ThreadSafety(t *testing.T) {
	dm := NewDryRunDeployManager()
	var wg sync.WaitGroup

	// Concurrent deploys
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			res := createTestResource("v1", "ConfigMap", "default", "cm-"+string(rune('a'+id)))
			dm.Deploy(context.Background(), []*unstructured.Unstructured{res}, DeployOptions{})
		}(i)
	}

	// Concurrent gets
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			dm.GetDeployedResources()
		}()
	}

	wg.Wait()

	// Should have 10 resources deployed
	deployed := dm.GetDeployedResources()
	require.Len(t, deployed, 10)
}

// Helper function to create test resources
func createTestResource(apiVersion, kind, namespace, name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": apiVersion,
			"kind":       kind,
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
			},
		},
	}
}
