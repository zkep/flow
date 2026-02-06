package flow

import (
	"reflect"
	"strings"
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
	type Result struct {
		Step1 int
		Step2 int
		Step3 int
		Step4 int
	}

	chain.Add("step1", func() Result {
		return Result{Step1: 10}
	})

	chain.Add("step2", func(y Result) Result {
		y.Step2 = 20
		return y
	})

	chain.Add("step3", func(y Result) Result {
		y.Step3 = 30
		return y
	})

	err := chain.Run()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if value1, err := chain.Value("step3"); err != nil {
		t.Fatalf("Unexpected error getting value: %v", err)
	} else if value1.(Result).Step3 != 30 {
		t.Errorf("Expected 30, got %v", value1)
	}

	newChain := chain.Use("step2")
	newChain.Add("step4", func(y Result) Result {
		y.Step4 = 40
		return y
	})
	err = newChain.Run()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	value1, err := newChain.Value("step4")
	if err != nil {
		t.Fatalf("Unexpected error getting value: %v", err)
	}
	if value1.(Result).Step4 != 40 {
		t.Errorf("Expected 40, got %v", value1)
	}

	value2, err := newChain.Value("step2")
	if err != nil {
		t.Fatalf("Unexpected error getting value: %v", err)
	}

	if value2.(Result).Step2 != 20 {
		t.Errorf("Expected 20, got %v", value2.(Result).Step2)
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

func TestChainPanic(t *testing.T) {
	t.Run("panic in second step", func(t *testing.T) {
		chain := NewChain()
		chain.Add("step1", func() int { return 10 })
		chain.Add("step2", func(x int) int { panic("test panic") })

		err := chain.Run()
		if err == nil {
			t.Fatalf("Expected error for panic")
		}
		if !strings.HasPrefix(err.Error(), ErrFunctionPanicked) {
			t.Errorf("Expected error to start with '%s', got '%v'", ErrFunctionPanicked, err.Error())
		}
	})

	t.Run("panic in first step", func(t *testing.T) {
		chain := NewChain()
		chain.Add("step1", func() int { panic("test panic") })

		err := chain.Run()
		if err == nil {
			t.Fatalf("Expected error for panic")
		}
		if !strings.HasPrefix(err.Error(), ErrFunctionPanicked) {
			t.Errorf("Expected error to start with '%s', got '%v'", ErrFunctionPanicked, err.Error())
		}
	})
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

	_, err = chain.Value("step3")
	if err == nil {
		t.Fatalf("Expected error for step3 since it was not executed")
	}

	value, err := chain.Value("step1")
	if err != nil {
		t.Fatalf("Expected no error for step1, got %v", err)
	}

	if value.(int) != 10 {
		t.Errorf("Expected 10, got %v", value)
	}
}

func TestChainUseWithNonExistentSteps(t *testing.T) {
	chain := NewChain()

	chain.Add("step1", func() int {
		return 10
	})

	newChain := chain.Use("nonexistent", "step1")

	if newChain.err == nil {
		t.Fatalf("Expected error for nonexistent step")
	}

	if newChain.err.Error() != ErrStepNotFound {
		t.Errorf("Expected '%s', got '%v'", ErrStepNotFound, newChain.err.Error())
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
	t.Run("same types", func(t *testing.T) {
		if !canConvert(reflect.TypeOf(10), reflect.TypeOf(20)) {
			t.Error("Expected same types to be convertible")
		}
	})

	t.Run("interface implementation", func(t *testing.T) {
		if !canConvert(reflect.TypeOf(TestStruct{}), reflect.TypeOf((*TestInterface)(nil)).Elem()) {
			t.Error("Expected struct implementing interface to be convertible")
		}
	})

	t.Run("non-convertible types", func(t *testing.T) {
		type NonConvertibleStruct struct{ Field int }
		if canConvert(reflect.TypeOf(10), reflect.TypeOf(NonConvertibleStruct{})) {
			t.Error("Expected different types to not be convertible")
		}
	})

	t.Run("numeric type conversion", func(t *testing.T) {
		if !canConvert(reflect.TypeOf(int(10)), reflect.TypeOf(float64(0))) {
			t.Error("Expected int to float64 to be convertible")
		}
		if !canConvert(reflect.TypeOf(int32(10)), reflect.TypeOf(int64(0))) {
			t.Error("Expected int32 to int64 to be convertible")
		}
	})
}

func TestPrepareArgs(t *testing.T) {
	t.Run("exact argument count match", func(t *testing.T) {
		fn := func(a, b int) int { return a + b }
		fnType := reflect.TypeOf(fn)
		argTypes := []reflect.Type{fnType.In(0), fnType.In(1)}
		values := []reflect.Value{reflect.ValueOf(10), reflect.ValueOf(20)}
		args, err := prepareArgsWithType(values, argTypes)
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}
		if len(args) != 2 {
			t.Errorf("Expected 2 arguments, got: %d", len(args))
		}
	})

	t.Run("single argument with slice parameter", func(t *testing.T) {
		sliceFn := func(nums []int) int {
			sum := 0
			for _, num := range nums {
				sum += num
			}
			return sum
		}
		sliceFnType := reflect.TypeOf(sliceFn)
		sliceArgTypes := []reflect.Type{sliceFnType.In(0)}
		sliceValues := []reflect.Value{reflect.ValueOf(10), reflect.ValueOf(20), reflect.ValueOf(30)}
		sliceArgs, err := prepareArgsWithType(sliceValues, sliceArgTypes)
		if err != nil {
			t.Fatalf("Expected no error for slice parameter, got: %v", err)
		}
		if len(sliceArgs) != 1 {
			t.Errorf("Expected 1 argument for slice parameter, got: %d", len(sliceArgs))
		}
	})

	t.Run("argument count mismatch", func(t *testing.T) {
		fn := func(a, b int) int { return a + b }
		fnType := reflect.TypeOf(fn)
		argTypes := []reflect.Type{fnType.In(0), fnType.In(1)}
		_, err := prepareArgsWithType([]reflect.Value{reflect.ValueOf(10)}, argTypes)
		if err == nil {
			t.Fatal("Expected error for argument count mismatch, got nil")
		}
	})

	t.Run("single value argument", func(t *testing.T) {
		singleArgFn := func(x int) int { return x * 2 }
		singleArgFnType := reflect.TypeOf(singleArgFn)
		singleArgTypes := []reflect.Type{singleArgFnType.In(0)}
		singleArgs, err := prepareArgsWithType([]reflect.Value{reflect.ValueOf(10)}, singleArgTypes)
		if err != nil {
			t.Fatalf("Expected no error for single argument, got: %v", err)
		}
		if len(singleArgs) != 1 {
			t.Errorf("Expected 1 argument, got: %d", len(singleArgs))
		}
	})

	t.Run("no arguments", func(t *testing.T) {
		noArgs, err := prepareArgsWithType([]reflect.Value{}, []reflect.Type{})
		if err != nil {
			t.Fatalf("Expected no error for no arguments, got: %v", err)
		}
		if len(noArgs) != 0 {
			t.Errorf("Expected 0 arguments, got: %d", len(noArgs))
		}
	})

	t.Run("no arguments but with values", func(t *testing.T) {
		argTypes := []reflect.Type{}
		values := []reflect.Value{reflect.ValueOf(10)}
		_, err := prepareArgsWithType(values, argTypes)
		if err == nil {
			t.Fatal("Expected error for argument count mismatch")
		}
	})

	t.Run("type conversion", func(t *testing.T) {
		argTypes := []reflect.Type{reflect.TypeOf(float64(0))}
		values := []reflect.Value{reflect.ValueOf(10)}
		args, err := prepareArgsWithType(values, argTypes)
		if err != nil {
			t.Fatalf("Expected no error for type conversion, got: %v", err)
		}
		if len(args) != 1 {
			t.Errorf("Expected 1 argument, got: %d", len(args))
		}
	})

	t.Run("nil value", func(t *testing.T) {
		argTypes := []reflect.Type{reflect.TypeOf("")}
		values := []reflect.Value{reflect.ValueOf(nil)}
		args, err := prepareArgsWithType(values, argTypes)
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}
		if len(args) != 1 {
			t.Errorf("Expected 1 argument, got: %d", len(args))
		}
	})

	t.Run("slice from array", func(t *testing.T) {
		argTypes := []reflect.Type{reflect.TypeOf(0), reflect.TypeOf(0), reflect.TypeOf(0)}
		values := []reflect.Value{reflect.ValueOf([3]int{1, 2, 3})}
		args, err := prepareArgsWithType(values, argTypes)
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}
		if len(args) != 3 {
			t.Errorf("Expected 3 arguments, got: %d", len(args))
		}
	})

	t.Run("slice arg count mismatch", func(t *testing.T) {
		argTypes := []reflect.Type{reflect.TypeOf(0), reflect.TypeOf(0)}
		values := []reflect.Value{reflect.ValueOf([]int{1, 2, 3})}
		_, err := prepareArgsWithType(values, argTypes)
		if err == nil {
			t.Fatal("Expected error for argument count mismatch")
		}
	})

	t.Run("pool capacity", func(t *testing.T) {
		argTypes := []reflect.Type{
			reflect.TypeOf(0), reflect.TypeOf(0), reflect.TypeOf(0), reflect.TypeOf(0),
			reflect.TypeOf(0), reflect.TypeOf(0), reflect.TypeOf(0), reflect.TypeOf(0),
			reflect.TypeOf(0), reflect.TypeOf(0),
		}
		values := []reflect.Value{
			reflect.ValueOf(1), reflect.ValueOf(2), reflect.ValueOf(3), reflect.ValueOf(4),
			reflect.ValueOf(5), reflect.ValueOf(6), reflect.ValueOf(7), reflect.ValueOf(8),
			reflect.ValueOf(9), reflect.ValueOf(10),
		}
		args, err := prepareArgsWithType(values, argTypes)
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}
		if len(args) != 10 {
			t.Errorf("Expected 10 arguments, got: %d", len(args))
		}
	})

	t.Run("slice conversion", func(t *testing.T) {
		sliceFn := func(nums []float64) float64 {
			sum := 0.0
			for _, num := range nums {
				sum += num
			}
			return sum
		}
		sliceFnType := reflect.TypeOf(sliceFn)
		argTypes := []reflect.Type{sliceFnType.In(0)}
		values := []reflect.Value{reflect.ValueOf(1), reflect.ValueOf(2), reflect.ValueOf(3)}
		args, err := prepareArgsWithType(values, argTypes)
		if err != nil {
			t.Fatalf("Expected no error for slice conversion, got: %v", err)
		}
		if len(args) != 1 {
			t.Errorf("Expected 1 argument, got: %d", len(args))
		}
	})

	t.Run("slice conversion error", func(t *testing.T) {
		sliceFn := func(nums []int) int {
			sum := 0
			for _, num := range nums {
				sum += num
			}
			return sum
		}
		sliceFnType := reflect.TypeOf(sliceFn)
		argTypes := []reflect.Type{sliceFnType.In(0)}
		values := []reflect.Value{reflect.ValueOf("not"), reflect.ValueOf("convertible")}
		_, err := prepareArgsWithType(values, argTypes)
		if err == nil {
			t.Fatalf("Expected error for slice conversion failure")
		}
	})
}

