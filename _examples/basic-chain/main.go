package main

import (
	"fmt"
	"math"

	"github.com/zkep/flow"
)

type Person struct {
	Name string
	Age  int
}

func main() {
	fmt.Println("=== Basic Chain Examples ===\n")

	example1BasicFlow()
	fmt.Println()
	example2MultipleArgs()
	fmt.Println()
	example3MultipleReturns()
	fmt.Println()
	example4NoArguments()
	fmt.Println()
	example5NoReturn()
	fmt.Println()
	example6TypeConversion()
	fmt.Println()
	example7ErrorHandling()
	fmt.Println()
}

func example1BasicFlow() {
	fmt.Println("1. Basic Flow Execution")
	fmt.Println("   ---------------------")

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
		fmt.Printf("   Error: %v\n", err)
		return
	}

	result, err := chain.Value("step3")
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
		return
	}

	fmt.Printf("   Input: 10\n")
	fmt.Printf("   10 * 2 = 20, then 20 + 5 = 25\n")
	fmt.Printf("   Final Result: %v\n", result)
}

func example2MultipleArgs() {
	fmt.Println("2. Multiple Arguments and Complex Logic")
	fmt.Println("   --------------------------------------")

	chain := flow.NewChain()

	chain.Add("step1", func() []int {
		return []int{10, 20, 30}
	})

	chain.Add("step2", func(a, b, c int) int {
		return a + b + c
	})

	chain.Add("step3", func(sum int) int {
		return sum * 2
	})

	chain.Add("step4", func(x int) int {
		return x - 5
	})

	err := chain.Run()
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
		return
	}

	result, err := chain.Value("step4")
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
		return
	}

	fmt.Printf("   Inputs: 10, 20, 30\n")
	fmt.Printf("   Sum: 10+20+30=60, multiply by 2, then subtract 5\n")
	fmt.Printf("   Final Result: %v\n", result)
}

func example3MultipleReturns() {
	fmt.Println("3. Multiple Return Values with Chaining")
	fmt.Println("   --------------------------------------")

	chain := flow.NewChain()

	chain.Add("step1", func() int {
		return 100
	})

	chain.Add("step2", func(a int) (int, string, float64) {
		return a / 2, "half", math.Sqrt(float64(a))
	})

	chain.Add("step3", func(result int, desc string, sqrt float64) int {
		fmt.Printf("   Intermediate: Result=%d, Desc=%s, Sqrt=%.2f\n", result, desc, sqrt)
		return result + 10
	})

	err := chain.Run()
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
		return
	}

	values, err := chain.Values("step2")
	if err != nil {
		fmt.Printf("   Error getting values: %v\n", err)
		return
	}
	fmt.Printf("   After step2: %v\n", values)

	result, err := chain.Value("step3")
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
		return
	}

	fmt.Printf("   Then 50 + 10 = 60\n")
	fmt.Printf("   Final Result: %v\n", result)
}

func example4NoArguments() {
	fmt.Println("4. No Initial Arguments with Custom Types")
	fmt.Println("   ----------------------------------------")

	chain := flow.NewChain()

	chain.Add("step1", func() string {
		return "Hello"
	})

	chain.Add("step2", func(s string) string {
		return s + " World"
	})

	chain.Add("step3", func(s string) string {
		return s + "!"
	})

	chain.Add("step4", func(s string) Person {
		return Person{Name: s, Age: 25}
	})

	err := chain.Run()
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
		return
	}

	result, err := chain.Value("step4")
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
		return
	}

	fmt.Printf("   Starting with no initial values\n")
	fmt.Printf("   Output sequence: \"Hello\" → \"Hello World\" → \"Hello World!\" → Person\n")
	fmt.Printf("   Final Result: %v\n", result)
}

func example5NoReturn() {
	fmt.Println("5. No Return Value with Side Effects")
	fmt.Println("   -----------------------------------")

	var counter int

	chain := flow.NewChain()

	chain.Add("step1", func() int {
		return 5
	})

	chain.Add("step2", func(n int) {
		counter += n * 10
	})

	err := chain.Run()
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
		return
	}

	fmt.Printf("   Input: 5\n")
	fmt.Printf("   Counter updated: 5 * 10 = 50\n")
	fmt.Printf("   Counter value: %d\n", counter)
}

func example6TypeConversion() {
	fmt.Println("6. Type Conversion Pipeline")
	fmt.Println("   -------------------------")

	chain := flow.NewChain()

	chain.Add("step1", func() int {
		return 123
	})

	chain.Add("step2", func(i int) string {
		return fmt.Sprintf("Number: %d", i)
	})

	chain.Add("step3", func(s string) int {
		return len(s)
	})

	chain.Add("step4", func(n int) float64 {
		return float64(n) * 2.5
	})

	chain.Add("step5", func(f float64) string {
		return fmt.Sprintf("Final: %.2f", f)
	})

	err := chain.Run()
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
		return
	}

	result, err := chain.Value("step5")
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
		return
	}

	fmt.Printf("   Input: 123 (int)\n")
	fmt.Printf("   Convert to string: \"Number: 123\"\n")
	fmt.Printf("   Get length of string: 11\n")
	fmt.Printf("   Multiply by 2.5: 27.5\n")
	fmt.Printf("   Format final string\n")
	fmt.Printf("   Final Result: %q\n", result)
}

func example7ErrorHandling() {
	fmt.Println("7. Error Handling in Chains")
	fmt.Println("   -------------------------")

	chain1 := flow.NewChain()

	chain1.Add("step1", func() string {
		return "not a number"
	})

	chain1.Add("step2", func(s string) int {
		return 42
	})

	err1 := chain1.Run()
	if err1 != nil {
		fmt.Printf("   Example A - Type conversion error: %v\n", err1)
	}

	fmt.Println()

	chain2 := flow.NewChain()

	chain2.Add("step1", func() int {
		return 42
	})

	chain2.Add("step2", func(x int) string {
		return fmt.Sprintf("Number: %d", x)
	})

	chain2.Add("step3", func(s string) int {
		return len(s)
	})

	chain2.Add("step4", func(n int) int {
		return n * 2
	})

	err2 := chain2.Run()
	if err2 != nil {
		fmt.Printf("   Example B - Error: %v\n", err2)
	} else {
		result2, err := chain2.Value("step4")
		if err != nil {
			fmt.Printf("   Example B - Error: %v\n", err)
		} else {
			fmt.Printf("   Example B - Result: %v\n", result2)
		}
	}
}
