package main

import (
	"fmt"

	"github.com/zkep/flow"
)

func main() {
	fmt.Println("=== Advanced Chain Examples ===\n")

	example1BasicAdvancedChain()
	fmt.Println()
	example2DataPipeline()
	fmt.Println()
	example3ComplexWorkflow()
	fmt.Println()
}

func example1BasicAdvancedChain() {
	fmt.Println("1. Basic Advanced Chain")
	fmt.Println("   -----------------------")

	chain := flow.NewChain()

	chain.Add("step1", func() []int {
		return []int{1, 2, 3, 4, 5}
	})

	chain.Add("step2", func(data []int) int {
		sum := 0
		for _, v := range data {
			sum += v
		}
		return sum
	})

	chain.Add("step3", func(sum int) float64 {
		return float64(sum) / 5
	})

	err := chain.Run()
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
		return
	}

	sum, err := chain.Value("step2")
	if err != nil {
		fmt.Printf("   Error getting sum: %v\n", err)
	} else {
		fmt.Printf("   Sum: %d\n", sum)
	}

	average, err := chain.Value("step3")
	if err != nil {
		fmt.Printf("   Error getting average: %v\n", err)
	} else {
		fmt.Printf("   Average: %.2f\n", average)
	}
}

func example2DataPipeline() {
	fmt.Println("2. Data Pipeline")
	fmt.Println("   -----------------")

	chain := flow.NewChain()

	chain.Add("step1", func() []int {
		return []int{10, 20, -5, 30, -10, 40, -15}
	})

	chain.Add("step2", func(data []int) []int {
		var valid []int
		for _, v := range data {
			if v > 0 {
				valid = append(valid, v)
			}
		}
		return valid
	})

	chain.Add("step3", func(data []int) int {
		sum := 0
		for _, v := range data {
			sum += v
		}
		return sum
	})

	err := chain.Run()
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
		return
	}

	sum, err := chain.Value("step3")
	if err != nil {
		fmt.Printf("   Error getting sum: %v\n", err)
		return
	}
	filtered, err := chain.Value("step2")
	if err != nil {
		fmt.Printf("   Error getting filtered: %v\n", err)
		return
	}
	fmt.Printf("   Original chain - step3: %v, step2: %v\n", sum, filtered)

	newChain := chain.Use("step2", "step3")
	newChain.Add("combined_step", func(data []int, sum int) float64 {
		return float64(sum) / float64(len(data))
	})

	err = newChain.Run()
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
	} else {
		average, err := newChain.Value("combined_step")
		if err != nil {
			fmt.Printf("   Error getting average: %v\n", err)
		} else {
			fmt.Printf("   Average: %.2f\n", average)
		}
	}
}

func example3ComplexWorkflow() {
	fmt.Println("3. Complex Workflow")
	fmt.Println("   --------------------")

	type Order struct {
		ID         int
		Items      []string
		Total      float64
		Discount   float64
		Tax        float64
		FinalPrice float64
	}

	chain := flow.NewChain()

	chain.Add("step1", func() []Order {
		return []Order{
			{ID: 1, Items: []string{"A", "B"}, Total: 100.0},
			{ID: 2, Items: []string{"C"}, Total: 50.0},
			{ID: 3, Items: []string{"D", "E", "F"}, Total: 150.0},
		}
	})

	chain.Add("step2", func(orders []Order) []Order {
		for i := range orders {
			if orders[i].Total > 100 {
				orders[i].Discount = orders[i].Total * 0.1
			}
		}
		return orders
	})

	chain.Add("step3", func(orders []Order) []Order {
		for i := range orders {
			orders[i].Tax = (orders[i].Total - orders[i].Discount) * 0.08
		}
		return orders
	})

	chain.Add("step4", func(orders []Order) []Order {
		for i := range orders {
			orders[i].FinalPrice = orders[i].Total - orders[i].Discount + orders[i].Tax
		}
		return orders
	})

	err := chain.Run()
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
		return
	}

	processedOrders, err := chain.Values("step4")
	if err != nil {
		fmt.Printf("   Error getting orders: %v\n", err)
		return
	}

	fmt.Printf("   Processed Orders:\n")
	for _, order := range processedOrders[0].([]Order) {
		fmt.Printf("   Order %d: Total=%.2f, Discount=%.2f, Tax=%.2f, Final=%.2f\n",
			order.ID, order.Total, order.Discount, order.Tax, order.FinalPrice)
	}
}