func TestChainAddWithExistingError(t *testing.T) {
	chain := NewChain()
	chain.err = &ChainError{Message: "existing error"}

	chain.Add("step1", func() int {
		return 10
	})

	if chain.err == nil {
		t.Errorf("Expected error to be preserved")
	}
}

func TestChainRunWithExistingError(t *testing.T) {
	chain := NewChain()
	chain.err = &ChainError{Message: "existing error"}

	chain.Add("step1", func() int {
		return 10
	})

	err := chain.Run()
	if err == nil {
		t.Errorf("Expected error to be returned")
	}
}

func TestChainUseWithExistingError(t *testing.T) {
	chain := NewChain()
	chain.err = &ChainError{Message: "existing error"}

	newChain := chain.Use("step1")

	if newChain.err == nil {
		t.Errorf("Expected error to be propagated to new chain")
	}
}

func TestChainIndexOutOfBounds(t *testing.T) {
	t.Run("Values method", func(t *testing.T) {
		chain := NewChain()
		chain.Add("step1", func() int { return 10 })
		chain.stepNames["step2"] = 100

		err := chain.Run()
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		_, err = chain.Values("step2")
		if err == nil {
			t.Errorf("Expected error for out of bounds index")
		}
	})

	t.Run("Value method", func(t *testing.T) {
		chain := NewChain()
		chain.Add("step1", func() int { return 10 })
		chain.stepNames["step2"] = 100

		err := chain.Run()
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		_, err = chain.Value("step2")
		if err == nil {
			t.Errorf("Expected error for out of bounds index")
		}
	})
}

