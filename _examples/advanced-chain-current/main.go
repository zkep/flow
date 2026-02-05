package main

import (
	"fmt"

	"github.com/zkep/flow"
)

func main() {
	fmt.Println("=== Advanced Chain Examples (Current Implementation) ===\n")

	example1MultipleSteps()
	fmt.Println()
	example2DataPipeline()
	fmt.Println()
	example3ComplexWorkflow()
	fmt.Println()
	example4ErrorPropagation()
	fmt.Println()
}

func example1MultipleSteps() {
	fmt.Println("1. Multiple Steps Pipeline")
	fmt.Println("   ------------------------")

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

	chain.Add("step4", func(z int) int {
		return z - 3
	})

	chain.Add("step5", func(w int) int {
		return w * 4
	})

	err := chain.Run()
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
		return
	}

	value, err := chain.Value("step5")
	if err != nil {
		fmt.Printf("   Error getting value: %v\n", err)
		return
	}

	fmt.Printf("   Step 1: 10\n")
	fmt.Printf("   Step 2: 10 * 2 = 20\n")
	fmt.Printf("   Step 3: 20 + 5 = 25\n")
	fmt.Printf("   Step 4: 25 - 3 = 22\n")
	fmt.Printf("   Step 5: 22 * 4 = 88\n")
	fmt.Printf("   Final Result: %v\n", value)
}

func example2DataPipeline() {
	fmt.Println("2. Data Pipeline")
	fmt.Println("   ----------------")

	type Product struct {
		ID    int
		Name  string
		Price float64
		Valid bool
	}

	chain := flow.NewChain()

	chain.Add("input", func() []Product {
		return []Product{
			{ID: 1, Name: "Product A", Price: 10.0},
			{ID: 2, Name: "Product B", Price: 20.0},
			{ID: 3, Name: "Product C", Price: -5.0},
			{ID: 4, Name: "Product D", Price: 30.0},
		}
	})

	chain.Add("validate", func(data []Product) []Product {
		var validated []Product
		for _, p := range data {
			if p.Price > 0 {
				p.Valid = true
				validated = append(validated, p)
			}
		}
		return validated
	})

	chain.Add("calculate", func(data []Product) float64 {
		var total float64
		for _, p := range data {
			total += p.Price
		}
		return total
	})

	err := chain.Run()
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
		return
	}

	validated, err := chain.Value("validate")
	if err != nil {
		fmt.Printf("   Error getting validated products: %v\n", err)
		return
	}

	total, err := chain.Value("calculate")
	if err != nil {
		fmt.Printf("   Error getting total: %v\n", err)
		return
	}

	fmt.Printf("   Input: %d products\n", 4)
	fmt.Printf("   Result: Validated %d products, total value: %.2f\n",
		len(validated.([]Product)), total)
}

func example3ComplexWorkflow() {
	fmt.Println("3. Complex Workflow")
	fmt.Println("   -------------------")

	type Order struct {
		ID         int
		Items      []string
		Total      float64
		Discount   float64
		Tax        float64
		FinalPrice float64
	}

	chain := flow.NewChain()

	chain.Add("input", func() []Order {
		return []Order{
			{ID: 1, Items: []string{"A", "B"}, Total: 100.0},
			{ID: 2, Items: []string{"C"}, Total: 50.0},
			{ID: 3, Items: []string{"D", "E", "F"}, Total: 150.0},
		}
	})

	chain.Add("apply_discounts", func(orders []Order) []Order {
		for i := range orders {
			if orders[i].Total > 100 {
				orders[i].Discount = orders[i].Total * 0.1
			}
		}
		return orders
	})

	chain.Add("calculate_tax", func(orders []Order) []Order {
		for i := range orders {
			orders[i].Tax = (orders[i].Total - orders[i].Discount) * 0.08
		}
		return orders
	})

	chain.Add("finalize", func(orders []Order) []Order {
		for i := range orders {
			orders[i].FinalPrice = orders[i].Total - orders[i].Discount + orders[i].Tax
		}
		return orders
	})

	chain.Add("summarize", func(orders []Order) string {
		var totalRevenue float64
		for _, order := range orders {
			totalRevenue += order.FinalPrice
		}
		return fmt.Sprintf("Success: %d orders processed, total revenue: %.2f",
			len(orders), totalRevenue)
	})

	err := chain.Run()
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
		return
	}

	result, err := chain.Value("summarize")
	if err != nil {
		fmt.Printf("   Error getting result: %v\n", err)
		return
	}

	fmt.Printf("   Result: %v\n", result)
}

func example4ErrorPropagation() {
	fmt.Println("4. Error Propagation")
	fmt.Println("   -------------------")

	fmt.Println("   Example 1: Error in the middle of chain")
	chain1 := flow.NewChain()

	chain1.Add("step1", func() int {
		return 10
	})

	chain1.Add("step2", func(x int) (int, error) {
		return 0, &flow.ChainError{Message: "test error"}
	})

	chain1.Add("step3", func(y int) int {
		return y + 5
	})

	err := chain1.Run()
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
		fmt.Printf("   step3 not executed due to error propagation\n")
	}

	fmt.Println()
	fmt.Println("   Example 2: Error with non-function")
	chain2 := flow.NewChain()

	chain2.Add("step1", func() int {
		return 10
	})

	chain2.Add("step2", "astep2 input is string")

	chain2.Add("step3", func(y int) int {
		return y + 5
	})

	err = chain2.Run()
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
	}

	value, err := chain2.Value("step2")
	if err == nil {
		fmt.Printf("   step2 value: %v\n", value)
	}

	fmt.Println()
	fmt.Println("   Example 3: Argument type mismatch")
	chain3 := flow.NewChain()

	chain3.Add("step1", func() string {
		return "string"
	})

	chain3.Add("step2", func(x int) int {
		return x * 2
	})

	err = chain3.Run()
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
	}
}
