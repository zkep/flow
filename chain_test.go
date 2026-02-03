package flow

import (
	"reflect"
	"testing"
)

func TestChainAddAndRun(t *testing.T) {
	chain := NewChain()

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
		t.Fatalf("Unexpected error: %v", err)
	}

	value, err := chain.Value("step3")
	if err != nil {
		t.Fatalf("Unexpected error getting value: %v", err)
	}

	if value.(int) != 25 {
		t.Errorf("Expected 25, got %v", value)
	}
}

func TestChainMultipleArgs(t *testing.T) {
	chain := NewChain()

	chain.Add("step1", func() []int {
		return []int{10, 20, 30}
	})

	chain.Add("step2", func(a, b, c int) int {
		return a + b + c
	})

	chain.Add("step3", func(sum int) int {
		return sum * 2
	})

	err := chain.Run()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	value, err := chain.Value("step3")
	if err != nil {
		t.Fatalf("Unexpected error getting value: %v", err)
	}

	if value.(int) != 120 {
		t.Errorf("Expected 120, got %v", value)
	}
}

func TestChainMultipleReturns(t *testing.T) {
	chain := NewChain()

	chain.Add("step1", func() int {
		return 100
	})

	chain.Add("step2", func(a int) (int, string) {
		return a / 2, "half"
	})

	chain.Add("step3", func(result int) int {
		return result + 10
	})

	err := chain.Run()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	value, err := chain.Value("step3")
	if err != nil {
		t.Fatalf("Unexpected error getting value: %v", err)
	}

	if value.(int) != 60 {
		t.Errorf("Expected 60, got %v", value)
	}
}

func TestChainUse(t *testing.T) {
	chain := NewChain()

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
		t.Fatalf("Unexpected error: %v", err)
	}

	newChain := chain.Use("step1", "step2")

	value1, err := newChain.Value("step1")
	if err != nil {
		t.Fatalf("Unexpected error getting value: %v", err)
	}

	if value1.(int) != 10 {
		t.Errorf("Expected 10, got %v", value1)
	}

	value2, err := newChain.Value("step2")
	if err != nil {
		t.Fatalf("Unexpected error getting value: %v", err)
	}

	if value2.(int) != 20 {
		t.Errorf("Expected 20, got %v", value2)
	}
}

func TestChainValues(t *testing.T) {
	chain := NewChain()

	chain.Add("step1", func() (int, string, bool) {
		return 10, "test", true
	})

	err := chain.Run()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	values, err := chain.Values("step1")
	if err != nil {
		t.Fatalf("Unexpected error getting values: %v", err)
	}

	if len(values) != 3 {
		t.Errorf("Expected 3 values, got %d", len(values))
	}

	if values[0] != 10 || values[1] != "test" || values[2] != true {
		t.Errorf("Expected [10, 'test', true], got %v", values)
	}
}

func TestChainError(t *testing.T) {
	chain := NewChain()

	chain.Add("step1", func() int {
		return 10
	})

	chain.Add("step2", func(x int) (int, error) {
		return 0, &ChainError{Message: "test error"}
	})

	err := chain.Run()
	if err == nil {
		t.Fatalf("Expected error, got nil")
	}

	if err.Error() != "test error" {
		t.Errorf("Expected 'test error', got %v", err.Error())
	}
}

func TestChainStepNotFound(t *testing.T) {
	chain := NewChain()

	chain.Add("step1", func() int {
		return 10
	})

	err := chain.Run()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	_, err = chain.Value("nonexistent")
	if err == nil {
		t.Fatalf("Expected error for non-existent step")
	}

	if err.Error() != ErrStepNotFound {
		t.Errorf("Expected '%s', got '%v'", ErrStepNotFound, err.Error())
	}
}

func TestChainFunctionPanic(t *testing.T) {
	chain := NewChain()

	chain.Add("step1", func() int {
		return 10
	})

	chain.Add("step2", func(x int) int {
		panic("test panic")
	})

	err := chain.Run()
	if err == nil {
		t.Fatalf("Expected error for panic")
	}

	if err.Error() != ErrFunctionPanicked {
		t.Errorf("Expected '%s', got '%v'", ErrFunctionPanicked, err.Error())
	}
}

