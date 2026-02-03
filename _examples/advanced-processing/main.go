package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/zkep/flow"
)

type Order struct {
	ID       int
	Items    []string
	Total    float64
	Delivery string
	Status   string
}

func main() {
	fmt.Println("=== Advanced Data Processing Pipeline ===\n")

	example1ComplexDataTransformation()
	fmt.Println()
	example2ValidationAndVerification()
	fmt.Println()
	example3ParallelProcessingWithGraph()
	fmt.Println()
	example4NestedWorkflowOrchestration()
	fmt.Println()
	example5ErrorHandlingAndRecovery()
}

func example1ComplexDataTransformation() {
	fmt.Println("1. Complex Data Transformation Pipeline")
	fmt.Println("   --------------------------------------")

	type InputRecord struct {
		ID          string   `json:"id"`
		Name        string   `json:"name"`
		Tags        []string `json:"tags"`
		Value       float64  `json:"value"`
		Valid       bool     `json:"valid"`
		ProcessedAt time.Time
	}

	type OutputSummary struct {
		TotalRecords   int
		ValidRecords   int
		InvalidRecords int
		AverageValue   float64
		MaxValue       float64
		MinValue       float64
		TagStats       map[string]int
	}

	chain := flow.NewChain()

	chain.Add("input", func() []InputRecord {
		return []InputRecord{
			{ID: "rec1", Name: "Record 1", Tags: []string{"data", "test"}, Value: 100.5, Valid: true},
			{ID: "rec2", Name: "Record 2", Tags: []string{"metadata"}, Value: 50.0, Valid: false},
			{ID: "rec3", Name: "Record 3", Tags: []string{"data", "important"}, Value: 200.2, Valid: true},
			{ID: "rec4", Name: "Record 4", Tags: []string{"metadata"}, Value: -10.5, Valid: false},
			{ID: "rec5", Name: "Record 5", Tags: []string{"data", "test", "important"}, Value: 150.8, Valid: true},
		}
	})

	chain.Add("filter", func(records []InputRecord) []InputRecord {
		var valid []InputRecord
		for _, r := range records {
			if r.Valid && r.Value > 0 {
				r.ProcessedAt = time.Now()
				valid = append(valid, r)
			}
		}
		return valid
	})

	chain.Add("transform", func(validRecords []InputRecord) OutputSummary {
		summary := OutputSummary{
			TotalRecords:   5,
			ValidRecords:   len(validRecords),
			InvalidRecords: 5 - len(validRecords),
			TagStats:       make(map[string]int),
			MaxValue:       -999999,
			MinValue:       999999,
		}

		sum := 0.0
		for _, r := range validRecords {
			sum += r.Value
			if r.Value > summary.MaxValue {
				summary.MaxValue = r.Value
			}
			if r.Value < summary.MinValue {
				summary.MinValue = r.Value
			}
			for _, t := range r.Tags {
				summary.TagStats[t]++
			}
		}

		if len(validRecords) > 0 {
			summary.AverageValue = sum / float64(len(validRecords))
		}

		return summary
	})

	err := chain.Run()
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
		return
	}

	summary, err := chain.Value("transform")
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
		return
	}

	os := summary.(OutputSummary)
	fmt.Printf("   Total records: %d\n", os.TotalRecords)
	fmt.Printf("   Valid records: %d\n", os.ValidRecords)
	fmt.Printf("   Invalid records: %d\n", os.InvalidRecords)
	fmt.Printf("   Average value: %.2f\n", os.AverageValue)
	fmt.Printf("   Max value: %.2f\n", os.MaxValue)
	fmt.Printf("   Min value: %.2f\n", os.MinValue)
	fmt.Printf("   Tag stats: %v\n", os.TagStats)

	expectedValid := 3
	if os.ValidRecords != expectedValid {
		fmt.Printf("   ⚠️  Expected valid records: %d, got: %d\n", expectedValid, os.ValidRecords)
	} else {
		fmt.Printf("   ✓ Validation passed\n")
	}
}

