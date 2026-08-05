

# Flow - Biblioteca de Orquestación de Flujos de Trabajo

🌍 **Cambio de Idioma**: [中文文档](README-zh.md)

Flow es una potente biblioteca en Go para construir y ejecutar flujos de trabajo, que ofrece dos modos de ejecución: cadena de ejecución lineal (Chain) y ejecutor gráfico (Graph).

## Características

### Modo Cadena

- **Ejecución Lineal**: Ejecuta las tareas secuencialmente, paso a paso
- **Paso Automático de Parámetros**: La salida de un paso se pasa automáticamente como entrada al siguiente
- **Manejo de Errores**: Propagación y manejo integral de errores
- **Simple y Fácil**: Ideal para escenarios de procesamiento secuencial simple

### Modo Grafo

- **Flujos de Trabajo Gráficos**: Construye flujos de trabajo complejos con nodos y aristas
- **Múltiples Tipos de Aristas**: Soporte para aristas normales, de bucle y de ramificación
- **Ejecución Condicional**: Agrega condiciones a las aristas para controlar las rutas del flujo de trabajo
- **Ejecución Paralela**: Ejecuta nodos independientes de forma concurrente para mejorar el rendimiento
- **Manejo Automático de Parámetros**: Paso inteligente de parámetros y conversión de tipos entre tareas
- **Manejo de Errores**: Propagación y manejo integral de errores
- **Soporte de Puntos de Control**: Guarda y restaura el estado del flujo de trabajo para tolerancia a fallos
- **Pausar/Reanudar**: Pausa la ejecución del flujo de trabajo y reanúdala más tarde
- **Soporte de Visualización**: Genera diagramas Mermaid y Graphviz para la visualización de flujos de trabajo

## Conceptos Clave

### Nodo

Un **Nodo** representa una sola tarea u operación en el flujo de trabajo. Cada nodo contiene:

- **name**: Un identificador único para el nodo
- **fn**: La función a ejecutar
- **status**: Estado actual de ejecución (pending, running, completed, failed)
- **inputs/outputs**: Conexiones de flujo de datos hacia otros nodos
- **result**: Los valores de retorno después de la ejecución

```go
// Adding a node with a function
g.AddNode("calculate", func(x int) int {
    return x * 2
})
```

### Arista

Una **Arista** define la conexión y el flujo de datos entre nodos. Las aristas controlan:

- **from/to**: Nombres de los nodos de origen y destino
- **edgeType**: Flujo de ejecución Normal, Loop o Branch
- **condition**: Función condicional opcional para controlar la ruta de ejecución
- **weight**: Prioridad de ejecución

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

### Cómo Funcionan Juntos

```
[Node A] ──Edge──> [Node B] ──Edge──> [Node C]
   │                                     ↑
   └───────────Edge─────────────────────┘
```

1. Los **Nodos** realizan el trabajo real (funciones)
2. Las **Aristas** definen el orden de ejecución y el paso de datos
3. Cuando el Nodo A se completa, su salida se pasa al Nodo B a través de la Arista
4. Las condiciones de la Arista determinan si el Nodo B debe ejecutarse

## Resultados de Benchmark

Resultados de benchmark en Apple M1 Pro (darwin/arm64):

| Benchmark | Iterations | Time (ns/op) | Memory (B/op) | Allocations (allocs/op) |
|-------------|------------|----------------|----------------|-------------------------|
| BenchmarkC32-8 | 69,494 | 16,679 | 3,987 | 37 |
| BenchmarkS32-8 | 418,197 | 2,687 | 2,976 | 34 |
| BenchmarkC6-8 | 193,282 | 5,968 | 1,179 | 16 |
| BenchmarkC8x8-8 | 15,262 | 78,383 | 9,027 | 125 |

**Descripciones de los Benchmarks:**

- **C32**: 32 nodos independientes (ejecución totalmente paralela)
- **S32**: 32 nodos en una cadena secuencial
- **C6**: 6 nodos con dependencias en forma de diamante
- **C8x8**: Red profunda de 8 capas x 8 nodos (cada capa completamente conectada a la siguiente)

## Instalación

```bash
go get github.com/zkep/flow
```

## Inicio Rápido

### Ejemplo Básico de Cadena

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

### Ejemplo Básico de Grafo

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

## Documentación

Para la documentación completa, consulta [docs/en-US/doc.md](docs/en-US/doc.md).

## Ejemplos

Consulta el directorio [_examples](_examples) para más ejemplos:

- [basic-chain](_examples/basic-chain/main.go) - Uso básico de Chain
- [basic-graph](_examples/basic-graph/main.go) - Uso básico de Graph
- [advanced-chain](_examples/advanced-chain/main.go) - Características avanzadas de Chain
- [advanced-graph](_examples/advanced-graph/main.go) - Características avanzadas de Graph
- [approval-flow](_examples/approval-flow/main.go) - Ejemplo de flujo de trabajo de aprobación en el mundo real

## Licencia

Licencia MIT
