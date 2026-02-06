# Flow - Workflow Orchestration Library

🌍 **Language Switch**: [中文文档](README-zh.md)

Flow is a powerful Go library for building and executing workflows, providing two execution modes: linear execution chain (Chain) and graphical executor (Graph).

## Features

- **Linear Workflows (Chain)**: Execute tasks in a sequential manner with automatic parameter passing
- **Graphical Workflows (Graph)**: Build complex workflows with nodes and edges, supporting different node types
- **Multiple Node Types**: Support for start, end, branch, parallel, and loop nodes
- **Conditional Execution**: Add conditions to edges for controlled workflow paths
- **Parallel Execution**: Execute independent nodes concurrently for improved performance
- **Automatic Parameter Handling**: Smart parameter passing and type conversion between tasks
- **Error Handling**: Comprehensive error propagation and handling
- **Visualization Support**: Generate Mermaid and Graphviz diagrams for workflow visualization
- **Flexible Execution Strategies**: Choose between sequential and parallel execution

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

### Using Existing Steps with `Use`

The `Use` method allows you to create a new chain by selecting specific steps from an existing chain. This is particularly useful when you want to reuse certain steps from a previously executed chain or create a subset of steps for further processing.

#### Example: Creating a Subset of Steps

```go
package main

import (
    "fmt"
    "github.com/zkep/flow"
)

func main() {
    // Create and run a full chain
    originalChain := flow.NewChain()
    
    originalChain.Add("loadData", func() []int {
        return []int{1, 2, 3, 4, 5}
    })
    
    originalChain.Add("filterData", func(data []int) []int {
        var filtered []int
        for _, num := range data {
            if num > 2 {
                filtered = append(filtered, num)
            }
        }
        return filtered
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
    
    fmt.Println("Running original chain:")
    err := originalChain.Run()
    if err != nil {
        fmt.Printf("Error: %v\n", err)
        return
    }
    
    // Create a new chain using only specific steps
    // This allows us to reuse the data loading and processing steps
    fmt.Println("\nRunning subset chain:")
    subsetChain := originalChain.Use("loadData", "processData")
    
    err = subsetChain.Run()
    if err != nil {
        fmt.Printf("Error: %v\n", err)
        return
    }
    
    // Get results from the subset chain
    result, err := subsetChain.Value("processData")
    if err != nil {
        fmt.Printf("Error: %v\n", err)
        return
    }
    
    fmt.Printf("Subset chain result: %v\n", result) // Output: [2 4 6 8 10]
}
```

#### Key Use Cases for `Use`

1. **Reusing Steps**: Extract specific steps from a complex chain to reuse them in different contexts
2. **Partial Processing**: Create a chain that only executes a subset of steps for focused processing
3. **Step Isolation**: Test individual steps or groups of steps independently
4. **Dynamic Workflow Construction**: Build new workflows on-the-fly by selecting steps from existing chains
5. **Performance Optimization**: Avoid re-executing unnecessary steps by creating targeted chains

The `Use` method maintains the original step names and their order, ensuring consistent behavior when creating subsets of steps.

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

### Graph Visualization

```mermaid
graph TD
    start --> process1
    process1 --> process2
    process2 --> end1
```

## Usage

### Chain

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
```

#### Retrieving Results

```go
// Get a single value from a step
result, err := chain.Value("stepName")

// Get all values from a step
results, err := chain.Values("stepName")
```

### Graph

The Graph mode allows you to create complex workflows with nodes and edges, supporting different node types and execution strategies.

#### Creating a Graph

```go
graph := flow.NewGraph()
```

#### Adding Nodes

```go
// Add a node
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
// Run the graph
err := graph.Run()

// Run with context
ctx := context.Background()
err := graph.RunWithContext(ctx)
```

#### Retrieving Node Information

```go
// Get node status
status := graph.NodeStatus("nodeName")

