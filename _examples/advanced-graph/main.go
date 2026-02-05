package main

import (
	"fmt"
	"math"

	"github.com/zkep/flow"
)

func main() {
	fmt.Println("=== Advanced Graph Examples ===\n")

	example1ParallelExecution()
	fmt.Println()
	example3StatusMonitoring()
	fmt.Println()
	example4DataProcessingPipeline()
}

func example1ParallelExecution() {
	fmt.Println("1. Parallel Execution Strategy")
	fmt.Println("   ------------------------------")

	g := flow.NewGraph()

	g.StartNode("init", func() []int {
		fmt.Println("   Initializing data")
		return []int{1, 2, 3, 4, 5}
	})

	g.ParallelNode("calc_square", func(data []int) []int {
		fmt.Printf("   Parallel calc_square: Input = %v\n", data)
		var result []int
		for _, v := range data {
			result = append(result, v*v)
		}
		fmt.Printf("   Parallel calc_square: Output = %v\n", result)
		return result
	})

	g.ParallelNode("calc_cube", func(data []int) []int {
		fmt.Printf("   Parallel calc_cube: Input = %v\n", data)
		var result []int
		for _, v := range data {
			result = append(result, v*v*v)
		}
		fmt.Printf("   Parallel calc_cube: Output = %v\n", result)
		return result
	})

	g.ParallelNode("calc_sqrt", func(data []int) []float64 {
		fmt.Printf("   Parallel calc_sqrt: Input = %v\n", data)
		var result []float64
		for _, v := range data {
			result = append(result, math.Sqrt(float64(v)))
		}
		fmt.Printf("   Parallel calc_sqrt: Output = %v\n", result)
		return result
	})

	g.Node("aggregate_square", func(results []int) string {
		fmt.Printf("   Aggregate square inputs = %v\n", results)
		var total int
		for _, v := range results {
			total += v
		}
		return fmt.Sprintf("Square total: %d", total)
	})

	g.Node("aggregate_cube", func(results []int) string {
		fmt.Printf("   Aggregate cube inputs = %v\n", results)
		var total int
		for _, v := range results {
			total += v
		}
		return fmt.Sprintf("Cube total: %d", total)
	})

	g.Node("aggregate_sqrt", func(results []float64) string {
		fmt.Printf("   Aggregate sqrt inputs = %v\n", results)
		var total float64
		for _, v := range results {
			total += v
		}
		return fmt.Sprintf("Sqrt total: %.2f", total)
	})

	g.EndNode("end_square", func(result string) {
		fmt.Printf("   Final result: %s\n", result)
	})

	g.EndNode("end_cube", func(result string) {
		fmt.Printf("   Final result: %s\n", result)
	})

	g.EndNode("end_sqrt", func(result string) {
		fmt.Printf("   Final result: %s\n", result)
	})

	g.AddEdge("init", "calc_square")
	g.AddEdge("init", "calc_cube")
	g.AddEdge("init", "calc_sqrt")
	g.AddEdge("calc_square", "aggregate_square")
	g.AddEdge("calc_cube", "aggregate_cube")
	g.AddEdge("calc_sqrt", "aggregate_sqrt")
	g.AddEdge("aggregate_square", "end_square")
	g.AddEdge("aggregate_cube", "end_cube")
	g.AddEdge("aggregate_sqrt", "end_sqrt")
	fmt.Println(g.Mermaid())
	fmt.Println("   Executing graph in parallel:")
	err := g.RunParallel()
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
	} else {
		fmt.Println("   Parallel execution completed successfully")
	}

	fmt.Println("\n   Compare with sequential execution:")
	g.ClearStatus()
	err = g.RunSequential()
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
	} else {
		fmt.Println("   Sequential execution completed successfully")
	}
}

func example3StatusMonitoring() {
	fmt.Println("3. Node Status Monitoring")
	fmt.Println("   --------------------------")

	g := flow.NewGraph()

	g.StartNode("start", func() string {
		return "start"
	})

	g.Node("step1", func(data string) string {
		fmt.Println("   Executing step1...")
		return data + "_step1"
	})

	g.Node("step2", func(data string) string {
		fmt.Println("   Executing step2...")
		return data + "_step2"
	})

	g.Node("step3", func(data string) string {
		fmt.Println("   Executing step3...")
		return data + "_step3"
	})

	g.EndNode("end", func(data string) {
		fmt.Println("   Execution completed")
	})

	g.AddEdge("start", "step1")
	g.AddEdge("step1", "step2")
	g.AddEdge("step2", "step3")
	g.AddEdge("step3", "end")

	fmt.Println("   Node status before execution:")
	fmt.Printf("   - start: %d\n", g.NodeStatus("start"))
	fmt.Printf("   - step1: %d\n", g.NodeStatus("step1"))
	fmt.Printf("   - step2: %d\n", g.NodeStatus("step2"))
	fmt.Printf("   - step3: %d\n", g.NodeStatus("step3"))
	fmt.Printf("   - end: %d\n", g.NodeStatus("end"))

	fmt.Println("\n   Executing graph...")
	err := g.Run()

	fmt.Println("\n   Node status after execution:")
	fmt.Printf("   - start: %d, Result: %v\n", g.NodeStatus("start"), g.NodeResult("start"))
	fmt.Printf("   - step1: %d, Result: %v\n", g.NodeStatus("step1"), g.NodeResult("step1"))
	fmt.Printf("   - step2: %d, Result: %v\n", g.NodeStatus("step2"), g.NodeResult("step2"))
	fmt.Printf("   - step3: %d, Result: %v\n", g.NodeStatus("step3"), g.NodeResult("step3"))
	fmt.Printf("   - end: %d, Result: %v\n", g.NodeStatus("end"), g.NodeResult("end"))

	if err != nil {
		fmt.Printf("   Error: %v\n", err)
	}
}

func example4DataProcessingPipeline() {
	fmt.Println("4. Data Processing Pipeline")
	fmt.Println("   ----------------------------")

	type Transaction struct {
		ID     int
		Amount float64
		Status string
	}

	g := flow.NewGraph()

	g.StartNode("load_data", func() []Transaction {
		fmt.Println("   Loading transactions")
		return []Transaction{
			{ID: 1, Amount: 100.0},
			{ID: 2, Amount: 200.0},
			{ID: 3, Amount: 50.0},
			{ID: 4, Amount: 300.0},
		}
	})

	g.Node("filter_valid", func(txs []Transaction) []Transaction {
		fmt.Printf("   Validating %d transactions\n", len(txs))
		var valid []Transaction
		for _, tx := range txs {
			if tx.Amount > 0 && tx.Amount < 1000 {
				tx.Status = "processed"
				valid = append(valid, tx)
			} else {
				tx.Status = "rejected"
			}
		}
		return valid
	})

	g.ParallelNode("calc_summary", func(txs []Transaction) map[string]float64 {
		fmt.Printf("   Calculating summary from %d transactions\n", len(txs))
		summary := make(map[string]float64)
		for _, tx := range txs {
			if tx.Status == "processed" {
				summary["total_amount"] += tx.Amount
				summary["count"]++
			}
		}
		return summary
	})

	g.EndNode("output_summary", func(summary map[string]float64) {
		fmt.Printf("   Output: Total=%.2f, Count=%.0f\n", summary["total_amount"], summary["count"])
	})

	g.AddEdge("load_data", "filter_valid")
	g.AddEdge("filter_valid", "calc_summary")
	g.AddEdge("calc_summary", "output_summary")

	err := g.Run()
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
	}
}