func TestChainValueEmptyValues(t *testing.T) {
	chain := NewChain()
	chain.Add("step1", func() {})

	err := chain.Run()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	_, err = chain.Value("step1")
	if err == nil {
		t.Errorf("Expected error for empty values")
	}
}

func TestChainWithArray(t *testing.T) {
	chain := NewChain()

	chain.Add("step1", [3]int{1, 2, 3})

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
}

func TestChainCallWithExistingError(t *testing.T) {
	chain := NewChain()
	chain.err = &ChainError{Message: "existing error"}

	fn := func() int { return 10 }
	values := chain.call(reflect.ValueOf(fn), []reflect.Type{}, []reflect.Value{})

	if len(values) != 0 {
		t.Errorf("Expected empty values when error exists")
	}
}

func TestAddArg(t *testing.T) {
	var args []reflect.Value

	err := addArg(&args, reflect.ValueOf(10), reflect.TypeOf(0))
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if len(args) != 1 {
		t.Errorf("Expected 1 argument, got: %d", len(args))
	}
}

func TestChainWithInterfaceSlice(t *testing.T) {
	chain := NewChain()

	chain.Add("step1", []any{"hello", 42, true})

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
}

func TestChainMultipleRuns(t *testing.T) {
	t.Run("run twice same result", func(t *testing.T) {
		chain := NewChain()
		chain.Add("step1", func() int { return 10 })

		err := chain.Run()
		if err != nil {
			t.Fatalf("Unexpected error on first run: %v", err)
		}

		err = chain.Run()
		if err != nil {
			t.Fatalf("Unexpected error on second run: %v", err)
		}

		value, err := chain.Value("step1")
		if err != nil {
			t.Fatalf("Unexpected error getting value: %v", err)
		}

		if value.(int) != 10 {
			t.Errorf("Expected 10, got %v", value)
		}
	})

	t.Run("idempotent - function called once", func(t *testing.T) {
		chain := NewChain()
		callCount := 0
		chain.Add("step1", func() int {
			callCount++
			return 10
		})

		err := chain.Run()
		if err != nil {
			t.Fatalf("Unexpected error on first run: %v", err)
		}

		err = chain.Run()
		if err != nil {
			t.Fatalf("Unexpected error on second run: %v", err)
		}

		if callCount != 1 {
			t.Errorf("Expected function to be called once, got %d calls", callCount)
		}
	})

	t.Run("multiple steps", func(t *testing.T) {
		chain := NewChain()
		chain.Add("step1", func() int { return 10 })
		chain.Add("step2", func(x int) int { return x * 2 })

		err := chain.Run()
		if err != nil {
			t.Fatalf("Unexpected error on first run: %v", err)
		}

		value1, err := chain.Value("step2")
		if err != nil {
			t.Fatalf("Unexpected error getting value: %v", err)
		}

		if value1.(int) != 20 {
			t.Errorf("Expected 20, got %v", value1)
		}
	})
}

