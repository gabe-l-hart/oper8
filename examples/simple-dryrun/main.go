package main

import (
	"context"
	"fmt"
	"os"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/IBM/oper8/pkg/component"
	"github.com/IBM/oper8/pkg/controller"
	"github.com/IBM/oper8/pkg/deployer"
	"github.com/IBM/oper8/pkg/rollout"
	"github.com/IBM/oper8/pkg/session"
)

// DatabaseComponent represents a simple database component with a ConfigMap and Deployment
type DatabaseComponent struct {
	*component.BaseComponent
	Replicas int
}

func NewDatabaseComponent(replicas int) *DatabaseComponent {
	return &DatabaseComponent{
		BaseComponent: component.NewBase("database", false),
		Replicas:      replicas,
	}
}

func (c *DatabaseComponent) BuildChart(ctx context.Context, sess component.Session) error {
	// For this example, we'll just create the manifests inline
	// In a real application, you might use templates or a more sophisticated approach
	return nil
}

func (c *DatabaseComponent) Deploy(ctx context.Context, sess component.Session) error {
	// Get the full session (type assertion needed because component.Session is minimal interface)
	fullSess, ok := sess.(*session.Session)
	if !ok {
		return fmt.Errorf("expected *session.Session")
	}

	// Create ConfigMap
	configMap := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]interface{}{
				"name":      "database-config",
				"namespace": fullSess.Namespace(),
			},
			"data": map[string]interface{}{
				"database_url": "postgres://localhost:5432/mydb",
				"max_connections": "100",
			},
		},
	}

	// Create Deployment
	deployment := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name":      "database",
				"namespace": fullSess.Namespace(),
			},
			"spec": map[string]interface{}{
				"replicas": c.Replicas,
				"selector": map[string]interface{}{
					"matchLabels": map[string]interface{}{
						"app": "database",
					},
				},
				"template": map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels": map[string]interface{}{
							"app": "database",
						},
					},
					"spec": map[string]interface{}{
						"containers": []interface{}{
							map[string]interface{}{
								"name":  "postgres",
								"image": "postgres:14",
								"env": []interface{}{
									map[string]interface{}{
										"name": "POSTGRES_DB",
										"valueFrom": map[string]interface{}{
											"configMapKeyRef": map[string]interface{}{
												"name": "database-config",
												"key":  "database_url",
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	// Deploy resources using the deploy manager
	resources := []*unstructured.Unstructured{configMap, deployment}
	_, err := fullSess.DeployManager().Deploy(ctx, resources, deployer.DeployOptions{
		ManageOwnerReferences: true,
		Method:                deployer.DeployMethodDefault,
	})

	return err
}

// MyAppController manages the MyApp custom resource
type MyAppController struct {
	*controller.BaseController
}

func NewMyAppController() *MyAppController {
	return &MyAppController{
		BaseController: controller.NewBase(schema.GroupVersionKind{
			Group:   "example.com",
			Version: "v1",
			Kind:    "MyApp",
		}),
	}
}

func (c *MyAppController) SetupComponents(ctx context.Context, sess *session.Session) error {
	// Create database component with 3 replicas
	database := NewDatabaseComponent(3)

	// Register it with the session
	return sess.AddComponent(database)
}

func main() {
	fmt.Println("=== Go-Oper8 DryRun Example ===\n")

	// Create a sample CR manifest
	cr := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "example.com/v1",
			"kind":       "MyApp",
			"metadata": map[string]interface{}{
				"name":      "my-app-instance",
				"namespace": "default",
			},
			"spec": map[string]interface{}{
				"replicas": 3,
			},
		},
	}

	// Create a DryRun deploy manager that outputs to stdout
	dm := deployer.NewDryRunDeployManagerWithOutput(os.Stdout)

	// Create a session
	sess := session.New("example-reconciliation", cr, nil, dm)

	// Create controller
	ctrl := NewMyAppController()

	// Execute reconciliation
	ctx := context.Background()
	state, err := rollout.ExecuteReconciliation(ctx, ctrl, sess)

	if err != nil {
		fmt.Fprintf(os.Stderr, "\nError during reconciliation: %v\n", err)
		os.Exit(1)
	}

	// Print completion state
	fmt.Fprintf(os.Stderr, "\n=== Reconciliation Complete ===\n")
	fmt.Fprintf(os.Stderr, "%s\n", state.DetailedString())

	if !state.IsSuccessful() {
		os.Exit(1)
	}
}
