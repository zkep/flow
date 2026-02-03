package main

import (
	"fmt"
	"strings"

	"github.com/zkep/flow"
)

func main() {
	fmt.Println("=== Basic Graph Examples ===\n")

	example1SimpleFlow()
	fmt.Println()
	example2NodeTypes()
	fmt.Println()
	example4GraphVisualization()
}

func example1SimpleFlow() {
	fmt.Println("1. Simple Flow Graph")
	fmt.Println("   ---------------------")

	g := flow.NewGraph()

	g.StartNode("start", func() int {
		fmt.Println("   Executing start node")
		return 10
	})

	g.Node("process1", func(x int) int {
		fmt.Printf("   Executing process1: %d * 2 = %d\n", x, x*2)
		return x * 2
	})

	g.Node("process2", func(x int) int {
		fmt.Printf("   Executing process2: %d + 5 = %d\n", x, x+5)
		return x + 5
	})

	g.EndNode("end", func(x int) {
		fmt.Printf("   Executing end node: Final result is %d\n", x)
	})

	g.AddEdge("start", "process1")
	g.AddEdge("process1", "process2")
	g.AddEdge("process2", "end")

	err := g.Run()
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
	} else {
		fmt.Println("   Execution completed successfully")
	}
}

func example2NodeTypes() {
	fmt.Println("2. Different Node Types")
	fmt.Println("   ------------------------")

	g := flow.NewGraph()

	g.StartNode("input", func() int {
		fmt.Println("   [Start] Node: Input")
		return 42
	})

	g.Node("branch", func(x int) int {
		fmt.Printf("   [Branch] Node: Check if %d > 40\n", x)
		if x > 40 {
			return 1
		}
		return 0
	})

	g.Node("processA", func(x int) int {
		fmt.Printf("   [Parallel] Node A: %d * 2\n", x)
		return x * 2
	})

	g.Node("processB", func(x int) int {
		fmt.Printf("   [Parallel] Node B: %d + 10\n", x)
		return x + 10
	})

	g.Node("loop", func(x int) int {
		fmt.Printf("   [Loop] Node: Loop %d times\n", x)
		return x - 1
	})

	g.EndNode("output", func(result int) {
		fmt.Printf("   [End] Node: Result = %d\n", result)
	})

	g.AddEdge("input", "branch")
	g.AddEdgeWithCondition("branch", "processA", func(x int) bool { return x > 0 })
	g.AddEdgeWithCondition("branch", "processB", func(x int) bool { return x == 0 })
	g.AddEdge("processA", "loop")
	g.AddEdge("processB", "loop")
	g.AddEdge("loop", "output")

	err := g.Run()
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
	}
}

func example4GraphVisualization() {
	fmt.Println("4. Graph Visualization")
	fmt.Println("   ----------------------")

	g := flow.NewGraph()

	g.StartNode("read", func() string {
		return "data"
	})

	g.Node("validate", func(data string) bool {
		return len(data) > 0
	})

	g.Node("transform", func(data string) string {
		return data + "_processed"
	})

	g.Node("analyze", func(data string) float64 {
		return float64(len(data))
	})

	g.EndNode("store", func(result float64) {
	})

	g.AddEdge("read", "validate")
	g.AddEdge("validate", "transform")
	g.AddEdge("transform", "analyze")
	g.AddEdge("analyze", "store")

	fmt.Println("   Mermaid Diagram:")
	fmt.Println("   " + strings.Repeat("-", 60))
	fmt.Println(g.Mermaid())
	fmt.Println("   " + strings.Repeat("-", 60))
	fmt.Println()

	fmt.Println("   Graphviz Diagram:")
	fmt.Println("   " + strings.Repeat("-", 60))
	fmt.Println(g.String())
	fmt.Println("   " + strings.Repeat("-", 60))
}
