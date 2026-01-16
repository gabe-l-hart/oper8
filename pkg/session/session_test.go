package session

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/IBM/oper8/pkg/component"
	"github.com/IBM/oper8/pkg/deployer"
)

// MockComponent implements component.Component for testing
type MockComponent struct {
	name string
}

func (m *MockComponent) GetName() string {
	return m.name
}

func (m *MockComponent) BuildChart(ctx context.Context, sess component.Session) error {
	return nil
}

func (m *MockComponent) Deploy(ctx context.Context, sess component.Session) error {
	return nil
}

func (m *MockComponent) Verify(ctx context.Context, sess component.Session) (bool, error) {
	return true, nil
}

func (m *MockComponent) Disable(ctx context.Context, sess component.Session) error {
	return nil
}

func (m *MockComponent) IsDisabled() bool {
	return false
}

func TestComponentNode(t *testing.T) {
	comp := &MockComponent{name: "test-component"}
	node := &ComponentNode{comp: comp}

	require.Equal(t, "test-component", node.GetName())
	require.Equal(t, comp, node.GetComponent())
}

func TestNew(t *testing.T) {
	t.Run("with all parameters", func(t *testing.T) {
		cr := createTestCR("test-app", "default", "TestKind")
		config := map[string]interface{}{
			"replicas": 3,
		}
		dm := deployer.NewDryRunDeployManager()

		sess := New("test-id", cr, config, dm)

		require.NotNil(t, sess)
		require.Equal(t, "test-id", sess.ID())
		require.Equal(t, cr, sess.CRManifest())
		require.Equal(t, config, sess.Config())
		require.Equal(t, dm, sess.DeployManager())
		require.NotNil(t, sess.graph)
		require.NotNil(t, sess.components)
	})

	t.Run("with nil config", func(t *testing.T) {
		cr := createTestCR("test-app", "default", "TestKind")
		dm := deployer.NewDryRunDeployManager()

		sess := New("test-id", cr, nil, dm)

		require.NotNil(t, sess)
		require.NotNil(t, sess.Config())
		require.Empty(t, sess.Config())
	})
}

func TestSession_Accessors(t *testing.T) {
	cr := createTestCR("test-app", "default", "TestKind")
	cr.Object["spec"] = map[string]interface{}{
		"replicas": int64(3),
		"image":    "nginx:latest",
	}
	cr.Object["status"] = map[string]interface{}{
		"ready": true,
	}

	config := map[string]interface{}{
		"globalConfig": "value",
	}
	dm := deployer.NewDryRunDeployManager()

	sess := New("test-id", cr, config, dm)

	t.Run("ID", func(t *testing.T) {
		require.Equal(t, "test-id", sess.ID())
	})

	t.Run("CRManifest", func(t *testing.T) {
		require.Equal(t, cr, sess.CRManifest())
	})

	t.Run("Name", func(t *testing.T) {
		require.Equal(t, "test-app", sess.Name())
	})

	t.Run("Namespace", func(t *testing.T) {
		require.Equal(t, "default", sess.Namespace())
	})

	t.Run("Kind", func(t *testing.T) {
		require.Equal(t, "TestKind", sess.Kind())
	})

	t.Run("APIVersion", func(t *testing.T) {
		require.Equal(t, "example.com/v1", sess.APIVersion())
	})

	t.Run("Spec", func(t *testing.T) {
		spec := sess.Spec()
		require.NotNil(t, spec)
		require.Equal(t, int64(3), spec["replicas"])
		require.Equal(t, "nginx:latest", spec["image"])
	})

	t.Run("Status", func(t *testing.T) {
		status := sess.Status()
		require.NotNil(t, status)
		require.Equal(t, true, status["ready"])
	})

	t.Run("Config", func(t *testing.T) {
		require.Equal(t, config, sess.Config())
		require.Equal(t, "value", sess.Config()["globalConfig"])
	})

	t.Run("DeployManager", func(t *testing.T) {
		require.Equal(t, dm, sess.DeployManager())
	})

	t.Run("GetName (dag.Node interface)", func(t *testing.T) {
		require.Equal(t, "test-app", sess.GetName())
	})
}

