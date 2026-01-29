# Flow - 工作流编排库

🌍 **语言切换**：[English](README.md)

Flow 是一个用于构建和执行工作流的 Go 库，提供两种执行模式：线性执行链（Chain）和图形化执行器（Graph）。

## 功能概述

### Chain - 线性执行链
提供简单的顺序执行模式，适合管道式数据处理：
- 链式函数调用
- 值传递历史记录
- 延迟任务执行
- 步骤命名和历史回溯

### Graph - 图形化执行器
提供复杂的工作流编排能力，支持有向无环图(DAG)：
- 多种节点类型（Start、End、Branch、Parallel、Loop）
- 条件执行路径
- 顺序和并行执行策略
- 循环依赖检测
- 可视化输出（Graphviz 和 Mermaid）

## 安装

```bash
go get -u github.com/zkep/flow
```

## 使用示例

### Chain 示例

```go
package main

import (
    "fmt"
    "github.com/zkep/flow"
)

func main() {
    result := flow.NewChain(10)
        .Call(func(x int) int { return x * 2 })
        .Call(func(x int) int { return x + 5 })
        .Call(func(x int) string { return fmt.Sprintf("Result: %d", x) })
        .Value()
    
    fmt.Println(result) // 输出: Result: 25
}
```

### Chain 多值示例

```go
package main

import (
    "fmt"
    "github.com/zkep/flow"
)

func main() {
    // 多个输入和多个输出
    c := flow.NewChain(10, 20).
        Call(func(a, b int) (int, int) {
            return a + b, a * b
        })

    // 获取所有当前值
    values := c.Values()
    fmt.Printf("所有值：%v\n", values) // 输出: 所有值：[30 200]
    
    // 获取第一个值（与 Value() 相同）
    firstValue := c.Value()
    fmt.Printf("第一个值：%v\n", firstValue) // 输出: 第一个值：30
    
    // 继续使用所有值
    c = c.Call(func(a, b int) string {
        return fmt.Sprintf("和：%d, 积：%d", a, b)
    })
    
    fmt.Printf("最终结果：%v\n", c.Value()) // 输出: 最终结果：和：30, 积：200
}
```

### Chain 延迟和运行示例

```go
package main

import (
    "fmt"
    "github.com/zkep/flow"
)

func main() {
    var sum int
    var product int
    
    result := flow.NewChain(1, 2, 3).
        Defer(func(a, b, c int) {
            sum = a + b + c
        }).
        Defer(func(a, b, c int) {
            product = a * b * c
        }).
        Call(func(a, b, c int) int {
            return (a + b + c) / 3 // 计算平均值
        })
    
    // 执行所有延迟任务
    err := result.Run()
    if err != nil {
        panic(err)
    }
    
    fmt.Printf("和：%d, 积：%d, 平均值：%d\n", sum, product, result.Value())
    // 输出: 和：6, 积：6, 平均值：2
}
```

### 复杂 Chain 示例（包含 Defer、Call 和 Use）

```go
package main

import (
    "fmt"
    "strings"
    "github.com/zkep/flow"
)

func main() {
    var intermediateResults []int
    var finalReport string
    
    // 处理流程：数据 -> 验证 -> 转换 -> 分析 -> 报告
    result := flow.NewChain(10, 20, 30, 40, 50).
        Name("raw_data").
        Defer(func(data ...int) {
            // 延迟任务 1：捕获初始数据用于审计
            fmt.Printf("审计：收到初始数据 %d 项\n", len(data))
        }).
        Call(func(data ...int) []int {
            // 步骤 1：验证数据
            var valid []int
            for _, v := range data {
                if v > 0 {
                    valid = append(valid, v)
                }
            }
            return valid
        }).
        Name("validated").
        Defer(func(valid []int) {
            intermediateResults = append(intermediateResults, len(valid))
        }).
        Call(func(data []int) []int {
            var normalized []int
            for _, v := range data {
                normalized = append(normalized, v/10)
            }
            return normalized
        }).
        Name("transformed").
        Defer(func(transformed []int) {
            var sum int
            for _, v := range transformed {
                sum += v
            }
            intermediateResults = append(intermediateResults, sum)
        }).
        Call(func(data []int) (int, int, float64) {
            if len(data) == 0 {
                return 0, 0, 0
            }
            
            sum := 0
            min := data[0]
            max := data[0]
            
            for _, v := range data {
                sum += v
                if v < min {
                    min = v
                }
                if v > max {
                    max = v
                }
            }
            
            average := float64(sum) / float64(len(data))
            return min, max, average
        }).
        Name("analyzed").
        Defer(func(min, max int, avg float64) {
            finalReport = fmt.Sprintf("分析报告 - 最小值：%d, 最大值：%d, 平均值：%.2f", min, max, avg)
        }).
        Use("raw_data", "validated").
        Call(func(rawData []int, validatedData []int) float64 {
            // 计算验证后的保留率
            return float64(len(validatedData)) / float64(len(rawData)) * 100
        })

    // 执行所有延迟任务
    err := result.Run()
    if err != nil {
        panic(err)
    }
    
    // 生成最终输出
    fmt.Println("=" + strings.Repeat("-", 50) + "=")
    fmt.Println(finalReport)
    fmt.Printf("验证保留率：%.2f%%\n", result.Value())
    fmt.Printf("中间结果（有效计数, 转换和）：%v\n", intermediateResults)
    fmt.Println("=" + strings.Repeat("-", 50) + "=")
    
    // 输出：
    // 审计：收到初始数据 5 项
    // =--------------------------------------------------=
    // 分析报告 - 最小值：1, 最大值：5, 平均值：3.00
    // 验证保留率：100.00%
    // 中间结果（有效计数, 转换和）：[5 15]
    // =--------------------------------------------------=
}
```

