# Flow - Workflow Orchestration Library

🌍 **Language Switch**: [中文文档](README-zh.md)

Flow is a powerful Go library for building and executing workflows, providing two execution modes: linear execution chain (Chain) and graphical executor (Graph).

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

## Benchmark Results

Benchmark results on Apple M1 Pro (darwin/arm64):

| Benchmark | Iterations | Time (ns/op) | Memory (B/op) | Allocations (allocs/op) |
|-------------|------------|----------------|----------------|-------------------------|
| BenchmarkC32-8 | 69,494 | 16,679 | 3,987 | 37 |
| BenchmarkS32-8 | 418,197 | 2,687 | 2,976 | 34 |
| BenchmarkC6-8 | 193,282 | 5,968 | 1,179 | 16 |
| BenchmarkC8x8-8 | 15,262 | 78,383 | 9,027 | 125 |

**Benchmark Descriptions:**

- **C32**: 32 independent nodes (fully parallel execution)
- **S32**: 32 nodes in sequential chain
- **C6**: 6 nodes with diamond dependencies
- **C8x8**: 8-layer x 8-nodes deep network (each layer fully connected to next)

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
        return 10
    })

    g.AddNode("process1", func(x int) int {
        return x * 2
    })

    g.AddNode("process2", func(x int) int {
        return x + 5
    })

    g.AddNode("end", func(x int) {
        fmt.Printf("Final result: %d\n", x)
    })

    g.AddEdge("start", "process1")
    g.AddEdge("process1", "process2")
    g.AddEdge("process2", "end")

    err := g.Run()
    if err != nil {
        fmt.Printf("Error: %v\n", err)
    }
}
```

## Documentation

For full documentation, see [docs/en-US/doc.md](docs/en-US/doc.md).

## Examples

See the [_examples](_examples) directory for more examples:

- [basic-chain](_examples/basic-chain/main.go) - Basic Chain usage
- [basic-graph](_examples/basic-graph/main.go) - Basic Graph usage
- [advanced-chain](_examples/advanced-chain/main.go) - Advanced Chain features
- [advanced-graph](_examples/advanced-graph/main.go) - Advanced Graph features
- [approval-flow](_examples/approval-flow/main.go) - Real-world approval workflow example

## License

MIT License