func TestSession_Accessors_NilCR(t *testing.T) {
	sess := New("test-id", nil, nil, deployer.NewDryRunDeployManager())

	t.Run("Name with nil CR", func(t *testing.T) {
		require.Equal(t, "", sess.Name())
	})

	t.Run("Namespace with nil CR", func(t *testing.T) {
		require.Equal(t, "", sess.Namespace())
	})

	t.Run("Kind with nil CR", func(t *testing.T) {
		require.Equal(t, "", sess.Kind())
	})

	t.Run("APIVersion with nil CR", func(t *testing.T) {
		require.Equal(t, "", sess.APIVersion())
	})

	t.Run("Spec with nil CR", func(t *testing.T) {
		require.Nil(t, sess.Spec())
	})

	t.Run("Status with nil CR", func(t *testing.T) {
		require.Nil(t, sess.Status())
	})
}

func TestSession_Spec_NoSpec(t *testing.T) {
	cr := createTestCR("test-app", "default", "TestKind")
	// Don't set spec
	sess := New("test-id", cr, nil, deployer.NewDryRunDeployManager())

	spec := sess.Spec()
	require.NotNil(t, spec)
	require.Empty(t, spec)
}

func TestSession_Status_NoStatus(t *testing.T) {
	cr := createTestCR("test-app", "default", "TestKind")
	// Don't set status
	sess := New("test-id", cr, nil, deployer.NewDryRunDeployManager())

	status := sess.Status()
	require.NotNil(t, status)
	require.Empty(t, status)
}

func TestSession_AddComponent(t *testing.T) {
	t.Run("add single component", func(t *testing.T) {
		sess := createTestSession()
		comp := &MockComponent{name: "database"}

		err := sess.AddComponent(comp)

		require.NoError(t, err)
		require.Equal(t, 1, sess.graph.NodeCount())

		retrieved, exists := sess.GetComponent("database")
		require.True(t, exists)
		require.Equal(t, comp, retrieved)
	})

	t.Run("add multiple components", func(t *testing.T) {
		sess := createTestSession()
		comp1 := &MockComponent{name: "database"}
		comp2 := &MockComponent{name: "backend"}
		comp3 := &MockComponent{name: "frontend"}

		require.NoError(t, sess.AddComponent(comp1))
		require.NoError(t, sess.AddComponent(comp2))
		require.NoError(t, sess.AddComponent(comp3))

		require.Equal(t, 3, sess.graph.NodeCount())

		all := sess.GetAllComponents()
		require.Len(t, all, 3)
	})

	t.Run("add duplicate component", func(t *testing.T) {
		sess := createTestSession()
		comp1 := &MockComponent{name: "database"}
		comp2 := &MockComponent{name: "database"} // Same name

		require.NoError(t, sess.AddComponent(comp1))
		err := sess.AddComponent(comp2)

		require.Error(t, err)
		require.Contains(t, err.Error(), "already exists")
	})
}

