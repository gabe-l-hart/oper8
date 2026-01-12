# Go-Oper8

A Go implementation of the oper8 Kubernetes operator framework. Built with Go 1.21+ generics, designed to work with controller-runtime, and prioritizing local development through DryRun mode.

## Status: Phase 1 Complete ✅

Core abstractions and DryRun functionality implemented and working end-to-end.

**What's Working:**
- ✅ Generic type-safe DAG with cycle detection
- ✅ Component and Controller abstractions
- ✅ Session state management
- ✅ DryRun deploy manager for local development
- ✅ Complete reconciliation lifecycle
- ✅ YAML manifest generation

## Quick Start

```bash
# Run the DryRun example
cd examples/simple-dryrun
go run main.go
```

Output:
```yaml
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: database-config
  namespace: default
data:
  database_url: postgres://localhost:5432/mydb
  max_connections: "100"
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: database
  namespace: default
spec:
  replicas: 3
  selector:
    matchLabels:
      app: database
  ...
```

## Architecture

Go-Oper8 follows the same core architecture as Python Oper8:

- **Component**: Atomic grouping of Kubernetes resources (e.g., Deployment + ConfigMap + Service)
- **Controller**: Manages reconciliation for a CR type by orchestrating Components
- **Session**: Immutable reconciliation context shared across components
- **DAG**: Manages component dependencies and execution order

### Reconciliation Flow

```
Controller.SetupComponents()
    ↓
Build Component DAG
    ↓
Phase 1: Deploy Components (in dependency order)
    ↓
Controller.AfterDeploy()
    ↓
Phase 2: Verify Components
    ↓
Controller.AfterVerify()
```

## Example Usage

```go
// 1. Define a Component
type DatabaseComponent struct {
    *component.BaseComponent
    Replicas int
}

func (c *DatabaseComponent) Deploy(ctx context.Context, sess component.Session) error {
    // Create Kubernetes resources
    configMap := &unstructured.Unstructured{...}
    deployment := &unstructured.Unstructured{...}

    // Deploy via DeployManager
    resources := []*unstructured.Unstructured{configMap, deployment}
    _, err := fullSess.DeployManager().Deploy(ctx, resources, opts)
    return err
}

// 2. Define a Controller
type MyAppController struct {
    *controller.BaseController
}

func (c *MyAppController) SetupComponents(ctx context.Context, sess *session.Session) error {
    // Create components
    database := NewDatabaseComponent(3)
    backend := NewBackendComponent()

    // Register and define dependencies
    sess.AddComponent(database)
    sess.AddComponent(backend)
    sess.AddComponentDependency(backend, database, nil) // backend depends on database

    return nil
}

// 3. Execute Reconciliation with DryRun
func main() {
    // Create CR manifest
    cr := &unstructured.Unstructured{...}

    // Use DryRun to render YAML
    dm := deployer.NewDryRunDeployManagerWithOutput(os.Stdout)
    sess := session.New("reconcile-id", cr, nil, dm)

    // Run reconciliation
    ctrl := NewMyAppController()
    state, err := rollout.ExecuteReconciliation(context.Background(), ctrl, sess)
}
```

## Key Features

### 1. DryRun Mode (Priority Feature)

Render Kubernetes YAML without a cluster:

```go
dm := deployer.NewDryRunDeployManagerWithOutput(os.Stdout)
```

**Use Cases:**
- Local development and testing
- CI/CD manifest generation
- GitOps workflows
- Validation without deployment

### 2. Type-Safe DAG with Generics

```go
type Graph[T Node] struct {
    root     *NodeWrapper[T]
    nodeDict map[string]*NodeWrapper[T]
    mu       sync.RWMutex
}
```

- Go 1.21+ generics for compile-time type safety
- Automatic cycle detection
- Topological ordering for correct execution
- Thread-safe with RWMutex

### 3. Goroutine Pool Execution

```go
runner := dag.NewRunner("deploy", graph, deployFunc).
    WithWorkerPool(4).
    WithVerifyUpstream(true)
```

- Parallel component deployment where dependencies allow
- Configurable worker pool size
- Respects component dependencies
- Automatic error propagation

## Package Structure

```
github.com/IBM/oper8/
├── pkg/
│   ├── dag/           # Generic DAG with Runner
│   │   ├── graph.go
│   │   ├── node.go
│   │   ├── runner.go
│   │   └── completion.go
│   ├── component/     # Component abstraction
│   │   └── component.go
│   ├── controller/    # Controller interface
│   │   └── controller.go
│   ├── session/       # Session state
│   │   └── session.go
│   ├── deployer/      # K8s operations
│   │   ├── manager.go
│   │   └── dryrun.go
│   └── rollout/       # Orchestration
│       └── manager.go
├── examples/
│   └── simple-dryrun/ # Working example
└── oper8/             # Python implementation
```

## Implementation Phases

### ✅ Phase 1: Core + DryRun (Complete)
- Generic DAG implementation
- Component, Controller, Session abstractions
- DryRun deploy manager
- Rollout orchestration
- **Deliverable:** YAML rendering works end-to-end

### ⏳ Phase 2: Kubernetes Integration (Next)
- KubernetesDeployManager with controller-runtime client
- Server-side apply with conflict resolution
- Owner reference management
- Resource verification helpers
- Status condition management

### 📋 Phase 3: Resource Management (Future)
- ResourceNode for fine-grained resource DAG
- ManagedObject wrapper
- Temporary patch support

### 📋 Phase 4: Examples & Documentation (Future)
- Multi-component example with dependencies
- controller-runtime integration
- Migration guide from Python oper8
- API documentation

## Design Decisions

| Decision | Rationale |
|----------|-----------|
| Go 1.21+ Generics | Type-safe DAG without reflection or casting |
| DryRun First | Enable local development and testing without cluster |
| controller-runtime | Primary integration path for Go operators |
| Interface-based | Flexibility for users to implement custom logic |
| Embedding | BaseComponent/BaseController for code reuse |
| Goroutine Pool | Parallel execution with bounded resources |

## Comparison with Python Oper8

| Feature | Python | Go | Status |
|---------|--------|-----|--------|
| Component abstraction | ✅ | ✅ | Complete |
| Controller abstraction | ✅ | ✅ | Complete |
| DAG dependencies | ✅ | ✅ | Complete |
| Session state | ✅ | ✅ | Complete |
| DryRun mode | ✅ | ✅ | Complete |
| Deploy to cluster | ✅ | ⏳ | Phase 2 |
| Resource verification | ✅ | ⏳ | Phase 2 |
| Status management | ✅ | ⏳ | Phase 2 |
| Lifecycle hooks | ✅ | ✅ | Complete |
| VCS integration | ✅ | ❌ | Not planned |

## Testing

The DryRun example serves as both documentation and integration test:

```bash
cd examples/simple-dryrun
go run main.go

# Verify YAML output
go run main.go 2>/dev/null | kubectl apply --dry-run=client -f -
```

## Module Information

```
module github.com/IBM/oper8
go 1.21
```

## Contributing

This is the initial Go implementation of Oper8. The core architecture (Phase 1) is complete. Next steps:
1. KubernetesDeployManager for real cluster integration
2. More examples demonstrating component dependencies
3. Unit and integration tests

## License

See LICENSE file.

## Related Projects

- Python Oper8: `oper8/` directory in this repo
- [controller-runtime](https://github.com/kubernetes-sigs/controller-runtime): Go library for building operators
- [operator-sdk](https://github.com/operator-framework/operator-sdk): Toolkit for operator development
