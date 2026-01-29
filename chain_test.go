package flow

import (
	"strconv"
	"testing"
)

type Person struct {
	Name string
	Age  int
}

type TestError struct {
	Message string
}

func (e *TestError) Error() string {
	return e.Message
}

func TestChain(t *testing.T) {
	result := NewChain(10).Call(func(a int) int {
		return a + 5
	}).Call(func(b int) int {
		return b * 2
	}).Value()

	if result != 30 {
		t.Errorf("Expected 30, got %v", result)
	}

	strResult := NewChain("hello").Call(func(s string) string {
		return s + " world"
	}).Call(func(s string) string {
		return s + "!"
	}).Value()

	if strResult != "hello world!" {
		t.Errorf("Expected 'hello world!', got %v", strResult)
	}

	typeResult := NewChain(123).Call(func(i int) string {
		return "number: " + string(rune(i))
	}).Call(func(s string) int {
		return len(s)
	}).Value()

	if typeResult != 9 {
		t.Errorf("Expected 9, got %v", typeResult)
	}

	noArgResult := NewChain().Call(func() string {
		return "initial"
	}).Call(func(s string) string {
		return s + " value"
	}).Value()

	if noArgResult != "initial value" {
		t.Errorf("Expected 'initial value', got %v", noArgResult)
	}

	noReturnResult := NewChain("test").Call(func(s string) {
	}).Value()

	if noReturnResult != nil {
		t.Errorf("Expected nil, got %v", noReturnResult)
	}

	multiArgResult := NewChain([]int{10, 20, 30}).Call(func(a, b, c int) int {
		return a + b + c
	}).Call(func(sum int) int {
		return sum * 2
	}).Value()

	if multiArgResult != 120 {
		t.Errorf("Expected 120, got %v", multiArgResult)
	}

	multiReturnResult := NewChain(100).Call(func(a int) (int, string) {
		return a / 2, "half"
	}).Call(func(result int) int {
		return result + 10
	}).Value()

	if multiReturnResult != 60 {
		t.Errorf("Expected 60, got %v", multiReturnResult)
	}

	mixedResult := NewChain([]int{5, 10}).Call(func(a, b int) (int, int) {
		return a + b, a * b
	}).Call(func(sum int) int {
		return sum * 2
	}).Value()

	if mixedResult != 30 {
		t.Errorf("Expected 30, got %v", mixedResult)
	}

	nonFunctionChain := NewChain(10).Call("not a function")
	if nonFunctionChain.Error() != nil {
		t.Errorf("Expected no error for non-function argument, got %v", nonFunctionChain.Error())
	}
	if nonFunctionChain.Value() != "not a function" {
		t.Errorf("Expected 'not a function', got %v", nonFunctionChain.Value())
	}

	sliceChain := NewChain(10).Call([]int{1, 2, 3})
	if sliceChain.Error() != nil {
		t.Errorf("Expected no error for slice argument, got %v", sliceChain.Error())
	}
	sliceValues := sliceChain.Values()
	if len(sliceValues) != 3 || sliceValues[0] != 1 || sliceValues[1] != 2 || sliceValues[2] != 3 {
		t.Errorf("Expected [1, 2, 3], got %v", sliceValues)
	}

	argCountError := NewChain([]int{10, 20}).Call(func(a, b, c int) int {
		return a + b + c
	})
	if argCountError.Error() == nil {
		t.Errorf("Expected error for argument count mismatch")
	}

	argTypeError := NewChain(10).Call(func(s string) string {
		return s
	})
	if argTypeError.Error() == nil {
		t.Errorf("Expected error for argument type mismatch")
	}

	errorPropagation := NewChain(10).Call("not a function").Call(func(a int) int {
		return a + 1
	})
	if errorPropagation.Error() == nil {
		t.Errorf("Expected error propagation")
	}

	errorReturn := NewChain(10).Call(func(a int) (int, error) {
		return 0, &TestError{Message: "function error"}
	}).Call(func(a int) int {
		return a + 1
	})
	if errorReturn.Error() == nil {
		t.Errorf("Expected error from function return")
	}

	person := Person{Name: "Alice", Age: 30}
	structResult := NewChain(person).Call(func(p Person) Person {
		p.Age++
		return p
	}).Call(func(p Person) string {
		return p.Name + " is " + string(rune(p.Age)) + " years old"
	}).Value()

	if structResult == nil {
		t.Errorf("Expected struct result, got nil")
	}

	personPtr := &Person{Name: "Bob", Age: 25}
	structPtrResult := NewChain(personPtr).Call(func(p *Person) *Person {
		p.Age++
		return p
	}).Call(func(p *Person) string {
		return p.Name + " is " + string(rune(p.Age)) + " years old"
	}).Value()

	if structPtrResult == nil {
		t.Errorf("Expected struct pointer result, got nil")
	}

	structMultiArgResult := NewChain([]any{person, 5}).Call(func(p Person, years int) Person {
		p.Age += years
		return p
	}).Call(func(p Person) int {
		return p.Age
	}).Value()

	if structMultiArgResult != 35 {
		t.Errorf("Expected 35, got %v", structMultiArgResult)
	}

	var resourceReleased bool
	resourceChain := NewChain("resource").Defer(func(v ...any) error {
		resourceReleased = true
		return nil
	}).Call(func(s string) string {
		return s + " processed"
	}).Call(func(s string) string {
		return s + " again"
	})

	runErr := resourceChain.Run()
	if runErr != nil {
		t.Errorf("Expected no error from Run, got %v", runErr)
	}

	if !resourceReleased {
		t.Errorf("Expected resource to be released")
	}

	var deferCount int
	multiDeferChain := NewChain(10).Defer(func(v ...any) error {
		deferCount++
		return nil
	}).Defer(func(v ...any) error {
		deferCount++
		return nil
	}).Call(func(a int) int {
		return a + 5
	})

	multiRunErr := multiDeferChain.Run()
	if multiRunErr != nil {
		t.Errorf("Expected no error from Run, got %v", multiRunErr)
	}

	if deferCount != 2 {
		t.Errorf("Expected 2 deferred functions to be called, got %v", deferCount)
	}

	errorDeferChain := NewChain(10).Defer(func(v ...any) error {
		return &TestError{Message: "defer error"}
	}).Call(func(a int) int {
		return a + 5
	})

	errorRunErr := errorDeferChain.Run()
	if errorRunErr == nil {
		t.Errorf("Expected error from Run, got nil")
	}

	errorPropagateChain := NewChain(10).Defer(func(v ...any) error {
		return nil
	}).Call("not a function")

	errorPropagateRunErr := errorPropagateChain.Run()
	if errorPropagateRunErr != nil {
		t.Errorf("Expected no error from Run, got %v", errorPropagateRunErr)
	}

	var capturedValues []any
	deferValueChain := NewChain(10).
		Defer(func(v ...any) error {
			if len(v) > 0 {
				capturedValues = append(capturedValues, v[0])
			}
			return nil
		}).
		Call(func(a int) int {
			return a + 5
		}).
		Defer(func(v ...any) error {
			if len(v) > 0 {
				capturedValues = append(capturedValues, v[0])
			}
			return nil
		}).
		Call(func(a int) int {
			return a * 2
		}).
		Defer(func(v ...any) error {
			if len(v) > 0 {
				capturedValues = append(capturedValues, v[0])
			}
			return nil
		})

	deferValueRunErr := deferValueChain.Run()
	if deferValueRunErr != nil {
		t.Errorf("Expected no error from Run, got %v", deferValueRunErr)
	}

	expectedValues := []any{10, 15, 30}
	if len(capturedValues) != len(expectedValues) {
		t.Errorf("Expected %d captured values, got %d", len(expectedValues), len(capturedValues))
	} else {
		for i, expected := range expectedValues {
			if capturedValues[i] != expected {
				t.Errorf("Expected captured value %d to be %v, got %v", i, expected, capturedValues[i])
			}
		}
	}

	var multiReturnCaptured []any
	multiReturnChain := NewChain(10).
		Call(func(a int) (int, string) {
			return a + 5, "result"
		}).
		Defer(func(values ...any) error {
			multiReturnCaptured = values
			return nil
		}).
		Call(func(a int) int {
			return a * 2
		})

	multiReturnRunErr := multiReturnChain.Run()
	if multiReturnRunErr != nil {
		t.Errorf("Expected no error from Run, got %v", multiReturnRunErr)
	}

	expectedMultiReturn := []any{15, "result"}
	if len(multiReturnCaptured) != len(expectedMultiReturn) {
		t.Errorf("Expected %d captured values, got %d", len(expectedMultiReturn), len(multiReturnCaptured))
	} else {
		for i, expected := range expectedMultiReturn {
			if multiReturnCaptured[i] != expected {
				t.Errorf("Expected captured value %d to be %v, got %v", i, expected, multiReturnCaptured[i])
			}
		}
	}

	valuesChain := NewChain(10).
		Call(func(a int) (int, string, bool) {
			return a + 5, "test", true
		})

	allValues := valuesChain.Values()
	expectedAllValues := []any{15, "test", true}
	if len(allValues) != len(expectedAllValues) {
		t.Errorf("Expected %d values, got %d", len(expectedAllValues), len(allValues))
	} else {
		for i, expected := range expectedAllValues {
			if allValues[i] != expected {
				t.Errorf("Expected value %d to be %v, got %v", i, expected, allValues[i])
			}
		}
	}

	var multipleDefersCaptured [][]any
	multipleDefersChain := NewChain(10).
		Defer(func(values ...any) error {
			multipleDefersCaptured = append(multipleDefersCaptured, values)
			return nil
		}).
		Call(func(a int) (int, string) {
			return a + 5, "first"
		}).
		Defer(func(values ...any) error {
			multipleDefersCaptured = append(multipleDefersCaptured, values)
			return nil
		}).
		Call(func(a int) (int, string, bool) {
			return a * 2, "second", true
		}).
		Defer(func(values ...any) error {
			multipleDefersCaptured = append(multipleDefersCaptured, values)
			return nil
		})

	multipleDefersRunErr := multipleDefersChain.Run()
	if multipleDefersRunErr != nil {
		t.Errorf("Expected no error from Run, got %v", multipleDefersRunErr)
	}

	if len(multipleDefersCaptured) != 3 {
		t.Errorf("Expected 3 captured value sets, got %d", len(multipleDefersCaptured))
	}

	errorMultiReturnChain := NewChain(10).
		Call(func(a int) (int, error) {
			return 0, &TestError{Message: "test error"}
		}).
		Defer(func(values ...any) error {
			return nil
		})

	errorMultiReturnRunErr := errorMultiReturnChain.Run()
	if errorMultiReturnRunErr != nil {
		t.Errorf("Expected no error from Run, got %v", errorMultiReturnRunErr)
	}

	multiReturnAsArgsChain := NewChain(10).
		Call(func(a int) (int, string, bool) {
			return a + 5, "test", true
		}).
		Call(func(a int, s string, b bool) string {
			if b {
				return s + " " + strconv.Itoa(a)
			}
			return ""
		})

	multiReturnAsArgsResult := multiReturnAsArgsChain.Value()
	if multiReturnAsArgsResult != "test 15" {
		t.Errorf("Expected 'test 15', got %v", multiReturnAsArgsResult)
	}

	useChain := NewChain(10).
		Call(func(a int) int {
			return a + 5
		}).
		Call(func(b int) int {
			return b * 2
		}).
		Use(1, 2).
		Call(func(a, b int) int {
			return a + b
		})

	useResult := useChain.Value()
	if useResult != 45 {
		t.Errorf("Expected 45, got %v", useResult)
	}

	useAnyChain := NewChain(5).
		Call(func(a int) (int, string) {
			return a * 2, "step1"
		}).
		Call(func(b int) (int, bool) {
			return b * 3, true
		}).
		Use(0, 1, 2).
		Call(func(a, b int, s string, c int, d bool) int {
			return a + b + c
		})

	useAnyResult := useAnyChain.Value()
	if useAnyResult != 45 {
		t.Errorf("Expected 45, got %v", useAnyResult)
	}

	useNameChain := NewChain(10).
		Call(func(a int) int {
			return a + 5
		}).
		Name("add").
		Call(func(b int) int {
			return b * 2
		}).
		Name("multiply").
		Use("add", "multiply").
		Call(func(a, b int) int {
			return a + b
		})

	useNameResult := useNameChain.Value()
	if useNameResult != 45 {
		t.Errorf("Expected 45, got %v", useNameResult)
	}
}