func TestSession_AddComponentDependency(t *testing.T) {
	t.Run("add dependency between components", func(t *testing.T) {
		sess := createTestSession()
		database := &MockComponent{name: "database"}
		backend := &MockComponent{name: "backend"}

		sess.AddComponent(database)
		sess.AddComponent(backend)

		// Backend depends on database
		err := sess.AddComponentDependency(backend, database, nil)

		require.NoError(t, err)
	})

	t.Run("add dependency with verify function", func(t *testing.T) {
		sess := createTestSession()
		database := &MockComponent{name: "database"}
		backend := &MockComponent{name: "backend"}

		sess.AddComponent(database)
		sess.AddComponent(backend)

		verifyCalled := false
		verifyFn := func(ctx context.Context) (bool, error) {
			verifyCalled = true
			return true, nil
		}

		err := sess.AddComponentDependency(backend, database, verifyFn)

		require.NoError(t, err)

		// Get the edge function and verify it works
		dbNode, _ := sess.graph.GetNode("database")
		backendNode, _ := sess.graph.GetNode("backend")
		edgeFn, err := sess.graph.GetEdgeFunc(dbNode, backendNode)

		require.NoError(t, err)
		require.NotNil(t, edgeFn)

		success, err := edgeFn(context.Background())
		require.NoError(t, err)
		require.True(t, success)
		require.True(t, verifyCalled)
	})

	t.Run("upstream component not found", func(t *testing.T) {
		sess := createTestSession()
		backend := &MockComponent{name: "backend"}
		database := &MockComponent{name: "database"} // Not added to session

		sess.AddComponent(backend)

		err := sess.AddComponentDependency(backend, database, nil)

		require.Error(t, err)
		require.Contains(t, err.Error(), "upstream component")
		require.Contains(t, err.Error(), "not found")
	})

	t.Run("dependent component not found", func(t *testing.T) {
		sess := createTestSession()
		database := &MockComponent{name: "database"}
		backend := &MockComponent{name: "backend"} // Not added to session

		sess.AddComponent(database)

		err := sess.AddComponentDependency(backend, database, nil)

		require.Error(t, err)
		require.Contains(t, err.Error(), "dependent component")
		require.Contains(t, err.Error(), "not found")
	})

	t.Run("cycle detection", func(t *testing.T) {
		sess := createTestSession()
		compA := &MockComponent{name: "A"}
		compB := &MockComponent{name: "B"}

		sess.AddComponent(compA)
		sess.AddComponent(compB)

		// A depends on B
		sess.AddComponentDependency(compA, compB, nil)

		// B depends on A (creates cycle)
		err := sess.AddComponentDependency(compB, compA, nil)

		require.Error(t, err)
		require.Contains(t, err.Error(), "cycle")
	})
}

func TestSession_GetComponent(t *testing.T) {
	sess := createTestSession()
	comp := &MockComponent{name: "database"}
	sess.AddComponent(comp)

	t.Run("get existing component", func(t *testing.T) {
		retrieved, exists := sess.GetComponent("database")
		require.True(t, exists)
		require.Equal(t, comp, retrieved)
	})

	t.Run("get non-existent component", func(t *testing.T) {
		retrieved, exists := sess.GetComponent("missing")
		require.False(t, exists)
		require.Nil(t, retrieved)
	})
}

func TestSession_GetAllComponents(t *testing.T) {
	t.Run("empty session", func(t *testing.T) {
		sess := createTestSession()
		all := sess.GetAllComponents()
		require.Empty(t, all)
	})

	t.Run("multiple components", func(t *testing.T) {
		sess := createTestSession()
		comp1 := &MockComponent{name: "database"}
		comp2 := &MockComponent{name: "backend"}
		comp3 := &MockComponent{name: "frontend"}

		sess.AddComponent(comp1)
		sess.AddComponent(comp2)
		sess.AddComponent(comp3)

		all := sess.GetAllComponents()
		require.Len(t, all, 3)

		names := make(map[string]bool)
		for _, comp := range all {
			names[comp.GetName()] = true
		}

		require.True(t, names["database"])
		require.True(t, names["backend"])
		require.True(t, names["frontend"])
	})
}

func TestSession_GetComponentTopology(t *testing.T) {
	t.Run("empty session", func(t *testing.T) {
		sess := createTestSession()
		topology := sess.GetComponentTopology()
		require.Empty(t, topology)
	})

	t.Run("components with dependencies", func(t *testing.T) {
		sess := createTestSession()
		database := &MockComponent{name: "database"}
		backend := &MockComponent{name: "backend"}
		frontend := &MockComponent{name: "frontend"}

		sess.AddComponent(database)
		sess.AddComponent(backend)
		sess.AddComponent(frontend)

		// Backend depends on database, frontend depends on backend
		sess.AddComponentDependency(backend, database, nil)
		sess.AddComponentDependency(frontend, backend, nil)

		topology := sess.GetComponentTopology()

		require.Len(t, topology, 3)

		// Build position map
		positions := make(map[string]int)
		for i, comp := range topology {
			positions[comp.GetName()] = i
		}

		// Database should come before backend, backend before frontend
		require.Less(t, positions["database"], positions["backend"])
		require.Less(t, positions["backend"], positions["frontend"])
	})
}

func TestSession_Graph(t *testing.T) {
	sess := createTestSession()
	comp := &MockComponent{name: "test"}
	sess.AddComponent(comp)

	graph := sess.Graph()

	require.NotNil(t, graph)
	require.Equal(t, 1, graph.NodeCount())
}