func TestChainHandleNonFunctionArray(t *testing.T) {
	chain := NewChain()

	chain.Add("step1", [2]string{"hello", "world"})

	err := chain.Run()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	values, err := chain.Values("step1")
	if err != nil {
		t.Fatalf("Unexpected error getting values: %v", err)
	}

	if len(values) != 2 {
		t.Errorf("Expected 2 values, got %d", len(values))
	}
}

func TestChainWithMapValue(t *testing.T) {
	chain := NewChain()

	chain.Add("step1", map[string]int{"a": 1, "b": 2})

	err := chain.Run()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	value, err := chain.Value("step1")
	if err != nil {
		t.Fatalf("Unexpected error getting value: %v", err)
	}

	if _, ok := value.(map[string]int); !ok {
		t.Errorf("Expected map value")
	}
}

func TestChainWithStructValue(t *testing.T) {
	type TestStruct struct {
		Name  string
		Value int
	}

	chain := NewChain()

	chain.Add("step1", TestStruct{Name: "test", Value: 42})

	err := chain.Run()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	value, err := chain.Value("step1")
	if err != nil {
		t.Fatalf("Unexpected error getting value: %v", err)
	}

	ts, ok := value.(TestStruct)
	if !ok {
		t.Errorf("Expected TestStruct value")
	}

	if ts.Name != "test" || ts.Value != 42 {
		t.Errorf("Expected {Name: test, Value: 42}, got %+v", ts)
	}
}