func TestChainArgCountMismatch(t *testing.T) {
	chain := NewChain()

	chain.Add("step1", func() []int {
		return []int{10, 20}
	})

	chain.Add("step2", func(a, b, c int) int {
		return a + b + c
	})

	err := chain.Run()
	if err == nil {
		t.Fatalf("Expected error for argument count mismatch")
	}

	if err.Error() != ErrArgCountMismatch {
		t.Errorf("Expected '%s', got '%v'", ErrArgCountMismatch, err.Error())
	}
}

func TestChainArgTypeMismatch(t *testing.T) {
	chain := NewChain()

	chain.Add("step1", func() string {
		return "not a number"
	})

	chain.Add("step2", func(x int) int {
		return x * 2
	})

	err := chain.Run()
	if err == nil {
		t.Fatalf("Expected error for argument type mismatch")
	}

	if err.Error() != ErrArgTypeMismatch {
		t.Errorf("Expected '%s', got '%v'", ErrArgTypeMismatch, err.Error())
	}
}

func TestChainWithNonFunction(t *testing.T) {
	chain := NewChain()

	chain.Add("step1", "not a function")

	err := chain.Run()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	value, err := chain.Value("step1")
	if err != nil {
		t.Fatalf("Unexpected error getting value: %v", err)
	}

	if value != "not a function" {
		t.Errorf("Expected 'not a function', got %v", value)
	}
}

func TestChainWithSlice(t *testing.T) {
	chain := NewChain()

	chain.Add("step1", []int{1, 2, 3, 4, 5})

	err := chain.Run()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	values, err := chain.Values("step1")
	if err != nil {
		t.Fatalf("Unexpected error getting values: %v", err)
	}

	if len(values) != 5 {
		t.Errorf("Expected 5 values, got %d", len(values))
	}

	expected := []any{1, 2, 3, 4, 5}
	for i, v := range values {
		if v != expected[i] {
			t.Errorf("Expected %v at index %d, got %v", expected[i], i, v)
		}
	}
}

func TestChainWithSliceParameter(t *testing.T) {
	chain := NewChain()

	chain.Add("step1", func() []int {
		return []int{1, 2, 3, 4, 5}
	})

	chain.Add("step2", func(nums []int) int {
		sum := 0
		for _, n := range nums {
			sum += n
		}
		return sum
	})

	err := chain.Run()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	value, err := chain.Value("step2")
	if err != nil {
		t.Fatalf("Unexpected error getting value: %v", err)
	}

	if value.(int) != 15 {
		t.Errorf("Expected 15, got %v", value)
	}
}

func TestChainErrorPropagation(t *testing.T) {
	chain := NewChain()

	chain.Add("step1", func() int {
		return 10
	})

	chain.Add("step2", func(x int) (int, error) {
		return 0, &ChainError{Message: "first error"}
	})

	chain.Add("step3", func(y int) int {
		return y + 5
	})

	err := chain.Run()
	if err == nil {
		t.Fatalf("Expected error")
	}

	if err.Error() != "first error" {
		t.Errorf("Expected 'first error', got '%v'", err.Error())
	}

	value, err := chain.Value("step3")
	if err != nil {
		t.Fatalf("Expected no error for step3, got %v", err)
	}

	if value.(int) != 0 {
		t.Errorf("Expected 0, got %v", value)
	}
}

func TestChainUseWithNonExistentSteps(t *testing.T) {
	chain := NewChain()

	chain.Add("step1", func() int {
		return 10
	})

	newChain := chain.Use("nonexistent", "step1")

	err := newChain.Run()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
}

func TestChainMultipleSteps(t *testing.T) {
	chain := NewChain()

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

	err := chain.Run()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	value, err := chain.Value("step4")
	if err != nil {
		t.Fatalf("Unexpected error getting value: %v", err)
	}

	if value.(int) != 22 {
		t.Errorf("Expected 22, got %v", value)
	}
}

