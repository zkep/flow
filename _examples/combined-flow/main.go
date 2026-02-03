package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/zkep/flow"
)

func main() {
	fmt.Println("=== Combined Chain and Graph Examples ===\n")

	example1ChainWithGraphSteps()
	fmt.Println()
	example2GraphWithChainSubflows()
	fmt.Println()
	example3WorkflowOrchestration()
	fmt.Println()
	example4DataTransformationPipeline()
}

func example1ChainWithGraphSteps() {
	fmt.Println("1. Chain with Graph Steps")
	fmt.Println("   --------------------------")

	type Product struct {
		ID    int
		Name  string
		Price float64
	}

	chain := flow.NewChain()

	chain.Add("input", func() []Product {
		return []Product{
			{ID: 1, Name: "Product A", Price: 10.0},
			{ID: 2, Name: "Product B", Price: 20.0},
			{ID: 3, Name: "Product C", Price: 30.0},
			{ID: 4, Name: "Invalid Product", Price: -5.0},
		}
	})

	chain.Add("validate", func(data []Product) []Product {
		fmt.Printf("   Step 1: Validating %d products\n", len(data))
		var validated []Product
		for _, p := range data {
			if p.Price > 0 {
				validated = append(validated, p)
			}
		}
		return validated
	})

	chain.Add("summarize", func(data []Product) string {
		fmt.Printf("   Step 2: Summarizing %d products\n", len(data))
		return fmt.Sprintf("Validated %d products", len(data))
	})

	err := chain.Run()
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
		return
	}

	input, err := chain.Value("input")
	if err != nil {
		fmt.Printf("   Error getting input: %v\n", err)
		return
	}

	validated, err := chain.Value("validate")
	if err != nil {
		fmt.Printf("   Error getting validated: %v\n", err)
		return
	}

	summary, err := chain.Value("summarize")
	if err != nil {
		fmt.Printf("   Error getting summary: %v\n", err)
		return
	}

	fmt.Println()
	fmt.Printf("   Input: %d products\n", len(input.([]Product)))
	fmt.Printf("   Validated: %d products\n", len(validated.([]Product)))
	fmt.Printf("   Invalid: %d products\n", len(input.([]Product)) - len(validated.([]Product)))
	fmt.Printf("   Summary: %v\n", summary)
}

func example2GraphWithChainSubflows() {
	fmt.Println("2. Graph with Chain Subflows")
	fmt.Println("   ----------------------------")

	g := flow.NewGraph()

	g.StartNode("init_data", func() []int {
		fmt.Println("   [Graph] Initializing data")
		return []int{10, 20, 30, 40, 50}
	})

	g.Node("graph_transform", func(data []int) []int {
		fmt.Printf("   [Graph] Executing chain transformation on %d items\n", len(data))

		chain := flow.NewChain()
		chain.Add("input", func() []int {
			return data
		})
		chain.Add("double", func(items []int) []int {
			fmt.Printf("     - Chain: Doubling %d items\n", len(items))
			var result []int
			for _, v := range items {
				result = append(result, v*2)
			}
			return result
		})
		chain.Add("add10", func(items []int) []int {
			fmt.Printf("     - Chain: Adding 10 to %d items\n", len(items))
			var result []int
			for _, v := range items {
				result = append(result, v+10)
			}
			return result
		})
		chain.Add("filter", func(items []int) []int {
			fmt.Printf("     - Chain: Filtering %d items > 50\n", len(items))
			var result []int
			for _, v := range items {
				if v > 50 {
					result = append(result, v)
				}
			}
			return result
		})

		err := chain.Run()
		if err != nil {
			fmt.Printf("     - Chain error: %v\n", err)
			return nil
		}

		transformed, err := chain.Value("filter")
		if err != nil {
			fmt.Printf("     - Chain error: %v\n", err)
			return nil
		}

		fmt.Printf("     - Chain result: %v\n", transformed)
		return transformed.([]int)
	})

	g.Node("graph_aggregate", func(transformed []int) string {
		fmt.Printf("   [Graph] Aggregating %d transformed items\n", len(transformed))
		total := 0
		for _, v := range transformed {
			total += v
		}
		return fmt.Sprintf("Transformed: %d items, Total: %d", len(transformed), total)
	})

	g.EndNode("end", func(result string) {
		fmt.Printf("   [Graph] Final result: %s\n", result)
	})

	g.AddEdge("init_data", "graph_transform")
	g.AddEdge("graph_transform", "graph_aggregate")
	g.AddEdge("graph_aggregate", "end")

	err := g.Run()
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
	}
}