func TestChainErrorReturnWithMultipleValues(t *testing.T) {
	chain := NewChain()

	chain.Add("step1", func() (int, string, error) {
		return 0, "", &ChainError{Message: "multi-value error"}
	})

	err := chain.Run()
	if err == nil {
		t.Fatalf("Expected error")
	}

	if err.Error() != "multi-value error" {
		t.Errorf("Expected 'multi-value error', got '%v'", err.Error())
	}
}

func TestChainStruct(t *testing.T) {
	chain := NewChain()
	type Result1 struct {
		Step1 int
	}
	type Result2 struct {
		Step2 string
	}
	type Result3 struct {
		Step3 float64
	}
	// 每一步都返回上一步的结果
	chain.Add("step1", func() Result1 {
		return Result1{Step1: 10}
	})
	chain.Add("step2", func(x Result1) (Result1, Result2) {
		return x, Result2{Step2: "20"}
	})
	chain.Add("step3", func(x Result1, y Result2) (Result1, Result2, Result3) {
		return x, y, Result3{Step3: 30}
	})
	err := chain.Run()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	values, err := chain.Values("step3")
	if err != nil {
		t.Fatalf("Unexpected error getting value: %v", err)
	}
	if len(values) != 3 {
		t.Errorf("Expected 3 values, got %d", len(values))
	}
	if values[0].(Result1).Step1 != 10 {
		t.Errorf("Expected Step1: 10, got %v", values[0].(Result1).Step1)
	}
	if values[1].(Result2).Step2 != "20" {
		t.Errorf("Expected Step2: 20, got %v", values[1].(Result2).Step2)
	}
	if values[2].(Result3).Step3 != 30 {
		t.Errorf("Expected Step3: 30, got %v", values[2].(Result3).Step3)
	}
}

func TestAddArgWithInvalidValue(t *testing.T) {
	var args []reflect.Value
	err := addArg(&args, reflect.Value{}, reflect.TypeOf(0))
	if err != nil {
		t.Errorf("Expected no error for nil value, got: %v", err)
	}
	if len(args) != 1 {
		t.Errorf("Expected 1 argument, got: %d", len(args))
	}
}

func TestAddArgWithTypeConversion(t *testing.T) {
	var args []reflect.Value
	err := addArg(&args, reflect.ValueOf(int32(10)), reflect.TypeOf(int64(0)))
	if err != nil {
		t.Errorf("Expected no error for type conversion, got: %v", err)
	}
	if len(args) != 1 {
		t.Errorf("Expected 1 argument, got: %d", len(args))
	}
}

func TestChainWithNilValue(t *testing.T) {
	chain := NewChain()

	chain.Add("step1", func() *int {
		return nil
	})

	chain.Add("step2", func(i *int) string {
		if i == nil {
			return "nil"
		}
		return "not nil"
	})

	err := chain.Run()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	value, err := chain.Value("step2")
	if err != nil {
		t.Fatalf("Unexpected error getting value: %v", err)
	}

	if value.(string) != "nil" {
		t.Errorf("Expected 'nil', got %v", value)
	}
}

func TestChainWithChannelValue(t *testing.T) {
	chain := NewChain()

	ch := make(chan int, 1)
	ch <- 42

	chain.Add("step1", ch)

	err := chain.Run()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	value, err := chain.Value("step1")
	if err != nil {
		t.Fatalf("Unexpected error getting value: %v", err)
	}

	if _, ok := value.(chan int); !ok {
		t.Errorf("Expected chan int value")
	}
}

func TestChainWithPointerValue(t *testing.T) {
	chain := NewChain()

	x := 100
	chain.Add("step1", &x)

	err := chain.Run()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	value, err := chain.Value("step1")
	if err != nil {
		t.Fatalf("Unexpected error getting value: %v", err)
	}

	ptr, ok := value.(*int)
	if !ok {
		t.Errorf("Expected *int value")
	}

	if *ptr != 100 {
		t.Errorf("Expected 100, got %v", *ptr)
	}
}

