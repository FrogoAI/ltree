## LTree: A Go Package for Layered Dependency Execution

`ltree` is a powerful and generic Go package designed to manage and execute a directed acyclic graph (DAG) of tasks. It organizes tasks into "layers" based on their dependencies and executes them in a structured, concurrent manner.

The core principle is to ensure that no task is executed until all of its dependencies have been successfully completed. It achieves this by first resolving the entire dependency graph into sequential layers and then executing layer by layer. Within each layer, it can run tasks in parallel (asynchronous) or sequentially (synchronous), providing fine-grained control over execution flow.

### Core Concepts

The package is built around a few key abstractions:

- **`Executor`**: An interface that represents a single task or unit of work in the tree. Any type that implements this interface can be added to the tree. It defines the task's key, its value, its dependencies, whether it's asynchronous, and if it should be skipped.
- **`Entry`**: The default, concrete implementation of the `Executor` interface. You use `NewEntry` to create the tasks you want to add to the tree.
- **`TreeLayer`**: The main orchestrator. It holds the entire graph, resolves dependencies, detects errors, and controls the execution process.
- **Layers**: Tasks are internally organized into layers.
    - **Layer 0** contains all tasks with no dependencies.
    - **Layer 1** contains all tasks whose dependencies are exclusively in Layer 0.
    - **Layer N** contains all tasks whose dependencies are in layers `< N`. This structure guarantees that when the executor is processing a given layer, all prerequisite tasks have already been completed.

### Key Features

- **Automatic Dependency Resolution**: Automatically calculates the correct execution order based on the dependencies (`DependsOn`) you define for each task.
- **Layered Execution**: Executes tasks in sequential layers, ensuring correctness in complex data processing or initialization pipelines.
- **Concurrency Control**: Within each layer, tasks marked as **asynchronous (`IsAsync`) are run in parallel** (each in its own goroutine), while **synchronous tasks are run sequentially** in a single goroutine. This optimizes resource usage while respecting constraints.
- **Circular Dependency Detection**: Before execution, the tree validates the dependency graph. If a circular dependency is found (e.g., A depends on B, and B depends on A), the `Add` method will immediately return an `ErrCircuitDependency`.
- **Context-Aware Execution**: The `Execute` method accepts a `context.Context`, allowing for graceful cancellation and timeouts across all running tasks.
- **Conditional Execution**: Tasks can provide a `Skip` function. Before a task is executed, this function is called, and if it returns `true`, the task is skipped.
- **Type-Safe Generics**: The package uses Go generics (`[K comparable, V any]`), allowing you to use any comparable type for task keys (like `string` or `int`) and any type for their values.
- **Concurrency-Safe Structure**: The `TreeLayer` is protected by a `sync.RWMutex`, making it safe to inspect its state from multiple goroutines (though `Add` should not be called concurrently with `Execute`).

### How It Works

The process is divided into two main phases: the Build Phase and the Execution Phase.

#### 1. Build Phase (`Add` method)

1. **Node Creation**: When you call `Add` with a list of `Executor`s, the `TreeLayer` creates an internal `Node` for each task and stores them in a dictionary for quick key-based lookups.
2. **Graph Traversal**: The tree then traverses the dependency list for each node. Using a DFS-like approach (`visit` method), it walks up the dependency chain.
3. **Level Calculation**: During this traversal, it calculates the `level` for each node. A node's level is determined by the highest level of its parents, plus one (`n.level = v.level + 1`). This is how the layered structure is formed.
4. **Cycle Detection**: While traversing, it checks if it encounters a node that is already in its own parental lineage. If so, it identifies a circuit and returns `ErrCircuitDependency`.
5. **Layer Population**: Once all nodes have been visited and their levels are set, they are grouped into the corresponding `layers` map within the `TreeLayer`.

#### 2. Execution Phase (`Execute` method)

1. **Layer Iteration**: The `Execute` method iterates sequentially from layer 0 to the maximum layer (`maxLayer`).
2. **Task Distribution**: For each layer, it retrieves the nodes and separates them into two channels: one for asynchronous tasks and one for synchronous tasks.
3. **Concurrent Execution**:
    - A single goroutine is spawned to process all synchronous tasks for the current layer.
    - A dedicated goroutine is spawned for *each* asynchronous task, allowing them to run fully in parallel.
4. **Synchronization**: A `sync.WaitGroup` is used to wait for **all tasks in the current layer to complete** before the execution proceeds to the next layer. This is the crucial step that enforces the dependency hierarchy.
5. **Context Cancellation**: Throughout the process, all goroutines monitor the `context.Context`. If the context is canceled, the execution loops terminate early, stopping the process.