// Get node result
result := graph.NodeResult("nodeName")

// Get node error
err := graph.NodeError("nodeName")
```

#### Visualization

```go
// Generate Mermaid diagram
mermaid := graph.Mermaid()
fmt.Println(mermaid)

// Generate Graphviz diagram
graphviz := graph.String()
fmt.Println(graphviz)
```

## Edge Types

| Edge Type | Description |
|-----------|-------------|
| Normal | Standard edge connecting two nodes |
| Loop | Edge for loop/retry operations (same source and target node) |
| Branch | Edge with conditional branching to multiple target nodes |

## Execution Strategies

- **Sequential Execution**: Nodes are executed one after another in topological order
- **Parallel Execution**: Independent nodes are executed concurrently for improved performance

## Advanced Features

### Conditional Execution

Use `AddEdgeWithCondition` to add conditions to edges, allowing for dynamic workflow paths based on runtime values.

### Parallel Execution

The graph executor automatically handles parallel execution of independent nodes when possible, which can significantly improve performance for workflows with many independent tasks.

### Error Handling

Flow automatically propagates errors through the workflow, stopping execution when an error occurs.

### Parameter Handling

Flow automatically handles parameter passing between nodes, including type conversion when possible.

## Real-World Use Cases

### 1. Data Processing Pipeline

**Scenario**: Processing large datasets with multiple transformation steps

**Implementation**: 
- Use `Chain` for sequential data processing steps
- Each step transforms the data and passes it to the next
- Add error handling at each step to catch data anomalies

**Example**: 
```go
chain := flow.NewChain()

chain.Add("loadData", func() []string {
    // Load data from file/database
    return []string{"data1", "data2", "data3"}
})

chain.Add("cleanData", func(data []string) []string {
    // Clean and validate data
    var cleaned []string
    for _, item := range data {
        if item != "" {
            cleaned = append(cleaned, strings.TrimSpace(item))
        }
    }
    return cleaned
})

chain.Add("transformData", func(data []string) []map[string]string {
    // Transform data into structured format
    var transformed []map[string]string
    for _, item := range data {
        transformed = append(transformed, map[string]string{"value": item})
    }
    return transformed
})

chain.Add("saveData", func(data []map[string]string) error {
    // Save data to database
    for _, item := range data {
        // Save item to database
        fmt.Printf("Saving: %v\n", item)
    }
    return nil
})

if err := chain.Run(); err != nil {
    fmt.Printf("Pipeline failed: %v\n", err)
}
```

### 2. Business Process Automation

**Scenario**: Automating a customer onboarding process with credit check, background verification, and approval workflow

**Implementation**: 
- Use `Graph` to model complex approval workflows
- Use `AddLoopEdge` for credit check retries
- Use `AddBranchEdge` for conditional approval/rejection paths
- Use parallel execution for independent verification steps

**Example**: 
```go
graph := flow.NewGraph()

// Collect customer information
graph.AddNode("collectInfo", func() map[string]string {
    return map[string]string{
        "name":  "John Doe",
        "email": "john@example.com",
        "score": "85",
    }
})

// Credit check
graph.AddNode("creditCheck", func(info map[string]string) (int, error) {
    score, _ := strconv.Atoi(info["score"])
    fmt.Printf("Credit check: score = %d\n", score)
    return score, nil
})

// Retry credit check (loop node)
graph.AddNode("retryCreditCheck", func(score int) int {
    fmt.Printf("Retrying credit check, current score: %d\n", score)
    return score + 5
})
// Add loop edge with condition and max iterations
graph.AddLoopEdge("retryCreditCheck", func(score int) bool {
    return score < 70
}, 3)

// Evaluate credit score
graph.AddNode("evaluateCredit", func(score int) bool {
    return score >= 70
})

// Background verification
graph.AddNode("backgroundCheck", func(info map[string]string) bool {
    time.Sleep(100 * time.Millisecond)
    return true
})

