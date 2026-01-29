package flow

import (
	"fmt"
	"strings"
	"testing"
)

func TestRunAllExamples(t *testing.T) {
	fmt.Println("=" + strings.Repeat("-", 80) + "=")
	fmt.Println("Flow README Examples - Testing All Code Examples")
	fmt.Println("=" + strings.Repeat("-", 80) + "=")

	testCases := []struct {
		name string
		fn   func()
	}{
		{"Chain Example", runChainExample},
		{"Chain with Values Example", runChainWithValuesExample},
		{"Chain with Defer and Run Example", runChainWithDeferAndRunExample},
		{"Complex Chain with Defer, Call and Use Example", runComplexWithDeferCallUseExample},
		{"Chain with Use and Name Example", runChainWithUseAndNameExample},
		{"Graph Example", runGraphExample},
		{"Graph with Condition Example", runGraphWithConditionExample},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fmt.Printf("\n--- [%s] %s ---\n", tc.name, tc.name)
			tc.fn()
		})
	}

	fmt.Println("\n" + "=" + strings.Repeat("-", 80) + "=")
	fmt.Println("All examples completed successfully!")
	fmt.Println("=" + strings.Repeat("-", 80) + "=")
}

func runChainExample() {
	result := NewChain(10).
		Call(func(x int) int { return x * 2 }).
		Call(func(x int) int { return x + 5 }).
		Call(func(x int) string { return fmt.Sprintf("Result: %d", x) }).
		Value()

	fmt.Printf("Result: %v\n", result)
}

func runChainWithValuesExample() {
	c := NewChain(10, 20).
		Call(func(a, b int) (int, int) {
			return a + b, a * b
		})

	values := c.Values()
	fmt.Printf("All values: %v\n", values)

	firstValue := c.Value()
	fmt.Printf("First value: %v\n", firstValue)

	c = c.Call(func(a, b int) string {
		return fmt.Sprintf("Sum: %d, Product: %d", a, b)
	})

	fmt.Printf("Final result: %v\n", c.Value())
}

func runChainWithDeferAndRunExample() {
	var sum int
	var product int

	result := NewChain(1, 2, 3).
		Defer(func(a, b, c int) {
			sum = a + b + c
		}).
		Defer(func(a, b, c int) {
			product = a * b * c
		}).
		Call(func(a, b, c int) int {
			return (a + b + c) / 3
		})

	err := result.Run()
	if err != nil {
		fmt.Printf("Error in Run(): %v\n", err)
	}

	fmt.Printf("Sum: %d, Product: %d, Average: %d\n", sum, product, result.Value())
}

func runComplexWithDeferCallUseExample() {
	var intermediateResults []int
	finalReport := ""

	result := NewChain(10, 20, 30, 40, 50).
		Name("raw_data").
		Defer(func(data ...int) {
			fmt.Printf("Audit: Initial data received with %d items\n", len(data))
		}).
		Call(func(data ...int) []int {
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
			finalReport = fmt.Sprintf("Analysis Report - Min: %d, Max: %d, Avg: %.2f", min, max, avg)
		}).
		Use("raw_data", "validated").
		Call(func(rawData []int, validatedData []int) float64 {
			return float64(len(validatedData)) / float64(len(rawData)) * 100
		})

	err := result.Run()
	if err != nil {
		fmt.Printf("Error in Run(): %v\n", err)
	}

	fmt.Println("=" + strings.Repeat("-", 50) + "=")
	fmt.Println(finalReport)
	fmt.Printf("Validation Retention Rate: %d\n", result.Value())
	fmt.Printf("Intermediate Results (Valid Count, Transform Sum): %v\n", intermediateResults)
	fmt.Println("=" + strings.Repeat("-", 80) + "=")
}

func runChainWithUseAndNameExample() {
	result := NewChain(10).
		Name("initial_value").
		Call(func(x int) int { return x * 2 }).
		Name("doubled").
		Call(func(x int) int { return x + 5 }).
		Name("added").
		Use("initial_value", 1).
		Call(func(a, b int) int { return a + b })

	fmt.Printf("Result: %d\n", result.Value())
}

func runGraphExample() {
	g := NewGraph()
	g.StartNode("start", func() int { return 10 })
	g.AddNode("double", func(x int) int { return x * 2 }, NodeTypeNormal)
	g.AddNode("add5", func(x int) int { return x + 5 }, NodeTypeNormal)
	g.EndNode("end", func(x int) {
		fmt.Println("Result:", x)
	})

	g.AddEdge("start", "double")
	g.AddEdge("double", "add5")
	g.AddEdge("add5", "end")

	err := g.Run()
	if err != nil {
		fmt.Printf("Error in Run(): %v\n", err)
	}
}

func runGraphWithConditionExample() {
	g := NewGraph()
	g.StartNode("input", func() int { return 42 })
	g.AddNode("processA", func(x int) int { return x * 2 }, NodeTypeNormal)
	g.AddNode("processB", func(x int) int { return x + 10 }, NodeTypeNormal)
	g.EndNode("output", func(x int) {
		fmt.Println("Result:", x)
	})

	g.AddEdgeWithCondition("input", "processA", func(x int) bool { return x > 40 })
	g.AddEdgeWithCondition("input", "processB", func(x int) bool { return x <= 40 })
	g.AddEdge("processA", "output")
	g.AddEdge("processB", "output")

	err := g.Run()
	if err != nil {
		fmt.Printf("Error in Run(): %v\n", err)
	}
}