### Chain Use 和 Name 示例

```go
package main

import (
    "fmt"
    "github.com/zkep/flow"
)

func main() {
    result := flow.NewChain(10).
        Name("initial_value").
        Call(func(x int) int { return x * 2 }).
        Name("doubled").
        Call(func(x int) int { return x + 5 }).
        Name("added").
        Use("initial_value", 1). // 使用初始值 (10) 和翻倍后的值 (20)
        Call(func(a, b int) int { return a + b })

    fmt.Printf("结果：%d\n", result.Value()) // 输出: 结果：30
}
```

### Graph 示例

```go
package main

import (
    "fmt"
    "github.com/zkep/flow"
)

func main() {
    g := flow.NewGraph()
    
    // 添加节点
    g.StartNode("start", func() int { return 10 })
	g.AddNode("double", func(x int) int { return x * 2 }, flow.NodeTypeNormal)
	g.AddNode("add5", func(x int) int { return x + 5 }, flow.NodeTypeNormal)     
    g.EndNode("end", func(x int) { fmt.Println("Result:", x) })
    
    // 添加边
    g.AddEdge("start", "double")
    g.AddEdge("double", "add5")
    g.AddEdge("add5", "end")
    
    // 执行
    err := g.Run()
    if err != nil {
        panic(err)
    }
}
```

### 条件执行示例

```go
g := flow.NewGraph()

g.StartNode("input", func() int { return 42 })
g.AddNode("processA", func(x int) int { return x * 2 })
g.AddNode("processB", func(x int) int { return x + 10 })
g.EndNode("output", func(x int) { fmt.Println(x) })

// 条件边：当输入 > 40 时执行 processA，否则执行 processB
g.AddEdgeWithCondition("input", "processA", func(x int) bool { return x > 40 })
g.AddEdgeWithCondition("input", "processB", func(x int) bool { return x <= 40 })

g.AddEdge("processA", "output")
g.AddEdge("processB", "output")

g.Run()
```

## API 文档

### Chain 类型

```go
// 创建新的执行链
func NewChain(initial ...any) *Chain

// 调用函数并传递当前值
func (c *Chain) Call(fn any) *Chain

// 延迟执行任务
func (c *Chain) Defer(fn any) *Chain

// 执行所有延迟任务
func (c *Chain) Run() error

// 获取当前值列表
func (c *Chain) Values() []any

// 获取第一个值
func (c *Chain) Value() any

// 获取错误
func (c *Chain) Error() error

// 获取执行历史
func (c *Chain) History() [][]any

// 为当前步骤命名
func (c *Chain) Name(name string) *Chain

// 使用历史步骤的值
func (c *Chain) Use(steps ...any) *Chain
```

### Graph 类型

```go
// 创建新的图执行器
func NewGraph() *Graph

// 添加节点
func (g *Graph) AddNode(name string, fn any, nodeType NodeType) *Graph
func (g *Graph) StartNode(name string, fn any) *Graph
func (g *Graph) EndNode(name string, fn any) *Graph
func (g *Graph) BranchNode(name string, fn any) *Graph
func (g *Graph) ParallelNode(name string, fn any) *Graph
func (g *Graph) LoopNode(name string, fn any) *Graph

// 添加边
func (g *Graph) AddEdge(from, to string) *Graph
func (g *Graph) AddEdgeWithCondition(from, to string, cond any) *Graph

// 执行策略
func (g *Graph) Run() error
func (g *Graph) RunSequential() error
func (g *Graph) RunParallel() error

// 节点状态
func (g *Graph) NodeStatus(name string) NodeStatus
func (g *Graph) NodeResult(name string) []any
func (g *Graph) NodeError(name string) error

// 可视化
func (g *Graph) String() string      // Graphviz 格式
func (g *Graph) Mermaid() string     // Mermaid 格式
```

## 节点类型

```go
type NodeType int

const (
    NodeTypeNormal   NodeType = iota  // 普通节点
    NodeTypeStart                     // 起始节点
    NodeTypeEnd                       // 结束节点
    NodeTypeBranch                    // 分支节点
    NodeTypeParallel                  // 并行节点
    NodeTypeLoop                      // 循环节点
)
```

## 执行状态

```go
type NodeStatus int

const (
    NodeStatusPending   NodeStatus = iota  // 待执行
    NodeStatusRunning                     // 执行中
    NodeStatusCompleted                   // 已完成
    NodeStatusFailed                      // 执行失败
)
```

## 许可证

MIT License