func TestSession_DeployManagerDelegation(t *testing.T) {
	t.Run("GetObjectCurrentState", func(t *testing.T) {
		dm := deployer.NewDryRunDeployManager()
		sess := createTestSessionWithDM(dm)

		// Deploy a resource first
		res := createTestResource("v1", "ConfigMap", "default", "test-cm")
		dm.Deploy(context.Background(), []*unstructured.Unstructured{res}, deployer.DeployOptions{})

		ref := deployer.ObjectRef{
			APIVersion: "v1",
			Kind:       "ConfigMap",
			Namespace:  "default",
			Name:       "test-cm",
		}

		result, err := sess.GetObjectCurrentState(context.Background(), ref)

		require.NoError(t, err)
		require.NotNil(t, result)
		require.Equal(t, "ConfigMap", result.GetKind())
	})

	t.Run("FilterObjectsCurrentState", func(t *testing.T) {
		dm := deployer.NewDryRunDeployManager()
		sess := createTestSessionWithDM(dm)

		res1 := createTestResource("v1", "ConfigMap", "default", "cm1")
		res2 := createTestResource("v1", "Secret", "default", "secret1")
		dm.Deploy(context.Background(), []*unstructured.Unstructured{res1, res2}, deployer.DeployOptions{})

		filter := deployer.ObjectFilter{
			Kind: "ConfigMap",
		}

		results, err := sess.FilterObjectsCurrentState(context.Background(), filter)

		require.NoError(t, err)
		require.Len(t, results, 1)
		require.Equal(t, "ConfigMap", results[0].GetKind())
	})

	t.Run("SetStatus", func(t *testing.T) {
		dm := deployer.NewDryRunDeployManager()
		sess := createTestSessionWithDM(dm)

		ref := deployer.ObjectRef{
			APIVersion: "v1",
			Kind:       "Pod",
			Namespace:  "default",
			Name:       "test-pod",
		}

		status := map[string]interface{}{
			"phase": "Running",
		}

		changed, err := sess.SetStatus(context.Background(), ref, status)

		require.NoError(t, err)
		require.True(t, changed)

		retrievedStatus, exists := dm.GetStatus(ref)
		require.True(t, exists)
		require.Equal(t, "Running", retrievedStatus["phase"])
	})
}

func TestSession_UtilityMethods(t *testing.T) {
	cr := createTestCR("my-app", "default", "TestKind")
	sess := New("test-id", cr, nil, deployer.NewDryRunDeployManager())

	t.Run("GetScopedName", func(t *testing.T) {
		result := sess.GetScopedName("database")
		require.Equal(t, "my-app-database", result)
	})

	t.Run("GetTruncatedName short name", func(t *testing.T) {
		result := sess.GetTruncatedName("short-name")
		require.Equal(t, "short-name", result)
	})

	t.Run("GetTruncatedName long name", func(t *testing.T) {
		longName := "this-is-a-very-long-name-that-exceeds-sixty-three-characters-limit"
		result := sess.GetTruncatedName(longName)

		require.Len(t, result, 63)
		require.Equal(t, longName[:63], result)
	})
}

func TestSession_ThreadSafety(t *testing.T) {
	sess := createTestSession()
	var wg sync.WaitGroup

	// Concurrent adds
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			comp := &MockComponent{name: string(rune('a' + id))}
			sess.AddComponent(comp)
		}(i)
	}

	// Concurrent reads
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sess.GetAllComponents()
			sess.GetComponentTopology()
		}()
	}

	wg.Wait()

	all := sess.GetAllComponents()
	require.Len(t, all, 10)
}

// Helper functions

func createTestSession() *Session {
	cr := createTestCR("test-app", "default", "TestKind")
	dm := deployer.NewDryRunDeployManager()
	return New("test-id", cr, nil, dm)
}

func createTestSessionWithDM(dm deployer.Manager) *Session {
	cr := createTestCR("test-app", "default", "TestKind")
	return New("test-id", cr, nil, dm)
}

func createTestCR(name, namespace, kind string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "example.com/v1",
			"kind":       kind,
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
			},
		},
	}
}

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
