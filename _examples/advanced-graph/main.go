package main

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/zkep/flow"
)

func main() {
	fmt.Println("=== Advanced Graph Examples ===\n")

	example1SequentialExecution()
	fmt.Println()
	example3StatusMonitoring()
	fmt.Println()
	example4DataProcessingPipeline()
	fmt.Println()
	example5CreditOnboarding()
	fmt.Println()
	example6ETLProcess()
	fmt.Println()
	example7OrderProcessing()
}

func example1SequentialExecution() {
	fmt.Println("1. Sequential Execution Strategy")
	fmt.Println("   ------------------------------")

	g := flow.NewGraph()

	g.AddNode("init", func() []int {
		fmt.Println("   Initializing data")
		return []int{1, 2, 3, 4, 5}
	})

	g.AddNode("calc_square", func(data []int) []int {
		fmt.Printf("   calc_square: Input = %v\n", data)
		var result []int
		for _, v := range data {
			result = append(result, v*v)
		}
		fmt.Printf("   calc_square: Output = %v\n", result)
		return result
	})

	g.AddNode("calc_cube", func(data []int) []int {
		fmt.Printf("   calc_cube: Input = %v\n", data)
		var result []int
		for _, v := range data {
			result = append(result, v*v*v)
		}
		fmt.Printf("   calc_cube: Output = %v\n", result)
		return result
	})

	g.AddNode("calc_sqrt", func(data []int) []float64 {
		fmt.Printf("   	 calc_sqrt: Input = %v\n", data)
		var result []float64
		for _, v := range data {
			result = append(result, math.Sqrt(float64(v)))
		}
		fmt.Printf("   	 calc_sqrt: Output = %v\n", result)
		return result
	})

	g.AddNode("aggregate_square", func(results []int) string {
		fmt.Printf("   Aggregate square inputs = %v\n", results)
		var total int
		for _, v := range results {
			total += v
		}
		return fmt.Sprintf("Square total: %d", total)
	})

	g.AddNode("aggregate_cube", func(results []int) string {
		fmt.Printf("   Aggregate cube inputs = %v\n", results)
		var total int
		for _, v := range results {
			total += v
		}
		return fmt.Sprintf("Cube total: %d", total)
	})

	g.AddNode("aggregate_sqrt", func(results []float64) string {
		fmt.Printf("   Aggregate sqrt inputs = %v\n", results)
		var total float64
		for _, v := range results {
			total += v
		}
		return fmt.Sprintf("Sqrt total: %.2f", total)
	})

	g.AddNode("end_square", func(result string) {
		fmt.Printf("   Final result: %s\n", result)
	})

	g.AddNode("end_cube", func(result string) {
		fmt.Printf("   Final result: %s\n", result)
	})

	g.AddNode("end_sqrt", func(result string) {
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
	fmt.Println("   Executing graph:")
	err := g.Run()
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
	} else {
		fmt.Println("   Execution completed successfully")
	}

	fmt.Println("\n   Compare with sequential execution:")
	g.ClearStatus()
	err = g.RunSequentialWithContext(context.Background())
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

	g.AddNode("start", func() string {
		return "start"
	})

	g.AddNode("step1", func(data string) string {
		fmt.Println("   Executing step1...")
		return data + "_step1"
	})

	g.AddNode("step2", func(data string) string {
		fmt.Println("   Executing step2...")
		return data + "_step2"
	})

	g.AddNode("step3", func(data string) string {
		fmt.Println("   Executing step3...")
		return data + "_step3"
	})

	g.AddNode("end", func(data string) {
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

	g.AddNode("load_data", func() []Transaction {
		fmt.Println("   Loading transactions")
		return []Transaction{
			{ID: 1, Amount: 100.0},
			{ID: 2, Amount: 200.0},
			{ID: 3, Amount: 50.0},
			{ID: 4, Amount: 300.0},
		}
	})

	g.AddNode("filter_valid", func(txs []Transaction) []Transaction {
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

	g.AddNode("calc_summary", func(txs []Transaction) map[string]float64 {
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

	g.AddNode("output_summary", func(summary map[string]float64) {
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

func example5CreditOnboarding() {
	fmt.Println("5. Credit Onboarding")
	fmt.Println("   ----------------------------")

	graph := flow.NewGraph()

	graph.AddNode("collectInfo", func() map[string]string {
		return map[string]string{
			"name":  "John Doe",
			"email": "john@example.com",
			"score": "85",
		}
	})

	graph.AddNode("creditCheck", func(info map[string]string) (int, error) {
		score, _ := strconv.Atoi(info["score"])
		fmt.Printf("Credit check: score = %d\n", score)
		return score, nil
	})

	graph.AddNode("retryCreditCheck", func(score int) int {
		fmt.Printf("Retrying credit check, current score: %d\n", score)
		return score + 5
	})
	graph.AddLoopEdge("retryCreditCheck", func(score int) bool {
		return score < 70
	}, 3)

	graph.AddNode("backgroundCheck", func(info map[string]string) bool {
		time.Sleep(100 * time.Millisecond)
		return true
	})

	graph.AddNode("documentCheck", func(info map[string]string) bool {
		time.Sleep(150 * time.Millisecond)
		return true
	})

	graph.AddNode("approval", func(creditOk, backgroundOk, documentOk bool) string {
		if creditOk && backgroundOk && documentOk {
			return "approve"
		}
		return "reject"
	})

	graph.AddNode("sendApproval", func(decision string) {
		fmt.Printf("Approving customer (decision: %s)\n", decision)
	})

	graph.AddNode("sendRejection", func(decision string) {
		fmt.Printf("Rejecting customer (decision: %s)\n", decision)
	})

	graph.AddNode("onboardingComplete", func() {
		fmt.Println("Customer onboarding completed successfully")
	})

	graph.AddNode("onboardingFailed", func() {
		fmt.Println("Customer onboarding failed")
	})

	graph.AddEdge("collectInfo", "creditCheck")
	graph.AddEdge("collectInfo", "backgroundCheck")
	graph.AddEdge("collectInfo", "documentCheck")
	graph.AddEdge("creditCheck", "retryCreditCheck")
	graph.AddNode("evaluateCredit", func(score int) bool {
		return score >= 70
	})
	graph.AddEdge("retryCreditCheck", "evaluateCredit")
	graph.AddEdge("evaluateCredit", "approval")
	graph.AddEdge("backgroundCheck", "approval")
	graph.AddEdge("documentCheck", "approval")
	graph.AddBranchEdge("approval", map[string]any{
		"sendApproval":  func(decision string) bool { return decision == "approve" },
		"sendRejection": func(decision string) bool { return decision == "reject" },
	})
	graph.AddEdge("sendApproval", "onboardingComplete")
	graph.AddEdge("sendRejection", "onboardingFailed")
	fmt.Println(graph.Mermaid())
	if err := graph.RunWithContext(context.Background()); err != nil {
		fmt.Printf("Onboarding process failed: %v\n", err)
	}
}

func example6ETLProcess() {
	fmt.Println("6. ETL Process")
	fmt.Println("   ----------------------------")

	graph := flow.NewGraph()

	graph.AddNode("extractFromAPI", func() []map[string]interface{} {
		return []map[string]interface{}{
			{"id": 1, "name": "Product A", "price": 100},
			{"id": 2, "name": "Product B", "price": 200},
		}
	})

	graph.AddNode("extractFromDatabase", func() []map[string]interface{} {
		return []map[string]interface{}{
			{"id": 3, "name": "Product C", "price": 150},
			{"id": 4, "name": "Product D", "price": 250},
		}
	})

	graph.AddNode("combineData", func(apiData, dbData []map[string]interface{}) []map[string]interface{} {
		combined := append(apiData, dbData...)
		return combined
	})

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

	graph.AddNode("retryValidation", func(countInvalid int, data []map[string]interface{}) (int, []map[string]interface{}) {
		fmt.Printf("Retrying validation, attempts remaining...\n")
		return countInvalid - 1, data
	})
	graph.AddLoopEdge("retryValidation", func(countInvalid int, data []map[string]interface{}) bool {
		return countInvalid > 0
	}, 2)

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

	graph.AddNode("loadToWarehouse", func(data []map[string]interface{}) error {
		fmt.Printf("Loading %d items to data warehouse\n", len(data))
		for _, item := range data {
			fmt.Printf("Loading: %v\n", item)
		}
		return nil
	})

	graph.AddNode("loadToPremium", func(data []map[string]interface{}) error {
		fmt.Printf("Loading %d high-value items to premium storage\n", len(data))
		for _, item := range data {
			fmt.Printf("Premium loading: %v\n", item)
		}
		return nil
	})

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
	fmt.Println(graph.Mermaid())
	if err := graph.RunWithContext(context.Background()); err != nil {
		fmt.Printf("ETL process failed: %v\n", err)
	}
}

func example7OrderProcessing() {
	fmt.Println("7. Order Processing")
	fmt.Println("   ----------------------------")

	graph := flow.NewGraph()

	graph.AddNode("createOrder", func() map[string]interface{} {
		return map[string]interface{}{
			"orderId":    "ORD-123",
			"customerId": "CUST-456",
			"items":      []string{"ITEM-1", "ITEM-2"},
			"total":      300,
		}
	})

	graph.AddNode("checkInventory", func(order map[string]interface{}) (int, map[string]interface{}) {
		fmt.Println("Checking inventory...")
		retryCount := 0
		return retryCount, order
	})

	graph.AddNode("retryInventory", func(retryCount int, order map[string]interface{}) (int, map[string]interface{}) {
		fmt.Printf("Retrying inventory check (attempt %d)...\n", retryCount+1)
		return retryCount + 1, order
	})
	graph.AddLoopEdge("retryInventory", func(retryCount int, order map[string]interface{}) bool {
		return retryCount < 2
	}, 3)

	graph.AddNode("evaluateInventory", func(retryCount int, order map[string]interface{}) bool {
		fmt.Println("Inventory available after retries")
		return true
	})

	graph.AddNode("processPayment", func(available bool) bool {
		fmt.Println("Processing payment...")
		return true
	})

	graph.AddNode("updateInventory", func(available bool) bool {
		fmt.Println("Updating inventory...")
		return true
	})

	graph.AddNode("shipOrder", func(success bool) string {
		fmt.Println("Shipping order...")
		return "SHIP-789"
	})

	graph.AddNode("sendNotification", func(trackingId string) {
		fmt.Printf("Sending notification with tracking %s\n", trackingId)
	})

	graph.AddNode("cancelPayment", func(success bool) {
		fmt.Println("Cancelling payment for order")
	})

	graph.AddNode("restoreInventory", func(available bool) {
		fmt.Println("Restoring inventory for order")
	})

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
	fmt.Println(graph.Mermaid())
	if err := graph.RunWithContext(context.Background()); err != nil {
		fmt.Printf("Order processing failed: %v\n", err)
	}
}