// Document verification
graph.AddNode("documentCheck", func(info map[string]string) bool {
    time.Sleep(150 * time.Millisecond)
    return true
})

// Approval decision
graph.AddNode("approval", func(creditOk, backgroundOk, documentOk bool) string {
    if creditOk && backgroundOk && documentOk {
        return "approve"
    }
    return "reject"
})

// Send approval notification
graph.AddNode("sendApproval", func(decision string) {
    fmt.Printf("Approving customer (decision: %s)\n", decision)
})

// Send rejection notification
graph.AddNode("sendRejection", func(decision string) {
    fmt.Printf("Rejecting customer (decision: %s)\n", decision)
})

// Onboarding complete
graph.AddNode("onboardingComplete", func() {
    fmt.Println("Customer onboarding completed successfully")
})

// Onboarding failed
graph.AddNode("onboardingFailed", func() {
    fmt.Println("Customer onboarding failed")
})

// Add edges
graph.AddEdge("collectInfo", "creditCheck")
graph.AddEdge("collectInfo", "backgroundCheck")
graph.AddEdge("collectInfo", "documentCheck")
graph.AddEdge("creditCheck", "retryCreditCheck")
graph.AddEdge("retryCreditCheck", "evaluateCredit")
graph.AddEdge("evaluateCredit", "approval")
graph.AddEdge("backgroundCheck", "approval")
graph.AddEdge("documentCheck", "approval")
// Branch edge for approval/rejection
graph.AddBranchEdge("approval", map[string]any{
    "sendApproval":  func(decision string) bool { return decision == "approve" },
    "sendRejection": func(decision string) bool { return decision == "reject" },
})
graph.AddEdge("sendApproval", "onboardingComplete")
graph.AddEdge("sendRejection", "onboardingFailed")

// Run the graph
if err := graph.Run(); err != nil {
    fmt.Printf("Onboarding process failed: %v\n", err)
}
```

### Customer Onboarding Visualization

```mermaid
graph TD
    documentCheck --> approval
    approval --> |cond|sendApproval
    approval --> |cond|sendRejection
    sendApproval --> onboardingComplete
    collectInfo --> creditCheck
    collectInfo --> backgroundCheck
    collectInfo --> documentCheck
    creditCheck --> retryCreditCheck
    retryCreditCheck --> |cond|retryCreditCheck
    retryCreditCheck --> evaluateCredit
    evaluateCredit --> approval
    backgroundCheck --> approval
    sendRejection --> onboardingFailed
```

### 3. ETL (Extract, Transform, Load) Workflow

**Scenario**: Extracting data from multiple sources, validating, transforming, and loading it into appropriate storage based on data value

**Implementation**: 
- Use `Graph` with parallel data extraction from multiple sources
- Use `AddLoopEdge` for data validation retries
- Use `AddBranchEdge` for conditional loading based on data value
- Add error handling for data quality issues

**Example**: 
```go
graph := flow.NewGraph()

// Extract data from API
graph.AddNode("extractFromAPI", func() []map[string]interface{} {
    return []map[string]interface{}{
        {"id": 1, "name": "Product A", "price": 100},
        {"id": 2, "name": "Product B", "price": 200},
    }
})

// Extract data from database
graph.AddNode("extractFromDatabase", func() []map[string]interface{} {
    return []map[string]interface{}{
        {"id": 3, "name": "Product C", "price": 150},
        {"id": 4, "name": "Product D", "price": 250},
    }
})

// Combine extracted data
graph.AddNode("combineData", func(apiData, dbData []map[string]interface{}) []map[string]interface{} {
    return append(apiData, dbData...)
})

// Validate data
graph.AddNode("validateData", func(data []map[string]interface{}) (int, []map[string]interface{}) {
    invalidCount := 0
    var validData []map[string]interface{}
    for _, item := range data {
        price := item["price"].(int)
        if price > 0 {
            validData = append(validData, item)
        } else {
            invalidCount++
        }
    }
    fmt.Printf("Validated data: %d valid, %d invalid\n", len(validData), invalidCount)
    return invalidCount, validData
})