func example3WorkflowOrchestration() {
	fmt.Println("3. Workflow Orchestration")
	fmt.Println("   -------------------------")

	type Order struct {
		ID       int
		Status   string
		Total    float64
		Discount float64
		Tax      float64
	}

	var processingResults []string

	start := time.Now()

	chain := flow.NewChain()

	chain.Add("input", func() []Order {
		return []Order{
			{ID: 1, Total: 100.0},
			{ID: 2, Total: 50.0},
			{ID: 3, Total: 150.0},
			{ID: 4, Total: 200.0},
		}
	})

	chain.Add("validate", func(orderList []Order) []Order {
		fmt.Println("   [Chain] Validating orders")
		var validated []Order
		for _, o := range orderList {
			if o.Total > 0 {
				o.Status = "validated"
				validated = append(validated, o)
			}
		}
		return validated
	})

	chain.Add("discount", func(orderResults []Order) []Order {
		fmt.Printf("   [Chain] Applying discounts to %d validated orders\n", len(orderResults))
		for i := range orderResults {
			if orderResults[i].Total > 100 {
				orderResults[i].Discount = orderResults[i].Total * 0.1
			}
			orderResults[i].Status = "discount_applied"
		}
		return orderResults
	})

	chain.Add("tax", func(orderResults []Order) []Order {
		fmt.Printf("   [Chain] Calculating tax for %d orders\n", len(orderResults))
		for _, o := range orderResults {
			o.Tax = (o.Total - o.Discount) * 0.08
			o.Status = "tax_calculated"
			o.Total = o.Total - o.Discount + o.Tax
			o.Status = "finalized"
			processingResults = append(processingResults,
				fmt.Sprintf("Order %d: Total=%.2f, Discount=%.2f, Tax=%.2f, Final=%.2f, Status=%s",
					o.ID, o.Total-o.Discount-o.Tax, o.Discount, o.Tax, o.Total, o.Status))
		}
		return orderResults
	})

	chain.Add("summarize", func(orders []Order) string {
		var totalRevenue float64
		for _, o := range orders {
			totalRevenue += o.Total
		}
		return fmt.Sprintf("Success: %d orders processed, total revenue: %.2f", len(orders), totalRevenue)
	})

	err := chain.Run()
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
		return
	}

	elapsed := time.Since(start)

	fmt.Println()
	fmt.Println("   " + strings.Repeat("=", 70))
	fmt.Println("   Processing Results")
	fmt.Println("   " + strings.Repeat("=", 70))
	for _, r := range processingResults {
		fmt.Println("   " + r)
	}
	fmt.Println("   " + strings.Repeat("=", 70))

	finalStatus, err := chain.Value("summarize")
	if err != nil {
		fmt.Printf("   Error getting final status: %v\n", err)
		return
	}
	fmt.Printf("   Final Status: %s\n", finalStatus)
	fmt.Printf("   Execution time: %v\n", elapsed)
}

func example4DataTransformationPipeline() {
	fmt.Println("4. Data Transformation Pipeline")
	fmt.Println("   -------------------------------")

	type DataPoint struct {
		ID     int
		Value  float64
		Status string
	}

	type PipelineResult struct {
		OriginalCount  int
		ProcessedCount int
		SkippedCount   int
		FinalValues    []float64
		Min            float64
		Max            float64
		Average        float64
	}

	var pipelineResult PipelineResult

	chain := flow.NewChain()

	chain.Add("input", func() []DataPoint {
		return []DataPoint{
			{ID: 1, Value: 10.0},
			{ID: 2, Value: -5.0},
			{ID: 3, Value: 30.0},
			{ID: 4, Value: 25.0},
			{ID: 5, Value: -2.0},
		}
	})

	chain.Add("filter", func(data []DataPoint) []DataPoint {
		fmt.Println("   [Chain] Filtering invalid data")
		var valid []DataPoint
		for _, dp := range data {
			if dp.Value > 0 {
				dp.Status = "validated"
				valid = append(valid, dp)
			}
		}
		return valid
	})

	chain.Add("process", func(data []DataPoint) PipelineResult {
		pipelineResult.OriginalCount = 5
		pipelineResult.ProcessedCount = len(data)
		pipelineResult.SkippedCount = 5 - len(data)

		res := PipelineResult{
			OriginalCount:  pipelineResult.OriginalCount,
			ProcessedCount: pipelineResult.ProcessedCount,
			SkippedCount:   pipelineResult.SkippedCount,
			FinalValues:    make([]float64, len(data)),
			Min:            999999,
			Max:            -999999,
			Average:        0,
		}
		sum := 0.0
		for i, v := range data {
			var processedValue float64
			if v.Value > 20 {
				processedValue = v.Value * 0.8
			} else {
				processedValue = v.Value * 1.2
			}
			res.FinalValues[i] = processedValue
			sum += processedValue
			if processedValue < res.Min {
				res.Min = processedValue
			}
			if processedValue > res.Max {
				res.Max = processedValue
			}
		}
		res.Average = sum / float64(len(data))
		return res
	})

	err := chain.Run()
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
		return
	}

	finalResult, err := chain.Value("process")
	if err != nil {
		fmt.Printf("   Error getting final result: %v\n", err)
		return
	}
	pr := finalResult.(PipelineResult)

	fmt.Println()
	fmt.Println("   " + strings.Repeat("=", 70))
	fmt.Println("   Pipeline Result Summary")
	fmt.Println("   " + strings.Repeat("=", 70))
	fmt.Printf("   Original Count: %d\n", pr.OriginalCount)
	fmt.Printf("   Processed Count: %d\n", pr.ProcessedCount)
	fmt.Printf("   Skipped Count: %d\n", pr.SkippedCount)
	fmt.Printf("   Final Values: %v\n", pr.FinalValues)
	fmt.Printf("   Min Value: %.2f\n", pr.Min)
	fmt.Printf("   Max Value: %.2f\n", pr.Max)
	fmt.Printf("   Average: %.2f\n", pr.Average)
	fmt.Println("   " + strings.Repeat("=", 70))
}