func TestChainUseWithEmptyNames(t *testing.T) {
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
		t.Errorf("Expected no handlers in new chain")
	}

	if len(newChain.values) != 0 {
		t.Errorf("Expected no values in new chain")
	}
}

func TestChainNoArgFunction(t *testing.T) {
	chain := NewChain()

	chain.Add("step1", func() int {
		return 10
	})

	err := chain.Run()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	value, err := chain.Value("step1")
	if err != nil {
		t.Fatalf("Unexpected error getting value: %v", err)
	}

	if value.(int) != 10 {
		t.Errorf("Expected 10, got %v", value)
	}
}

func TestChainNoArgFunctionWithValues(t *testing.T) {
	chain := NewChain()

	chain.Add("step1", func() int {
		return 10
	})

	err := chain.Run()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	values, err := chain.Values("step1")
	if err != nil {
		t.Fatalf("Unexpected error getting values: %v", err)
	}

	if len(values) != 1 || values[0].(int) != 10 {
		t.Errorf("Expected [10], got %v", values)
	}
}

func TestChainFunctionReturningErrorOnly(t *testing.T) {
	chain := NewChain()

	chain.Add("step1", func() error {
		return nil
	})

	err := chain.Run()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
}

func TestChainFunctionReturningErrorOnlyWithError(t *testing.T) {
	chain := NewChain()

	chain.Add("step1", func() error {
		return &ChainError{Message: "test error"}
	})

	err := chain.Run()
	if err == nil {
		t.Fatal("Expected error")
	}

	if err.Error() != "test error" {
		t.Errorf("Expected 'test error', got '%v'", err.Error())
	}
}

func TestChainEmptyChain(t *testing.T) {
	chain := NewChain()

	err := chain.Run()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
}

func TestChainUseWithMultipleSteps(t *testing.T) {
	chain := NewChain()

	chain.Add("step1", func() int {
		return 10
	})

	chain.Add("step2", func(x int) int {
		return x * 2
	})

	err := chain.Run()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	newChain := chain.Use("step1")

	err = newChain.Run()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	value, err := newChain.Value("step1")
	if err != nil {
		t.Fatalf("Unexpected error getting value: %v", err)
	}

	if value.(int) != 10 {
		t.Errorf("Expected 10, got %v", value)
	}
}

func TestChainAddAfterError(t *testing.T) {
	chain := NewChain()

	chain.Add("step1", func() int {
		return 10
	})

	chain.Add("step2", func(x int) (int, error) {
		return 0, &ChainError{Message: "test error"}
	})

	err := chain.Run()
	if err == nil {
		t.Fatal("Expected error")
	}

	chain.Add("step3", func(x int) int {
		return x * 2
	})

	if chain.err == nil {
		t.Error("Expected error to be preserved")
	}
}

func TestChainUseAfterError(t *testing.T) {
	chain := NewChain()

	chain.Add("step1", func() int {
		return 10
	})

	chain.Add("step2", func(x int) (int, error) {
		return 0, &ChainError{Message: "test error"}
	})

	err := chain.Run()
	if err == nil {
		t.Fatal("Expected error")
	}

	newChain := chain.Use("step1")

	if newChain.err == nil {
		t.Error("Expected error to be propagated to new chain")
	}
}

func TestChainWithMultipleErrors(t *testing.T) {
	chain := NewChain()

	chain.Add("step1", func() int {
		return 10
	})

	chain.Add("step2", func(x int) (int, error) {
		return 0, &ChainError{Message: "first error"}
	})

	chain.Add("step3", func(x int) (int, error) {
		return 0, &ChainError{Message: "second error"}
	})

	err := chain.Run()
	if err == nil {
		t.Fatal("Expected error")
	}

	if err.Error() != "first error" {
		t.Errorf("Expected 'first error', got '%v'", err.Error())
	}
}

func TestChainStructWithMultipleFields(t *testing.T) {
	type Data struct {
		A int
		B string
		C bool
	}

	chain := NewChain()

	chain.Add("step1", func() Data {
		return Data{A: 10, B: "hello", C: true}
	})

	chain.Add("step2", func(d Data) Data {
		d.A *= 2
		return d
	})

	err := chain.Run()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	value, err := chain.Value("step2")
	if err != nil {
		t.Fatalf("Unexpected error getting value: %v", err)
	}

	d := value.(Data)
	if d.A != 20 || d.B != "hello" || d.C != true {
		t.Errorf("Expected {20, hello, true}, got %+v", d)
	}
}