// Retry validation (loop node)
graph.AddNode("retryValidation", func(countInvalid int, data []map[string]interface{}) (int, []map[string]interface{}) {
    fmt.Println("Retrying validation...")
    return countInvalid - 1, data
})
graph.AddLoopEdge("retryValidation", func(countInvalid int, data []map[string]interface{}) bool {
    return countInvalid > 0
}, 2)

// Transform data
graph.AddNode("transformData", func(data []map[string]interface{}) []map[string]interface{} {
    var transformed []map[string]interface{}
    for _, item := range data {
        price := item["price"].(int)
        item["priceWithTax"] = float64(price) * 1.2
        item["category"] = "General"
        transformed = append(transformed, item)
    }
    return transformed
})

// Categorize data by value
graph.AddNode("categorizeData", func(data []map[string]interface{}) string {
    totalValue := 0
    for _, item := range data {
        totalValue += item["price"].(int)
    }
    if totalValue > 500 {
        return "high_value"
    }
    return "normal_value"
})

// Load to warehouse
graph.AddNode("loadToWarehouse", func(data []map[string]interface{}) error {
    fmt.Printf("Loading %d items to data warehouse\n", len(data))
    return nil
})

// Load to premium storage
graph.AddNode("loadToPremium", func(data []map[string]interface{}) error {
    fmt.Printf("Loading %d high-value items to premium storage\n", len(data))
    return nil
})

// Add edges
graph.AddEdge("extractFromAPI", "combineData")
graph.AddEdge("extractFromDatabase", "combineData")
graph.AddEdge("combineData", "validateData")
graph.AddEdge("validateData", "retryValidation")
graph.AddEdge("retryValidation", "transformData")
graph.AddEdge("transformData", "categorizeData")
graph.AddBranchEdge("categorizeData", map[string]any{
    "loadToWarehouse": func(category string) bool { return category == "normal_value" },
    "loadToPremium":   func(category string) bool { return category == "high_value" },
})

if err := graph.Run(); err != nil {
    fmt.Printf("ETL process failed: %v\n", err)
}
```

### ETL Workflow Visualization

```mermaid
graph TD
    validateData --> retryValidation
    extractFromAPI --> combineData
    retryValidation --> |cond|retryValidation
    retryValidation --> transformData
    combineData --> validateData
    transformData --> categorizeData
    categorizeData --> |cond|loadToWarehouse
    categorizeData --> |cond|loadToPremium
    extractFromDatabase --> combineData
```

### 4. Order Processing

**Scenario**: Processing customer orders with inventory check, payment processing, and shipping

**Implementation**: 
- Use `Graph` to model order processing workflow
- Use `AddLoopEdge` for inventory check retries
- Use `AddBranchEdge` for conditional payment/shipping paths
- Add compensation nodes for failure handling

**Example**: 
```go
graph := flow.NewGraph()

// Create order
graph.AddNode("createOrder", func() map[string]interface{} {
    return map[string]interface{}{
        "orderId":    "ORD-123",
        "customerId": "CUST-456",
        "items":      []string{"ITEM-1", "ITEM-2"},
        "total":      300,
    }
})

// Check inventory
graph.AddNode("checkInventory", func(order map[string]interface{}) (int, map[string]interface{}) {
    fmt.Println("Checking inventory...")
    return 0, order
})

// Retry inventory check (loop node)
graph.AddNode("retryInventory", func(retryCount int, order map[string]interface{}) (int, map[string]interface{}) {
    fmt.Printf("Retrying inventory check (attempt %d)...\n", retryCount+1)
    return retryCount + 1, order
})
graph.AddLoopEdge("retryInventory", func(retryCount int, order map[string]interface{}) bool {
    return retryCount < 2
}, 3)

