# flow 性能优化深度解析

### 1. 全面的对象池复用机制

这是 flow 最核心的优化策略。几乎所有高频创建的对象都使用了 `sync.Pool` 进行复用。

#### 1.1 对象池架构

```
┌─────────────────────────────────────────────────────────────┐
│                    flow 对象池体系                           │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐  │
│  │  nodePool   │  │  edgePool   │  │  nodeStatePool      │  │
│  │  Node 复用  │  │  Edge 复用  │  │  执行状态复用        │  │
│  └─────────────┘  └─────────────┘  └─────────────────────┘  │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐  │
│  │  taskPool   │  │ localWorker │  │  condCompilerPool   │  │
│  │  任务对象   │  │  Pool 复用  │  │  条件编译器复用      │  │
│  └─────────────┘  └─────────────┘  └─────────────────────┘  │
│  ┌─────────────────────────────────────────────────────────┐│
│  │              Slice Pool 切片池                          ││
│  │  anySlicePool | stringSlicePool | reflectValueSlicePool ││
│  └─────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────┘
```

#### 1.2 泛型对象池实现

```go
// pool.go
type ObjectPool[T any] struct {
    pool  sync.Pool
    reset func(T)
}

func NewObjectPool[T any](creator func() T, opts ...PoolOption[T]) *ObjectPool[T] {
    p := &ObjectPool[T]{
        pool: sync.Pool{
            New: func() any { return creator() },
        },
    }
    for _, opt := range opts {
        opt(p)
    }
    return p
}

func (p *ObjectPool[T]) Get() T {
    return p.pool.Get().(T)
}

func (p *ObjectPool[T]) Put(x T) {
    if p.reset != nil {
        p.reset(x)  // 重置对象状态
    }
    p.pool.Put(x)
}
```

#### 1.3 带 Reset 函数的对象池

```go
// Node 对象池示例
nodePool = NewObjectPool(
    func() *Node { return &Node{} },
    WithReset(func(n *Node) {
        n.name = ""
        n.status = NodeStatusPending
        n.fn = nil
        n.fnValue = reflect.Value{}
        n.fnType = nil
        n.argTypes = nil
        n.numOut = 0
        n.hasErrorReturn = false
        n.description = ""
        n.inputs = nil
        n.outputs = nil
        n.err = nil
        n.result = nil
        n.callFn = nil
        n.argCount = 0
        n.sliceArg = false
        n.sliceElemType = nil
    }),
)
```

**优化效果**：内存分配从 ~226 allocs/op 降至 ~37 allocs/op，减少 **83%**

---

### 2. Map 复用与 clear 优化

flow 大量复用已分配的 Map，避免每次执行都重新创建：

```go
// graph.go - 执行入边缓存
if g.execInEdges != nil && g.execPlanValid {
    incomingEdges = g.execInEdges
} else {
    if g.execInEdges == nil {
        g.execInEdges = make(map[string][]*Edge, len(allEdges))
    } else {
        clear(g.execInEdges)  // Go 1.21+ clear 复用内存
    }
}

// 执行状态复用
if g.execStates == nil {
    g.execStates = make(map[string]*nodeState, len(plan))
} else {
    clear(g.execStates)
}
```

**关键点**：使用 `clear()` 而非 `make()` 可以复用底层数组内存。

---

### 3. 执行计划缓存

flow 缓存拓扑排序结果，避免重复计算：

```go
func (g *Graph) buildExecutionPlan() ([]string, error) {
    // 快速路径：返回缓存结果
    if g.execPlanValid && len(g.execPlan) > 0 {
        return g.execPlan, nil
    }
    
    // 构建新计划
    // ... 拓扑排序逻辑 ...
    
    // 缓存结果
    g.execPlan = append(g.execPlan[:0], plan...)
    g.execPlanValid = true
    
    return g.execPlan, nil
}
```

**优化效果**：重复执行同一图时，跳过拓扑排序开销。

---

### 4. 自适应执行策略

根据图的规模选择不同的执行模式：

```go
const largeGraphThreshold = 128

func (g *Graph) executeGraphParallelWithContext(ctx context.Context) error {
    nodeCount := len(g.nodes)
    
    threshold := largeGraphThreshold
    if g.largeThreshold > 0 {
        threshold = g.largeThreshold
    }
    
    if nodeCount >= threshold {
        return g.executeGraphParallelLarge(ctx)  // 分层执行
    }
    return g.executeGraphParallelSmall(ctx)      // 全局工作池
}
```

#### 4.1 小图执行 (executeGraphParallelSmall)