func TestChainUseWithEdgeCases(t *testing.T) {
	chain := NewChain()

	chain.Add("step1", func() int {
		return 10
	})

	err := chain.Run()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	newChain := chain.Use()

	if len(newChain.handlers) != 0 {
		t.Errorf("Expected no handlers in empty use")
	}
}

func TestChainWithPanicRecovery(t *testing.T) {
	chain := NewChain()

	chain.Add("step1", func() int {
		panic("test panic")
	})

	err := chain.Run()
	if err == nil {
		t.Fatalf("Expected error for panic")
	}

	if err.Error() != ErrFunctionPanicked {
		t.Errorf("Expected '%s', got '%v'", ErrFunctionPanicked, err.Error())
	}
}

func TestChainErrorMethod(t *testing.T) {
	chain := NewChain()

	// Test Error() method on chain with no error
	err := chain.Error()
	if err != nil {
		t.Errorf("Expected nil error, got: %v", err)
	}

	// Test Error() method on chain with error (using argument count mismatch)
	chain2 := NewChain()
	chain2.Add("step1", func() int {
		return 10
	})
	// This function expects 2 arguments but will only get 1
	chain2.Add("step2", func(a, b int) int {
		return a + b
	})
	chain2.Run() // Run to trigger error
	err = chain2.Error()
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
}

// TestInterface is a test interface for canConvert function
type TestInterface interface {
	Test()
}

// TestStruct is a test struct implementing TestInterface
type TestStruct struct{}

// Test implements TestInterface for TestStruct
func (t TestStruct) Test() {}

func TestCanConvert(t *testing.T) {
	// Test case 1: Same type
	if !canConvert(reflect.TypeOf(10), reflect.TypeOf(20)) {
		t.Error("Expected same types to be convertible")
	}

	// Test case 2: Assignable types (interface implementation)
	if !canConvert(reflect.TypeOf(TestStruct{}), reflect.TypeOf((*TestInterface)(nil)).Elem()) {
		t.Error("Expected struct implementing interface to be convertible")
	}

	// Test case 3: Non-convertible types
	if canConvert(reflect.TypeOf(10), reflect.TypeOf("string")) {
		t.Error("Expected different types to not be convertible")
	}
}

func TestPrepareArgs(t *testing.T) {
	// Test case 1: Exact argument count match
	fn := func(a, b int) int {
		return a + b
	}
	fnType := reflect.TypeOf(fn)
	args, err := prepareArgs([]any{10, 20}, fnType)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if len(args) != 2 {
		t.Errorf("Expected 2 arguments, got: %d", len(args))
	}

	// Test case 2: Single argument with slice parameter
	sliceFn := func(nums []int) int {
		sum := 0
		for _, num := range nums {
			sum += num
		}
		return sum
	}
	sliceFnType := reflect.TypeOf(sliceFn)
	sliceArgs, err := prepareArgs([]any{10, 20, 30}, sliceFnType)
	if err != nil {
		t.Fatalf("Expected no error for slice parameter, got: %v", err)
	}
	if len(sliceArgs) != 1 {
		t.Errorf("Expected 1 argument for slice parameter, got: %d", len(sliceArgs))
	}

	// Test case 3: Argument count mismatch
	_, err = prepareArgs([]any{10}, fnType)
	if err == nil {
		t.Fatal("Expected error for argument count mismatch, got nil")
	}

	// Test case 4: Single value argument
	singleArgFn := func(x int) int {
		return x * 2
	}
	singleArgFnType := reflect.TypeOf(singleArgFn)
	singleArgs, err := prepareArgs([]any{10}, singleArgFnType)
	if err != nil {
		t.Fatalf("Expected no error for single argument, got: %v", err)
	}
	if len(singleArgs) != 1 {
		t.Errorf("Expected 1 argument, got: %d", len(singleArgs))
	}
}
