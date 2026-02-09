# Flow - 工作流编排库

Flow 是一个强大的 Go 语言工作流构建和执行库，提供两种执行模式：线性执行链（Chain）和图形化执行器（Graph）。

## 目录

- [特性](#特性)
- [安装](#安装)
- [快速开始](#快速开始)
- [API 示例](#api-示例)
  - [Chain API](#chain-api)
  - [Graph API](#graph-api)
  - [边类型](#边类型)
  - [执行策略](#执行策略)
  - [检查点](#检查点)
  - [暂停与恢复](#暂停与恢复)
  - [图形可视化](#图形可视化)
- [高级特性](#高级特性)
- [实际应用场景](#实际应用场景)
- [错误处理](#错误处理)
- [配置选项](#配置选项)
- [性能优化](#性能优化)

## 特性

### Chain 模式

- **线性执行**：按顺序执行任务，步骤依次运行
- **自动参数传递**：上一步输出自动作为下一步输入
- **错误处理**：全面的错误传播和处理机制
- **简单易用**：适合简单的顺序处理场景

### Graph 模式

- **图形工作流**：使用节点（Node）和边（Edge）构建复杂工作流
- **多种边类型**：支持普通边、循环边和分支边
- **条件执行**：为边添加条件以控制工作流路径
- **并行执行**：并发执行独立节点以提高性能
- **自动参数处理**：任务间智能参数传递和类型转换
- **错误处理**：全面的错误传播和处理机制
- **检查点支持**：保存和恢复工作流状态以实现容错
- **暂停/恢复**：暂停工作流执行并在稍后恢复
- **可视化支持**：生成 Mermaid 和 Graphviz 图表用于工作流可视化

## 安装

```bash
go get github.com/zkep/flow
```

## 快速开始

### 基础 Chain 示例

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
        fmt.Printf("错误: %v\n", err)
        return
    }

    result, err := chain.Value("step3")
    if err != nil {
        fmt.Printf("错误: %v\n", err)
        return
    }

    fmt.Printf("最终结果: %v\n", result) // 输出: 25
}
```

### 基础 Graph 示例

```go
package main

import (
    "fmt"
    "github.com/zkep/flow"
)

func main() {
    g := flow.NewGraph()

    g.AddNode("start", func() int {
        fmt.Println("执行开始节点")
        return 10
    })

    g.AddNode("process1", func(x int) int {
        fmt.Printf("执行 process1: %d * 2 = %d\n", x, x*2)
        return x * 2
    })

    g.AddNode("process2", func(x int) int {
        fmt.Printf("执行 process2: %d + 5 = %d\n", x, x+5)
        return x + 5
    })

    g.AddNode("end1", func(x int) {
        fmt.Printf("执行 end1 节点: 最终结果是 %d\n", x)
    })

    g.AddEdge("start", "process1")
    g.AddEdge("process1", "process2")
    g.AddEdge("process2", "end1")

    err := g.Run()
    if err != nil {
        fmt.Printf("错误: %v\n", err)
    } else {
        fmt.Println("执行成功完成")
    }
}
```

## API 示例

### Chain API

Chain 模式允许创建线性工作流，每个步骤按顺序执行，上一步的输出自动作为下一步的输入。

#### 创建 Chain

```go
chain := flow.NewChain()
```

#### 添加步骤

```go
chain.Add("stepName", func() int {
    return 42
})
```

#### 运行 Chain

```go
err := chain.Run()
if err != nil {
    // 处理错误
}

// 或使用 context
ctx := context.Background()
err := chain.RunWithContext(ctx)
```

#### 获取结果

```go
// 获取单个步骤的值
result, err := chain.Value("stepName")

// 获取步骤的所有返回值
results, err := chain.Values("stepName")
```

#### 使用 `Use` 复用步骤

`Use` 方法可以从现有链中选择特定步骤创建新链。

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
    fmt.Printf("保存数据: %v\n", data)
    return nil
})

originalChain.Run()

// 仅使用特定步骤创建新链
subsetChain := originalChain.Use("loadData", "processData")
subsetChain.Run()
```

#### Chain 错误处理

```go
chain := flow.NewChain()

chain.Add("step1", func() int {
    return 42
})

chain.Add("step2", func(x int) (int, error) {
    if x < 0 {
        return 0, fmt.Errorf("无效值")
    }
    return x * 2, nil
})

err := chain.Run()
if err != nil {
    fmt.Printf("Chain 错误: %v\n", err)
}
```

### Graph API

Graph 模式允许创建带有节点和边的复杂工作流，支持不同的边类型和执行策略。

#### 创建 Graph

```go
graph := flow.NewGraph()

// 带选项
graph := flow.NewGraph(
    flow.WithCapacity(64),
    flow.WithLargeGraphThreshold(256),
)
```

#### 添加节点

```go
// 简单节点
graph.AddNode("process", func(x int) int {
    return x * 2
})

// 多输入节点
graph.AddNode("combine", func(a, b int) int {
    return a + b
})

// 带错误返回的节点
graph.AddNode("validate", func(x int) (int, error) {
    if x < 0 {
        return 0, fmt.Errorf("无效值")
    }
    return x, nil
})

// 无返回值节点（副作用）
graph.AddNode("log", func(x int) {
    fmt.Printf("值: %d\n", x)
})
```

#### 添加边

```go
// 简单边
graph.AddEdge("fromNode", "toNode")

// 带条件的边
graph.AddEdgeWithCondition("fromNode", "toNode", func(x int) bool {
    return x > 0
})

// 循环边（用于重试/循环场景）
graph.AddLoopEdge("retryNode", func(result int) bool {
    return result < 100
}, 3) // 最大 3 次迭代

// 分支边（多条条件路径）
graph.AddBranchEdge("decisionNode", map[string]any{
    "pathA": func(result int) bool { return result > 50 },
    "pathB": func(result int) bool { return result <= 50 },
})
```

#### 运行 Graph

```go
// 默认并行执行
err := graph.Run()

// 带 context 运行
ctx := context.Background()
err := graph.RunWithContext(ctx)

// 顺序运行
err := graph.RunSequential()
err := graph.RunSequentialWithContext(ctx)
```

#### 获取节点信息

```go
// 获取节点状态
status, err := graph.NodeStatus("nodeName")

// 获取节点结果
result, err := graph.NodeResult("nodeName")

// 获取节点错误
err := graph.NodeError("nodeName")

// 按状态获取节点列表
pendingNodes := graph.GetNodesByStatus(flow.NodeStatusPending)
completedNodes := graph.GetNodesByStatus(flow.NodeStatusCompleted)
```

#### 清除 Graph 状态

```go
graph.ClearStatus()
```

### 边类型

| 边类型 | 描述 | 示例 |
|--------|------|------|
| Normal | 连接两个节点的标准边 | `AddEdge("a", "b")` |
| Loop | 用于循环/重试操作的边（源和目标节点相同） | `AddLoopEdge("a", cond, maxIter)` |
| Branch | 带条件分支到多个目标节点的边 | `AddBranchEdge("a", branches)` |

### 执行策略

#### 并行执行（默认）

独立节点并发执行以提高性能。图执行器在可能时自动处理并行执行。

```go
// 默认并行执行
err := graph.Run()
```

#### 顺序执行

节点按拓扑顺序一个接一个执行。

```go
err := graph.RunSequential()
```

### 检查点

Flow 提供检查点功能来保存和恢复工作流状态。

#### 创建检查点存储

```go
// 基于文件的检查点存储
store, err := flow.NewFileCheckpointStore("/path/to/checkpoints")

// 内存中的检查点存储
store := flow.NewMemoryCheckpointStore()
```

#### 保存和加载检查点

```go
// 保存检查点
checkpoint, err := graph.SaveCheckpoint()
err = store.Save("my-flow", checkpoint)

// 加载检查点
checkpoint, err = store.Load("my-flow")
err = graph.LoadCheckpoint(checkpoint)

// 列出所有检查点
keys, err := store.List()

// 删除检查点
err = store.Delete("my-flow")
```

#### 检查点接口

```go
type FlowCheckpointable interface {
    SaveCheckpoint() (*Checkpoint, error)
    LoadCheckpoint(checkpoint *Checkpoint) error
    SaveToStore(store CheckpointStore, key string) error
    LoadFromStore(store CheckpointStore, key string) error
    Reset()
}
```

### 暂停与恢复

Flow 支持暂停和恢复工作流执行。

#### 基础暂停/恢复

```go
// 暂停执行
err := graph.Pause()

// 恢复执行
ctx := context.Background()
err := graph.Resume(ctx)
```

#### 暂停配置

```go
config := flow.NewPauseConfig()

// 在特定节点暂停
config.SetPauseAtNodes("node1", "node2")

// 出错时暂停
config.SetPauseOnError()

// 应用配置
graph.SetPauseConfig(config)
err := graph.PauseWithConfig(config)
```

#### 恢复配置

```go
config := flow.NewResumeConfig()

// 跳过已完成的节点
config.SkipCompleted = true

// 重试失败的节点
config.SetRetryFailed()

// 带配置恢复
err := graph.ResumeWithConfig(ctx, config)
```

#### 暂停信号

```go
signal := flow.NewSimplePauseSignal()
graph.SetPauseSignal(signal)

// 触发暂停
signal.SetPaused(true)

// 重置信号
signal.Reset()
```

#### 资源检查

```go
checker := flow.NewSimpleResourceChecker(100, 10)
graph.SetResourceChecker(checker)

// 检查可用性
available := checker.CheckAvailable("nodeName")

// 消耗/释放资源
checker.Consume()
checker.Release()
```

#### 获取流程状态

```go
state := graph.State()

// 状态: FlowStateIdle, FlowStateRunning, FlowStatePaused, FlowStateCompleted, FlowStateFailed
if state == flow.FlowStatePaused {
    pausedAt := graph.GetPausedAtNode()
    fmt.Printf("暂停在节点: %s\n", pausedAt)
}
```

### 图形可视化

Flow 支持生成图表用于可视化。

#### Mermaid 图表

```go
mermaid := graph.Mermaid()
fmt.Println(mermaid)
```

输出:
```
graph TD
    start --> process1
    process1 --> process2
    process2 --> end
```

#### Graphviz 图表

```go
graphviz := graph.String()
fmt.Println(graphviz)
```

输出:
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

## 高级特性

### 条件执行

使用 `AddEdgeWithCondition` 为边添加条件，根据运行时值动态确定工作流路径。

```go
graph.AddNode("check", func(x int) int {
    return x
})

graph.AddNode("high", func(x int) string {
    return "高值"
})

graph.AddNode("low", func(x int) string {
    return "低值"
})

graph.AddEdgeWithCondition("check", "high", func(x int) bool {
    return x > 100
})

graph.AddEdgeWithCondition("check", "low", func(x int) bool {
    return x <= 100
})
```

### 循环执行

使用 `AddLoopEdge` 创建带有自动重试支持的循环场景。

```go
graph.AddNode("retry", func(attempt int) (int, error) {
    fmt.Printf("尝试 %d\n", attempt)
    if attempt < 3 {
        return attempt + 1, fmt.Errorf("失败")
    }
    return attempt, nil
})

graph.AddLoopEdge("retry", func(result int, err error) bool {
    return err != nil
}, 5) // 最大 5 次迭代
```

### 分支执行

使用 `AddBranchEdge` 创建到多个目标节点的条件分支。

```go
graph.AddNode("decision", func(x int) string {
    if x > 50 {
        return "approve"
    }
    return "reject"
})

graph.AddNode("approve", func(decision string) {
    fmt.Println("已批准")
})

graph.AddNode("reject", func(decision string) {
    fmt.Println("已拒绝")
})

graph.AddBranchEdge("decision", map[string]any{
    "approve": func(decision string) bool { return decision == "approve" },
    "reject":  func(decision string) bool { return decision == "reject" },
})
```

### 并行执行

图执行器在可能时自动处理独立节点的并行执行。

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

// parallel1 和 parallel2 并行执行
graph.Run()
```

### 类型转换

Flow 在可能时自动处理节点间的类型转换。

```go
graph.AddNode("int_to_string", func(x int) string {
    return fmt.Sprintf("数字: %d", x)
})

graph.AddNode("string_to_int", func(s string) int {
    return len(s)
})

graph.AddEdge("int_to_string", "string_to_int")
```

### 大图优化

对于有很多节点的图，Flow 提供优化执行。

```go
graph := flow.NewGraph(
    flow.WithLargeGraphThreshold(256),
)

// 大图使用基于层的并行执行策略
graph.Run()
```

## 实际应用场景

### 数据处理管道

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
        fmt.Printf("保存: %v\n", item)
    }
    return nil
})

if err := chain.Run(); err != nil {
    fmt.Printf("管道失败: %v\n", err)
}
```

### 业务流程自动化

请参考 [approval-flow](_examples/approval-flow/main.go) 示例了解完整的客户入职工作流。

### ETL（提取、转换、加载）工作流

请参考 [advanced-graph](_examples/advanced-graph/main.go) 示例了解完整的 ETL 流程。

### 订单处理

请参考 [advanced-graph](_examples/advanced-graph/main.go) 示例了解完整的订单处理工作流。

## 错误处理

Flow 提供全面的错误处理。

### 错误类型

| 错误常量 | 描述 |
|----------|------|
| `ErrArgTypeMismatch` | 参数类型与预期类型不匹配 |
| `ErrArgCountMismatch` | 参数数量与预期数量不匹配 |
| `ErrNotFunction` | 提供的值不是函数 |
| `ErrFunctionPanicked` | 函数执行导致 panic |
| `ErrStepNotFound` | 未找到指定名称的步骤 |
| `ErrNodeNotFound` | 未找到指定名称的节点 |
| `ErrDuplicateNode` | 同名节点已存在 |
| `ErrSelfDependency` | 节点不能依赖自身 |
| `ErrCyclicDependency` | 图中检测到循环依赖 |
| `ErrNoStartNode` | 图中未找到起始节点 |
| `ErrExecutionFailed` | 执行失败 |
| `ErrFlowPaused` | 流程已暂停 |
| `ErrResourceNotAvailable` | 资源不可用 |
| `ErrCheckpointNotFound` | 未找到检查点 |
| `ErrInvalidCheckpoint` | 检查点数据无效 |

### 错误传播

错误自动通过工作流传播：

```go
graph.AddNode("step1", func() (int, error) {
    return 0, fmt.Errorf("step1 失败")
})

graph.AddNode("step2", func(x int) int {
    return x * 2 // 永远不会执行
})

graph.AddEdge("step1", "step2")

err := graph.Run()
if err != nil {
    // err 包含 "step1 失败"
}
```

## 配置选项

### Graph 选项

| 选项 | 描述 | 默认值 |
|------|------|--------|
| `WithCapacity(capacity)` | 设置内部映射的初始容量 | 32 |
| `WithLargeGraphThreshold(threshold)` | 大图优化的阈值 | 128 |

### 节点状态

| 状态 | 值 | 描述 |
|------|-----|------|
| `NodeStatusPending` | 0 | 节点等待执行 |
| `NodeStatusRunning` | 1 | 节点正在执行 |
| `NodeStatusCompleted` | 2 | 节点成功完成 |
| `NodeStatusFailed` | 3 | 节点执行失败 |

### 流程状态

| 状态 | 值 | 描述 |
|------|-----|------|
| `FlowStateIdle` | 0 | 流程未启动 |
| `FlowStateRunning` | 1 | 流程正在运行 |
| `FlowStatePaused` | 2 | 流程已暂停 |
| `FlowStateCompleted` | 3 | 流程已完成 |
| `FlowStateFailed` | 4 | 流程已失败 |

### 暂停模式

| 模式 | 描述 |
|------|------|
| `PauseModeImmediate` | 立即暂停 |
| `PauseModeAtNode` | 在特定节点暂停 |
| `PauseModeOnError` | 出错时暂停 |