// Evaluate inventory availability
graph.AddNode("evaluateInventory", func(retryCount int, order map[string]interface{}) bool {
    fmt.Println("Inventory available after retries")
    return true
})

// Process payment
graph.AddNode("processPayment", func(available bool) bool {
    fmt.Println("Processing payment...")
    return true
})

// Update inventory
graph.AddNode("updateInventory", func(available bool) bool {
    fmt.Println("Updating inventory...")
    return true
})

// Ship order
graph.AddNode("shipOrder", func(success bool) string {
    fmt.Println("Shipping order...")
    return "SHIP-789"
})

// Send notification
graph.AddNode("sendNotification", func(trackingId string) {
    fmt.Printf("Sending notification with tracking %s\n", trackingId)
})

// Compensation nodes for failures
graph.AddNode("cancelPayment", func(success bool) {
    fmt.Println("Cancelling payment for order")
})

graph.AddNode("restoreInventory", func(available bool) {
    fmt.Println("Restoring inventory for order")
})

// Add edges
graph.AddEdge("createOrder", "checkInventory")
graph.AddEdge("checkInventory", "retryInventory")
graph.AddEdge("retryInventory", "evaluateInventory")
graph.AddBranchEdge("evaluateInventory", map[string]any{
    "processPayment":   func(available bool) bool { return available },
    "restoreInventory": func(available bool) bool { return !available },
})
graph.AddEdge("evaluateInventory", "updateInventory")
graph.AddBranchEdge("processPayment", map[string]any{
    "shipOrder":     func(success bool) bool { return success },
    "cancelPayment": func(success bool) bool { return !success },
})
graph.AddEdge("shipOrder", "sendNotification")

if err := graph.Run(); err != nil {
    fmt.Printf("Order processing failed: %v\n", err)
}
```

### Order Processing Visualization

```mermaid
graph TD
    retryInventory --> |cond|retryInventory
    retryInventory --> evaluateInventory
    evaluateInventory --> |cond|processPayment
    evaluateInventory --> |cond|restoreInventory
    evaluateInventory --> updateInventory
    shipOrder --> sendNotification
    checkInventory --> retryInventory
    processPayment --> |cond|shipOrder
    processPayment --> |cond|cancelPayment
    createOrder --> checkInventory
```

## Benchmarks

Benchmark results (Apple M1 Pro):

| Benchmark | Iterations | Time (ns/op) | Memory (B/op) | Allocations (allocs/op) |
|-----------|------------|--------------|---------------|------------------------|
| BenchmarkC32-8 | 50720 | 23908 | 4789 | 69 |
| BenchmarkS32-8 | 172738 | 6797 | 3499 | 35 |
| BenchmarkC6-8 | 157916 | 7419 | 1342 | 22 |
| BenchmarkC8x8-8 | 12381 | 96892 | 10616 | 189 |

**Benchmark Descriptions:**

- **C32**: 32 concurrent nodes with no dependencies (full parallelism)
- **S32**: 32 nodes in a sequential chain
- **C6**: 6 nodes with diamond-shaped dependencies
- **C8x8**: 8 layers × 8 nodes with full connectivity between layers

## Examples

The library includes several examples in the `_examples` directory:

- **Basic Examples**:
  - [`basic-chain`](https://github.com/zkep/flow/tree/master/_examples/basic-chain): Basic chain workflow
  - [`basic-graph`](https://github.com/zkep/flow/tree/master/_examples/basic-graph): Basic graph workflow

- **Advanced Examples**:
  - [`advanced-chain`](https://github.com/zkep/flow/tree/master/_examples/advanced-chain): Advanced chain with complex parameter passing
  - [`advanced-graph`](https://github.com/zkep/flow/tree/master/_examples/advanced-graph): Advanced graph with multiple node types


## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

Flow is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.
