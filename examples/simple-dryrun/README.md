# Simple DryRun Example

This example demonstrates the core Go-Oper8 functionality using DryRun mode to render Kubernetes YAML without a cluster.

## What This Example Shows

1. **Component Definition**: `DatabaseComponent` defines a logical grouping of Kubernetes resources (ConfigMap + Deployment)
2. **Controller Setup**: `MyAppController` manages a custom resource and defines which components to deploy
3. **DryRun Mode**: Uses `DryRunDeployManager` to render YAML to stdout instead of applying to a cluster
4. **Full Reconciliation Flow**: Demonstrates the complete oper8 reconciliation lifecycle

## Running the Example

```bash
go run main.go
```

This will output valid Kubernetes YAML that can be piped to kubectl:

```bash
go run main.go > manifests.yaml
kubectl apply -f manifests.yaml
```

## Code Walkthrough

### 1. Component Definition

```go
type DatabaseComponent struct {
    *component.BaseComponent
    Replicas int
}
```

Components are atomic units that manage related Kubernetes resources. This example creates a ConfigMap and Deployment.

### 2. Resource Generation

In `Deploy()`, the component creates Kubernetes resource manifests as `unstructured.Unstructured` objects and passes them to the `DeployManager`.

### 3. Controller Orchestration

```go
func (c *MyAppController) SetupComponents(ctx context.Context, sess *session.Session) error {
    database := NewDatabaseComponent(3)
    return sess.AddComponent(database)
}
```

The controller's `SetupComponents()` method defines what gets deployed during reconciliation.

### 4. DryRun Execution

```go
dm := deployer.NewDryRunDeployManagerWithOutput(os.Stdout)
sess := session.New("example-reconciliation", cr, nil, dm)
```

By using `DryRunDeployManager` with an output writer, all "deployed" resources are written as YAML instead of being applied to a cluster.

## Key Features Demonstrated

- ✅ Component-based architecture
- ✅ DAG-based dependency management (single component in this example)
- ✅ Type-safe generics for DAG operations
- ✅ DryRun mode for local development
- ✅ YAML manifest generation
- ✅ Full reconciliation lifecycle (setup → deploy → verify)

## Next Steps

For more complex examples showing:
- Multiple components with dependencies
- Custom verification logic
- Controller lifecycle hooks
- Integration with controller-runtime

See the other examples in the `examples/` directory (coming soon).
