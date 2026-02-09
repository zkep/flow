# Flow - Workflow Orchestration Library

Flow is a powerful Go library for building and executing workflows, providing two execution modes: linear execution chain (Chain) and graphical executor (Graph).

## Contents

- [Features](#features)
- [Core Concepts](#core-concepts)
- [Installation](#installation)
- [Quick Start](#quick-start)
- [API Examples](#api-examples)
  - [Chain API](#chain-api)
  - [Graph API](#graph-api)
  - [Edge Types](#edge-types)
  - [Execution Strategies](#execution-strategies)
  - [Checkpoint](#checkpoint)
  - [Pause and Resume](#pause-and-resume)
  - [Graph Visualization](#graph-visualization)
- [Advanced Features](#advanced-features)
- [Real-World Use Cases](#real-world-use-cases)
- [Error Handling](#error-handling)
- [Configuration Options](#configuration-options)

## Features

### Chain Mode

- **Linear Execution**: Execute tasks sequentially, step by step
- **Automatic Parameter Passing**: Output from one step is automatically passed as input to the next
- **Error Handling**: Comprehensive error propagation and handling
- **Simple & Easy**: Ideal for simple sequential processing scenarios

### Graph Mode

- **Graphical Workflows**: Build complex workflows with nodes and edges
- **Multiple Edge Types**: Support for normal, loop, and branch edges
- **Conditional Execution**: Add conditions to edges for controlled workflow paths
- **Parallel Execution**: Execute independent nodes concurrently for improved performance
- **Automatic Parameter Handling**: Smart parameter passing and type conversion between tasks
- **Error Handling**: Comprehensive error propagation and handling
- **Checkpoint Support**: Save and restore workflow state for fault tolerance
- **Pause/Resume**: Pause workflow execution and resume later
- **Visualization Support**: Generate Mermaid and Graphviz diagrams for workflow visualization

## Core Concepts

### Node

A **Node** represents a single task or operation in the workflow. Each node contains:

- **name**: A unique identifier for the node
- **fn**: The function to execute
- **status**: Current execution state (pending, running, completed, failed)
- **inputs/outputs**: Data flow connections to other nodes
- **result**: The return values after execution

```go
// Adding a node with a function
g.AddNode("calculate", func(x int) int {
    return x * 2
})
```

### Edge

An **Edge** defines the connection and data flow between nodes. Edges control:

- **from/to**: Source and target node names
- **edgeType**: Normal, Loop, or Branch execution flow
- **condition**: Optional conditional function to control execution path
- **weight**: Execution priority

```go
// Basic edge - data flows from start to process
g.AddEdge("start", "process")

// Conditional edge - only executes when condition is true
g.AddEdge("check", "action", flow.WithCondition(func(result []any) bool {
    return result[0].(int) > 10
}))

// Branch edge - for conditional branching
g.AddEdge("decision", "branchA", flow.WithEdgeType(flow.EdgeTypeBranch))
```

### How They Work Together

```
[Node A] ──Edge──> [Node B] ──Edge──> [Node C]
   │                                     ↑
   └───────────Edge─────────────────────┘
```

1. **Nodes** perform the actual work (functions)
2. **Edges** define execution order and data passing
3. When Node A completes, its output is passed to Node B via Edge
4. Edge conditions determine if Node B should execute

## Installation

```bash
go get github.com/zkep/flow
```

## Quick Start

### Basic Chain Example

```go
package main

import (
    "fmt"
    "github.com/zkep/flow"
)

func main() {
    chain := flow.NewChain()

    chain.Add("step1", func() int {
        return 10
    })

    chain.Add("step2", func(x int) int {
        return x * 2
    })

    chain.Add("step3", func(y int) int {
        return y + 5
    })

    err := chain.Run()
    if err != nil {
        fmt.Printf("Error: %v\n", err)
        return
    }

    result, err := chain.Value("step3")
    if err != nil {
        fmt.Printf("Error: %v\n", err)
        return
    }

    fmt.Printf("Final Result: %v\n", result) // Output: 25
}
```

### Basic Graph Example

```go
package main

import (
    "fmt"
    "github.com/zkep/flow"
)

func main() {
    g := flow.NewGraph()

    g.AddNode("start", func() int {
        fmt.Println("Executing start node")
        return 10
    })

    g.AddNode("process1", func(x int) int {
        fmt.Printf("Executing process1: %d * 2 = %d\n", x, x*2)
        return x * 2
    })

    g.AddNode("process2", func(x int) int {
        fmt.Printf("Executing process2: %d + 5 = %d\n", x, x+5)
        return x + 5
    })

    g.AddNode("end1", func(x int) {
        fmt.Printf("Executing end1 node: Final result is %d\n", x)
    })

    g.AddEdge("start", "process1")
    g.AddEdge("process1", "process2")
    g.AddEdge("process2", "end1")

    err := g.Run()
    if err != nil {
        fmt.Printf("Error: %v\n", err)
    } else {
        fmt.Println("Execution completed successfully")
    }
}
```

## API Examples

### Chain API

The Chain mode allows you to create linear workflows where each step executes in sequence, with the output of one step automatically passed as input to the next.

#### Creating a Chain

```go
chain := flow.NewChain()
```

#### Adding Steps

```go
chain.Add("stepName", func() int {
    return 42
})
```

#### Running the Chain

```go
err := chain.Run()
if err != nil {
    // Handle error
}

// Or with context
ctx := context.Background()
err := chain.RunWithContext(ctx)
```

#### Retrieving Results

```go
// Get a single value from a step
result, err := chain.Value("stepName")

// Get all values from a step
results, err := chain.Values("stepName")
```

#### Using Existing Steps with `Use`

The `Use` method allows you to create a new chain by selecting specific steps from an existing chain.

```go
originalChain := flow.NewChain()
originalChain.Add("loadData", func() []int {
    return []int{1, 2, 3, 4, 5}
})
originalChain.Add("processData", func(data []int) []int {
    var processed []int
    for _, num := range data {
        processed = append(processed, num*2)
    }
    return processed
})
originalChain.Add("saveData", func(data []int) error {
    fmt.Printf("Saving data: %v\n", data)
    return nil
})

originalChain.Run()

// Create a new chain using only specific steps
subsetChain := originalChain.Use("loadData", "processData")
subsetChain.Run()
```

#### Error Handling in Chains

```go
chain := flow.NewChain()

chain.Add("step1", func() int {
    return 42
})

chain.Add("step2", func(x int) (int, error) {
    if x < 0 {
        return 0, fmt.Errorf("invalid value")
    }
    return x * 2, nil
})

err := chain.Run()
if err != nil {
    fmt.Printf("Chain error: %v\n", err)
}
```

### Graph API

The Graph mode allows you to create complex workflows with nodes and edges, supporting different edge types and execution strategies.

#### Creating a Graph

```go
graph := flow.NewGraph()

// With options
graph := flow.NewGraph(
    flow.WithCapacity(64),
    flow.WithLargeGraphThreshold(256),
)
```

#### Adding Nodes

```go
// Simple node
graph.AddNode("process", func(x int) int {
    return x * 2
})

// Node with multiple inputs
graph.AddNode("combine", func(a, b int) int {
    return a + b
})

// Node with error return
graph.AddNode("validate", func(x int) (int, error) {
    if x < 0 {
        return 0, fmt.Errorf("invalid value")
    }
    return x, nil
})

// Node with no return (side effects)
graph.AddNode("log", func(x int) {
    fmt.Printf("Value: %d\n", x)
})
```

#### Adding Edges

```go
// Simple edge
graph.AddEdge("fromNode", "toNode")

// Edge with condition
graph.AddEdgeWithCondition("fromNode", "toNode", func(x int) bool {
    return x > 0
})

// Loop edge (for retry/loop scenarios)
graph.AddLoopEdge("retryNode", func(result int) bool {
    return result < 100
}, 3) // max 3 iterations

// Branch edge (multiple conditional paths)
graph.AddBranchEdge("decisionNode", map[string]any{
    "pathA": func(result int) bool { return result > 50 },
    "pathB": func(result int) bool { return result <= 50 },
})
```

#### Running the Graph

```go
// Run the graph with default parallel execution
err := graph.Run()

// Run with context
ctx := context.Background()
err := graph.RunWithContext(ctx)

// Run sequentially
err := graph.RunSequential()
err := graph.RunSequentialWithContext(ctx)
```

#### Retrieving Node Information

```go
// Get node status
status, err := graph.NodeStatus("nodeName")

// Get node result
result, err := graph.NodeResult("nodeName")

// Get node error
err := graph.NodeError("nodeName")

// Get nodes by status
pendingNodes := graph.GetNodesByStatus(flow.NodeStatusPending)
completedNodes := graph.GetNodesByStatus(flow.NodeStatusCompleted)
```

#### Clearing Graph Status

```go
graph.ClearStatus()
```

### Edge Types

| Edge Type | Description | Example |
|-----------|-------------|----------|
| Normal | Standard edge connecting two nodes | `AddEdge("a", "b")` |
| Loop | Edge for loop/retry operations (same source and target node) | `AddLoopEdge("a", cond, maxIter)` |
| Branch | Edge with conditional branching to multiple target nodes | `AddBranchEdge("a", branches)` |

### Execution Strategies

#### Parallel Execution (Default)

Independent nodes are executed concurrently for improved performance. The graph executor automatically handles parallel execution when possible.

```go
// Default parallel execution
err := graph.Run()
```

#### Sequential Execution

Nodes are executed one after another in topological order.

```go
err := graph.RunSequential()
```

### Checkpoint

Flow provides checkpoint functionality to save and restore workflow state.

#### Creating a Checkpoint Store

```go
// File-based checkpoint store
store, err := flow.NewFileCheckpointStore("/path/to/checkpoints")

// In-memory checkpoint store
store := flow.NewMemoryCheckpointStore()
```

#### Saving and Loading Checkpoints

```go
// Save checkpoint
checkpoint, err := graph.SaveCheckpoint()
err = store.Save("my-flow", checkpoint)

// Load checkpoint
checkpoint, err = store.Load("my-flow")
err = graph.LoadCheckpoint(checkpoint)

// List all checkpoints
keys, err := store.List()

// Delete a checkpoint
err = store.Delete("my-flow")
```

#### Checkpoint Interface

```go
type FlowCheckpointable interface {
    SaveCheckpoint() (*Checkpoint, error)
    LoadCheckpoint(checkpoint *Checkpoint) error
    SaveToStore(store CheckpointStore, key string) error
    LoadFromStore(store CheckpointStore, key string) error
    Reset()
}
```

### Pause and Resume

Flow supports pausing and resuming workflow execution.

#### Basic Pause/Resume

```go
// Pause execution
err := graph.Pause()

// Resume execution
ctx := context.Background()
err := graph.Resume(ctx)
```

#### Pause Configuration

```go
config := flow.NewPauseConfig()

// Pause at specific nodes
config.SetPauseAtNodes("node1", "node2")

// Pause on error
config.SetPauseOnError()

// Apply configuration
graph.SetPauseConfig(config)
err := graph.PauseWithConfig(config)
```

#### Resume Configuration

```go
config := flow.NewResumeConfig()

// Skip completed nodes
config.SkipCompleted = true

// Retry failed nodes
config.SetRetryFailed()

// Resume with configuration
err := graph.ResumeWithConfig(ctx, config)
```

#### Pause Signal

```go
signal := flow.NewSimplePauseSignal()
graph.SetPauseSignal(signal)

// Trigger pause
signal.SetPaused(true)

// Reset signal
signal.Reset()
```

#### Resource Checking

```go
checker := flow.NewSimpleResourceChecker(100, 10)
graph.SetResourceChecker(checker)

// Check availability
available := checker.CheckAvailable("nodeName")

// Consume/release resources
checker.Consume()
checker.Release()
```

#### Getting Flow State

```go
state := graph.State()

// States: FlowStateIdle, FlowStateRunning, FlowStatePaused, FlowStateCompleted, FlowStateFailed
if state == flow.FlowStatePaused {
    pausedAt := graph.GetPausedAtNode()
    fmt.Printf("Paused at node: %s\n", pausedAt)
}
```

### Graph Visualization

Flow supports generating diagrams for visualization.

#### Mermaid Diagram

```go
mermaid := graph.Mermaid()
fmt.Println(mermaid)
```

Output:
```
graph TD
    start --> process1
    process1 --> process2
    process2 --> end
```

#### Graphviz Diagram

```go
graphviz := graph.String()
fmt.Println(graphviz)
```

Output:
```
digraph Graph {
    rankdir=TD;

    "start" [shape=box,label="start"];
    "process1" [shape=box,label="process1"];
    "process2" [shape=box,label="process2"];
    "end" [shape=box,label="end"];

    "start" -> "process1" [];
    "process1" -> "process2" [];
    "process2" -> "end" [];
}
```

## Advanced Features

### Conditional Execution

Use `AddEdgeWithCondition` to add conditions to edges, allowing for dynamic workflow paths based on runtime values.

```go
graph.AddNode("check", func(x int) int {
    return x
})

graph.AddNode("high", func(x int) string {
    return "high value"
})

graph.AddNode("low", func(x int) string {
    return "low value"
})

graph.AddEdgeWithCondition("check", "high", func(x int) bool {
    return x > 100
})

graph.AddEdgeWithCondition("check", "low", func(x int) bool {
    return x <= 100
})
```

### Loop Execution

Use `AddLoopEdge` to create loop scenarios with automatic retry support.

```go
graph.AddNode("retry", func(attempt int) (int, error) {
    fmt.Printf("Attempt %d\n", attempt)
    if attempt < 3 {
        return attempt + 1, fmt.Errorf("failed")
    }
    return attempt, nil
})

graph.AddLoopEdge("retry", func(result int, err error) bool {
    return err != nil
}, 5) // Max 5 iterations
```

### Branch Execution

Use `AddBranchEdge` to create conditional branching to multiple target nodes.

```go
graph.AddNode("decision", func(x int) string {
    if x > 50 {
        return "approve"
    }
    return "reject"
})

graph.AddNode("approve", func(decision string) {
    fmt.Println("Approved")
})

graph.AddNode("reject", func(decision string) {
    fmt.Println("Rejected")
})

graph.AddBranchEdge("decision", map[string]any{
    "approve": func(decision string) bool { return decision == "approve" },
    "reject":  func(decision string) bool { return decision == "reject" },
})
```

### Parallel Execution

The graph executor automatically handles parallel execution of independent nodes when possible.

```go
graph.AddNode("start", func() int {
    return 10
})

graph.AddNode("parallel1", func(x int) int {
    time.Sleep(100 * time.Millisecond)
    return x * 2
})

graph.AddNode("parallel2", func(x int) int {
    time.Sleep(100 * time.Millisecond)
    return x + 5
})

graph.AddNode("combine", func(a, b int) int {
    return a + b
})

graph.AddEdge("start", "parallel1")
graph.AddEdge("start", "parallel2")
graph.AddEdge("parallel1", "combine")
graph.AddEdge("parallel2", "combine")

// parallel1 and parallel2 execute concurrently
graph.Run()
```

### Type Conversion

Flow automatically handles type conversion between nodes when possible.

```go
graph.AddNode("int_to_string", func(x int) string {
    return fmt.Sprintf("Number: %d", x)
})

graph.AddNode("string_to_int", func(s string) int {
    return len(s)
})

graph.AddEdge("int_to_string", "string_to_int")
```

### Large Graph Optimization

For graphs with many nodes, Flow provides optimized execution.

```go
graph := flow.NewGraph(
    flow.WithLargeGraphThreshold(256),
)

// Large graphs use a layer-based parallel execution strategy
graph.Run()
```

## Real-World Use Cases

### Data Processing Pipeline

```go
chain := flow.NewChain()

chain.Add("loadData", func() []string {
    return []string{"data1", "data2", "data3"}
})

chain.Add("cleanData", func(data []string) []string {
    var cleaned []string
    for _, item := range data {
        if item != "" {
            cleaned = append(cleaned, strings.TrimSpace(item))
        }
    }
    return cleaned
})

chain.Add("transformData", func(data []string) []map[string]string {
    var transformed []map[string]string
    for _, item := range data {
        transformed = append(transformed, map[string]string{"value": item})
    }
    return transformed
})

chain.Add("saveData", func(data []map[string]string) error {
    for _, item := range data {
        fmt.Printf("Saving: %v\n", item)
    }
    return nil
})

if err := chain.Run(); err != nil {
    fmt.Printf("Pipeline failed: %v\n", err)
}
```

### Business Process Automation

See the [approval-flow](_examples/approval-flow/main.go) example for a complete customer onboarding workflow.

### ETL (Extract, Transform, Load) Workflow

See the [advanced-graph](_examples/advanced-graph/main.go) example for a complete ETL process.

### Order Processing

See the [advanced-graph](_examples/advanced-graph/main.go) example for a complete order processing workflow.

## Error Handling

Flow provides comprehensive error handling.

### Error Types

| Error Constant | Description |
|---------------|-------------|
| `ErrArgTypeMismatch` | Argument type doesn't match expected type |
| `ErrArgCountMismatch` | Argument count doesn't match expected count |
| `ErrNotFunction` | Provided value is not a function |
| `ErrFunctionPanicked` | Function execution caused a panic |
| `ErrStepNotFound` | Step with given name not found |
| `ErrNodeNotFound` | Node with given name not found |
| `ErrDuplicateNode` | Node with same name already exists |
| `ErrSelfDependency` | Node cannot depend on itself |
| `ErrCyclicDependency` | Cyclic dependency detected in graph |
| `ErrNoStartNode` | No start node found in graph |
| `ErrExecutionFailed` | Execution failed |
| `ErrFlowPaused` | Flow is paused |
| `ErrResourceNotAvailable` | Resource not available |
| `ErrCheckpointNotFound` | Checkpoint not found |
| `ErrInvalidCheckpoint` | Invalid checkpoint data |

### Error Propagation

Errors are automatically propagated through the workflow:

```go
graph.AddNode("step1", func() (int, error) {
    return 0, fmt.Errorf("step1 failed")
})

graph.AddNode("step2", func(x int) int {
    return x * 2 // Never executed
})

graph.AddEdge("step1", "step2")

err := graph.Run()
if err != nil {
    // err contains "step1 failed"
}
```

## Configuration Options

### Graph Options

| Option | Description | Default |
|--------|-------------|----------|
| `WithCapacity(capacity)` | Set initial capacity for internal maps | 32 |
| `WithLargeGraphThreshold(threshold)` | Threshold for large graph optimization | 128 |

### Node Status

| Status | Value | Description |
|--------|-------|-------------|
| `NodeStatusPending` | 0 | Node waiting to execute |
| `NodeStatusRunning` | 1 | Node currently executing |
| `NodeStatusCompleted` | 2 | Node completed successfully |
| `NodeStatusFailed` | 3 | Node execution failed |

### Flow State

| State | Value | Description |
|-------|-------|-------------|
| `FlowStateIdle` | 0 | Flow not started |
| `FlowStateRunning` | 1 | Flow currently running |
| `FlowStatePaused` | 2 | Flow paused |
| `FlowStateCompleted` | 3 | Flow completed |
| `FlowStateFailed` | 4 | Flow failed |

### Pause Mode

| Mode | Description |
|------|-------------|
| `PauseModeImmediate` | Pause immediately |
| `PauseModeAtNode` | Pause at specific nodes |
| `PauseModeOnError` | Pause when error occurs |