- 使用全局共享的 `globalWorker`
- 避免创建/销毁 goroutine 开销
- 所有任务共享一个工作池

```go
var gw *globalWorker
var gwOnce sync.Once

func getGlobalWorker() *globalWorker {
    gwOnce.Do(func() {
        gw = &globalWorker{
            taskChan: make(chan *nodeTask, defaultTaskChannelSize),
        }
        for i := 0; i < defaultWorkerCount; i++ {
            gw.wg.Add(1)
            go gw.worker()
        }
    })
    return gw
}
```

#### 4.2 大图执行 (executeGraphParallelLarge)

- 按拓扑层执行
- 每层内并行，层间同步
- 减少不必要的等待和竞争

```go
for _, layer := range layers {
    // 提交当前层所有任务
    for _, nodeName := range layer {
        task := taskPool.Get().(*nodeTask)
        task.ctx = execCtx
        task.name = nodeName
        pool.Submit(task)
    }
    
    // 等待当前层完成
    layerTotal := len(layer)
    layerCompleted := 0
    for layerCompleted < layerTotal {
        select {
        case <-ctx.Done():
            return
        case err := <-errChan:
            return err
        case <-layerDone:
            layerCompleted++
        }
    }
}
```

---

### 5. 高效的等待机制

使用原子操作 + channel 的混合等待策略：

```go
func waitForDone(state *nodeState, ctx context.Context) bool {
    // 快速路径：原子检查
    if atomic.LoadUint32(&state.done) != 0 {
        return true
    }
    
    // 慢速路径：channel 等待
    select {
    case <-state.doneSig:
        return true
    case <-ctx.Done():
        return false
    }
}
```

**设计意图**：已完成的任务通过原子操作快速返回，避免 channel 开销。

---

### 6. 反射调用预编译

将运行时反射分析移至初始化阶段：

```go
func (g *Graph) compileNodeCall(node *Node) func([]any) ([]any, error) {
    // 提取所有反射信息（仅在添加节点时执行一次）
    fnValue := node.fnValue
    argCount := node.argCount
    sliceArg := node.sliceArg
    sliceElemType := node.sliceElemType
    hasError := node.hasErrorReturn
    argTypes := node.argTypes
    
    // 返回预编译的闭包
    return func(inputs []any) ([]any, error) {
        // 运行时只执行调用，不再做反射分析
        args := reflectValueSlicePool.Get(argCount)
        defer reflectValueSlicePool.Put(args)
        
        // ... 参数处理 ...
        
        results := fnValue.Call(args)
        
        // ... 结果处理 ...
        return out, nil
    }
}
```

**优化效果**：每次节点执行只做 `reflect.Value.Call()`，避免重复的类型分析。

---

### 7. 串行执行的极致优化

串行执行完全避免 goroutine 开销：

```go
func (g *Graph) executeSequential(ctx context.Context, plan []string) error {
    resultsMap := make(map[string][]any, len(plan))
    
    for _, name := range plan {
        // 直接在当前 goroutine 执行
        results, err := g.executeNodeWithLoop(name, inputs)
        if err != nil {
            return err
        }
        resultsMap[name] = results
    }
    
    return nil
}
```


---

## 技术总结

### 优化策略汇总

| 优化策略 | 技术实现 | 性能影响 |
|----------|----------|----------|
| **对象池复用** | `sync.Pool` + 泛型封装 | 内存分配减少 75-85% |
| **Map 复用** | `clear()` 复用底层内存 | 减少 GC 压力 |
| **执行计划缓存** | 缓存拓扑排序结果 | 重复执行零开销 |
| **自适应执行** | 小图共享池 / 大图分层 | 减少竞争和等待 |
| **混合等待机制** | atomic + channel | 快速路径优化 |
| **反射预编译** | 初始化时分析，运行时执行 | 运行时零反射分析 |
| **串行极致优化** | 直接同步调用 | 无并发抽象开销 |

### 设计哲学

flow 的核心设计理念是：

> **在保证正确性和功能完整性的前提下，最小化运行时内存分配和反射开销。**

这一理念贯穿于：
1. 数据结构的复用设计
2. 执行路径的分层优化
3. 反射分析的前置处理

### 适用场景

基于以上分析，flow 特别适合：

1. **高频任务编排**：对象池复用显著降低 GC 压力
2. **串行流水线**：极致的串行执行优化
3. **大规模 DAG 执行**：分层执行策略减少等待
4. **性能敏感场景**：每一点优化都能带来显著收益

---