func example2ValidationAndVerification() {
	fmt.Println("2. Validation and Verification")
	fmt.Println("   -----------------------------")

	type Product struct {
		ID         int
		Name       string
		Price      float64
		Stock      int
		IsValid    bool
		Validation []string
	}

	var validationResults []Product

	chain := flow.NewChain()

	chain.Add("input", func() []Product {
		return []Product{
			{ID: 1, Name: "Product A", Price: 10.0, Stock: 5},
			{ID: 2, Name: "Product B", Price: -5.0, Stock: 10},
			{ID: 3, Name: "Product C", Price: 20.0, Stock: 0},
			{ID: 4, Name: "", Price: 15.0, Stock: 3},
			{ID: 5, Name: "Product E", Price: 25.0, Stock: 8},
		}
	})

	chain.Add("validate", func(products []Product) []Product {
		var validated []Product
		for _, p := range products {
			validProduct := p
			validProduct.Validation = []string{}

			if p.Price <= 0 {
				validProduct.Validation = append(validProduct.Validation, "Invalid price")
			}
			if p.Stock < 0 {
				validProduct.Validation = append(validProduct.Validation, "Negative stock")
			}
			if len(p.Name) == 0 {
				validProduct.Validation = append(validProduct.Validation, "Missing name")
			}

			validProduct.IsValid = len(validProduct.Validation) == 0
			validated = append(validated, validProduct)
			validationResults = append(validationResults, validProduct)
		}
		return validated
	})

	chain.Add("verify", func(validated []Product) string {
		validCount := 0
		total := len(validated)

		for _, p := range validated {
			if p.IsValid {
				validCount++
			}
		}

		return fmt.Sprintf("Validation: %d/%d products are valid", validCount, total)
	})

	err := chain.Run()
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
		return
	}

	validation, err := chain.Value("verify")
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
		return
	}

	fmt.Printf("   %s\n", validation)

	for _, p := range validationResults {
		status := "✓"
		if !p.IsValid {
			status = "✗"
		}
		fmt.Printf("   %s Product %d: %s, Price=%.2f, Stock=%d, Issues=%v\n",
			status, p.ID, p.Name, p.Price, p.Stock, p.Validation)
	}

	expectedValid := 3
	actualValid := 0
	for _, p := range validationResults {
		if p.IsValid {
			actualValid++
		}
	}

	if actualValid != expectedValid {
		fmt.Printf("\n   ❌ Validation failed: Expected %d valid products, got %d\n", expectedValid, actualValid)
	} else {
		fmt.Printf("\n   ✓ Validation passed\n")
	}
}

func example3ParallelProcessingWithGraph() {
	fmt.Println("3. Parallel Processing with Graph")
	fmt.Println("   ---------------------------------")

	type DataChunk struct {
		ID    int
		Value int
	}

	type ProcessedResult struct {
		ChunkID  int
		Original int
		Double   int
		Square   int
		Sum      int
	}

	g := flow.NewGraph()

	g.StartNode("init", func() []DataChunk {
		fmt.Println("   [Graph] Initializing data chunks")
		return []DataChunk{
			{ID: 1, Value: 10},
			{ID: 2, Value: 20},
			{ID: 3, Value: 30},
			{ID: 4, Value: 40},
		}
	})

	g.Node("double", func(data []DataChunk) []ProcessedResult {
		fmt.Println("   [Graph] Processing pipeline: Doubling")
		var results []ProcessedResult
		for _, c := range data {
			results = append(results, ProcessedResult{
				ChunkID:  c.ID,
				Original: c.Value,
				Double:   c.Value * 2,
			})
		}
		return results
	})

	g.Node("square", func(results []ProcessedResult) []ProcessedResult {
		fmt.Println("   [Graph] Applying square values")
		for i := range results {
			results[i].Square = results[i].Original * results[i].Original
		}
		return results
	})

	g.Node("sum", func(results []ProcessedResult) []ProcessedResult {
		fmt.Println("   [Graph] Calculating sums")
		for i := range results {
			results[i].Sum = results[i].Double + results[i].Square
		}
		return results
	})

	g.EndNode("output", func(results []ProcessedResult) {
		fmt.Println("   [Graph] Final results:")
		for _, r := range results {
			fmt.Printf("     - Chunk %d: Original=%d, Double=%d, Square=%d, Sum=%d\n",
				r.ChunkID, r.Original, r.Double, r.Square, r.Sum)
		}
	})

	g.AddEdge("init", "double")
	g.AddEdge("double", "square")
	g.AddEdge("square", "sum")
	g.AddEdge("sum", "output")

	err := g.Run()
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
		return
	}

	fmt.Println("   ✓ Parallel processing completed")
}