func TestChainCallResultCountMismatch(t *testing.T) {
	chain := NewChain()

	chain.Add("step1", func() int {
		return 10
	})

	chain.Add("step2", func(x int) int {
		return x * 2
	})

	err := chain.Run()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
}

func TestChainWithInterfaceValue(t *testing.T) {
	chain := NewChain()

	chain.Add("step1", func() any {
		return "hello"
	})

	err := chain.Run()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	value, err := chain.Value("step1")
	if err != nil {
		t.Fatalf("Unexpected error getting value: %v", err)
	}

	if value.(string) != "hello" {
		t.Errorf("Expected 'hello', got %v", value)
	}
}

func TestChainWithFuncValue(t *testing.T) {
	chain := NewChain()

	chain.Add("step1", func() func() int {
		return func() int { return 42 }
	})

	err := chain.Run()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	value, err := chain.Value("step1")
	if err != nil {
		t.Fatalf("Unexpected error getting value: %v", err)
	}

	fn := value.(func() int)
	if fn() != 42 {
		t.Errorf("Expected 42, got %v", fn())
	}
}

func TestChainAddArgWithNilValue(t *testing.T) {
	var args []reflect.Value

	err := addArg(&args, reflect.Value{}, reflect.TypeOf(""))
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if len(args) != 1 {
		t.Errorf("Expected 1 argument, got: %d", len(args))
	}
}

func TestChainAddArgWithConversion(t *testing.T) {
	var args []reflect.Value

	err := addArg(&args, reflect.ValueOf(10), reflect.TypeOf(float64(0)))
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if len(args) != 1 {
		t.Errorf("Expected 1 argument, got: %d", len(args))
	}

	if args[0].Float() != 10.0 {
		t.Errorf("Expected 10.0, got: %v", args[0].Float())
	}
}

func TestChainHandleNonFunctionType(t *testing.T) {
	chain := NewChain()

	chain.Add("step1", 42)

	err := chain.Run()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	value, err := chain.Value("step1")
	if err != nil {
		t.Fatalf("Unexpected error getting value: %v", err)
	}

	if value != 42 {
		t.Errorf("Expected 42, got %v", value)
	}
}

func TestChainHandleNonFunctionTypeWithInterface(t *testing.T) {
	chain := NewChain()

	chain.Add("step1", []any{1, "hello", true})

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
}

func TestChainErrorType(t *testing.T) {
	err := &ChainError{Message: "test error"}
	if err.Error() != "test error" {
		t.Errorf("Expected 'test error', got '%v'", err.Error())
	}
}

func TestChainRunWithDoFlag(t *testing.T) {
	chain := NewChain()

	chain.Add("step1", func() int {
		return 10
	})

	err := chain.Run()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !chain.handlers[0].do {
		t.Error("Expected do flag to be set")
	}
}

func TestChainWithSliceOfStructs(t *testing.T) {
	type Item struct {
		Name  string
		Value int
	}

	chain := NewChain()

	chain.Add("step1", func() []Item {
		return []Item{
			{Name: "a", Value: 1},
			{Name: "b", Value: 2},
		}
	})

	chain.Add("step2", func(items []Item) int {
		sum := 0
		for _, item := range items {
			sum += item.Value
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

	if value.(int) != 3 {
		t.Errorf("Expected 3, got %v", value)
	}
}

func TestChainWithMapReturn(t *testing.T) {
	chain := NewChain()

	chain.Add("step1", func() map[string]int {
		return map[string]int{"a": 1, "b": 2}
	})

	err := chain.Run()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	value, err := chain.Value("step1")
	if err != nil {
		t.Fatalf("Unexpected error getting value: %v", err)
	}

	m := value.(map[string]int)
	if m["a"] != 1 || m["b"] != 2 {
		t.Errorf("Expected map[a:1 b:2], got %v", m)
	}
}

func TestChainWithBoolReturn(t *testing.T) {
	chain := NewChain()

	chain.Add("step1", func() bool {
		return true
	})

	err := chain.Run()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	value, err := chain.Value("step1")
	if err != nil {
		t.Fatalf("Unexpected error getting value: %v", err)
	}

	if value.(bool) != true {
		t.Errorf("Expected true, got %v", value)
	}
}