func example4NestedWorkflowOrchestration() {
	fmt.Println("4. Nested Workflow Orchestration")
	fmt.Println("   ---------------------------------")

	chain := flow.NewChain()

	chain.Add("input", func() []Order {
		return []Order{
			{ID: 1, Items: []string{"Item A", "Item B"}, Total: 100.0, Delivery: "standard"},
			{ID: 2, Items: []string{"Item C"}, Total: 50.0, Delivery: "express"},
			{ID: 3, Items: []string{"Item D", "Item E", "Item F"}, Total: 200.0, Delivery: "standard"},
		}
	})

	chain.Add("process_orders", func(orders []Order) []Order {
		fmt.Println("   [Chain] Processing orders")
		for i := range orders {
			orders[i].Status = "processing"

			currentOrder := &orders[i]

			status, err := processOrderSubchain(currentOrder)
			if err != nil {
				fmt.Printf("     Error processing order %d: %v\n", currentOrder.ID, err)
				currentOrder.Status = "error"
				continue
			}
			currentOrder.Status = status
		}
		return orders
	})

	chain.Add("summarize", func(orders []Order) string {
		processed := 0
		for _, o := range orders {
			if o.Status != "invalid" && o.Status != "error" {
				processed++
			}
		}
		return fmt.Sprintf("%d/%d orders processed successfully", processed, len(orders))
	})

	err := chain.Run()
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
		return
	}

	result, err := chain.Value("summarize")
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
		return
	}

	fmt.Printf("   %s\n", result)

	output, err := chain.Value("process_orders")
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
		return
	}

	for _, o := range output.([]Order) {
		fmt.Printf("   Order %d: Status=%s, Delivery=%s, Total=%.2f, Items=%v\n",
			o.ID, o.Status, o.Delivery, o.Total, o.Items)
	}

	if strings.Contains(result.(string), "3/3") {
		fmt.Printf("   ✓ Validation passed\n")
	} else {
		fmt.Printf("   ❌ Validation failed: Expected all 3 orders to be processed\n")
	}
}

func processOrderSubchain(order *Order) (string, error) {
	var valid bool
	var tax float64
	var discount float64

	chain := flow.NewChain()

	chain.Add("validate", func() any {
		valid = order.Total > 0 && len(order.Items) > 0
		return valid
	})

	chain.Add("calculate_tax", func() any {
		if !valid {
			tax = 0.0
		} else {
			tax = order.Total * 0.08
		}
		return tax
	})

	chain.Add("apply_discount", func() any {
		if !valid {
			discount = 0.0
		} else if order.Delivery == "express" {
			discount = 0.0
		} else if order.Total > 150 {
			discount = order.Total * 0.1
		} else {
			discount = 0.0
		}
		return discount
	})

	chain.Add("finalize", func() any {
		if !valid {
			return "invalid"
		}
		final := (order.Total - discount) + tax
		if final == order.Total {
			return "processed"
		}
		return fmt.Sprintf("processed_with_adjustment")
	})

	err := chain.Run()
	if err != nil {
		return "", err
	}

	finalResult, err := chain.Value("finalize")
	if err != nil {
		return "", err
	}

	return finalResult.(string), nil
}

func example5ErrorHandlingAndRecovery() {
	fmt.Println("5. Error Handling and Recovery")
	fmt.Println("   ------------------------------")

	type Data struct {
		ID    int
		Value float64
		Valid bool
	}

	chain := flow.NewChain()

	chain.Add("input", func() []Data {
		return []Data{
			{ID: 1, Value: 10.0, Valid: true},
			{ID: 2, Value: 20.0, Valid: true},
			{ID: 3, Value: 0.0, Valid: false},
			{ID: 4, Value: 40.0, Valid: true},
			{ID: 5, Value: 50.0, Valid: false},
		}
	})

	chain.Add("calculate", func(data []Data) []float64 {
		fmt.Println("   [Chain] Processing with error simulation")
		var results []float64
		var errors []string

		for _, d := range data {
			if !d.Valid {
				errors = append(errors, fmt.Sprintf("Invalid data: ID=%d", d.ID))
				continue
			}

			if d.Value == 0 {
				errors = append(errors, fmt.Sprintf("Zero division risk: ID=%d", d.ID))
				continue
			}

			result := 100 / d.Value
			results = append(results, result)
		}

		if len(errors) > 0 {
			fmt.Printf("   ⚠️  Warnings: %v\n", errors)
		}

		return results
	})

	chain.Add("verify", func(results []float64) string {
		expectedCount := 3
		actualCount := len(results)

		if actualCount == expectedCount {
			return fmt.Sprintf("✓ Success: Processed %d records as expected", actualCount)
		} else {
			return fmt.Sprintf("❌ Error: Expected %d records, got %d", expectedCount, actualCount)
		}
	})

	err := chain.Run()
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
		return
	}

	results, err := chain.Value("calculate")
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
		return
	}

	verification, err := chain.Value("verify")
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
		return
	}

	fmt.Printf("   %s\n", verification)
	fmt.Printf("   Results: %v\n", results)

	expectedResults := []float64{10, 5, 2.5}
	if len(results.([]float64)) == len(expectedResults) {
		fmt.Printf("   ✓ Output verification passed\n")
	} else {
		fmt.Printf("   ❌ Output verification failed\n")
	}
}
