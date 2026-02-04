package flow

import (
	"fmt"
	"reflect"
)

const (
	ErrArgTypeMismatch  = "argument type mismatch"
	ErrArgCountMismatch = "argument count mismatch"
	ErrNotFunction      = "argument is not a function"
	ErrFunctionPanicked = "function panicked"
	ErrStepNotFound     = "step not found"
)

type task struct {
	name   string
	fn     any
	values []any
	do     bool
}

type Chain struct {
	err       error
	values    []any
	stepNames map[string]int
	handlers  []*task
}

func NewChain() *Chain {
	return &Chain{
		values:    make([]any, 0),
		stepNames: make(map[string]int),
		handlers:  make([]*task, 0),
	}
}

func (c *Chain) Add(name string, fn any) *Chain {
	if c.err != nil {
		return c
	}
	c.stepNames[name] = len(c.handlers)
	c.handlers = append(c.handlers, &task{name: name, fn: fn})
	return c
}

func (c *Chain) Run() error {
	if c.err != nil {
		return c.err
	}
	for i := range c.handlers {
		if !c.handlers[i].do {
			c.values = c.call(c.handlers[i].fn, c.values)
			if c.err != nil {
				return c.err
			}
			c.handlers[i].do = true
		}
		c.handlers[i].values = c.values
	}
	return c.err
}

func (c *Chain) call(fn any, values []any) []any {
	if c.err != nil {
		return values
	}
	fnValue := reflect.ValueOf(fn)
	fnType := fnValue.Type()

	if fnType.Kind() != reflect.Func {
		c.handleNonFunctionType(fn, fnValue, fnType)
		return c.values
	}

	args, err := prepareArgs(values, fnType)
	if err != nil {
		c.err = err
		return values
	}

	var results []reflect.Value
	func() {
		defer func() {
			if r := recover(); r != nil {
				c.err = &ChainError{Message: fmt.Sprintf("%v: %v (fnType: %v, args: %v)", ErrFunctionPanicked, r, fnType, args)}
			}
		}()
		results = fnValue.Call(args)
	}()

	if c.err != nil {
		return values
	}

	if len(results) > fnType.NumOut() {
		c.err = &ChainError{Message: ErrFunctionPanicked}
		return values
	}

	newValues := make([]any, 0, len(results))
	for _, result := range results {
		newValues = append(newValues, result.Interface())
	}

	if fnType.NumOut() > 0 {
		lastOutType := fnType.Out(fnType.NumOut() - 1)
		if lastOutType.Implements(reflect.TypeOf((*error)(nil)).Elem()) {
			if len(results) == fnType.NumOut() {
				errValue := results[fnType.NumOut()-1]
				if !errValue.IsNil() {
					c.err = errValue.Interface().(error)
				}
			}
			if len(newValues) > 0 {
				newValues = newValues[:len(newValues)-1]
			}
			if len(newValues) == 0 {
				return values
			}
		}
	}

	return newValues
}

func (c *Chain) handleNonFunctionType(value any, valueReflect reflect.Value, valueType reflect.Type) {
	if valueType.Kind() == reflect.Slice || valueType.Kind() == reflect.Array {
		c.values = make([]any, valueReflect.Len())
		for i := range valueReflect.Len() {
			elem := valueReflect.Index(i)
			if elem.Kind() == reflect.Interface {
				elem = elem.Elem()
			}
			c.values[i] = elem.Interface()
		}
	} else {
		c.values = []any{value}
	}
}

func (c *Chain) Values(name string) ([]any, error) {
	if idx, ok := c.stepNames[name]; ok {
		if idx < len(c.handlers) {
			return c.handlers[idx].values, nil
		}
	}
	return nil, &ChainError{Message: ErrStepNotFound}
}

func (c *Chain) Value(name string) (any, error) {
	if idx, ok := c.stepNames[name]; ok {
		if idx < len(c.handlers) {
			if len(c.handlers[idx].values) > 0 {
				return c.handlers[idx].values[0], nil
			}
		}
	}
	return nil, &ChainError{Message: ErrStepNotFound}
}

func (c *Chain) Error() error {
	return c.err
}

func (c *Chain) Use(names ...string) *Chain {
	if c.err != nil {
		return c
	}

	newChain := &Chain{
		err:       c.err,
		values:    make([]any, 0),
		stepNames: make(map[string]int),
		handlers:  make([]*task, 0),
	}

	for _, name := range names {
		if idx, ok := c.stepNames[name]; !ok {
			c.err = &ChainError{Message: ErrStepNotFound}
			return c
		} else {
			newChain.values = append(newChain.values, c.handlers[idx].values...)
			newChain.handlers = append(newChain.handlers, c.handlers[idx])
			newChain.stepNames[name] = len(newChain.handlers) - 1
		}
	}

	return newChain
}

type ChainError struct {
	Message string
}

func (e *ChainError) Error() string {
	return e.Message
}

func canConvert(from, to reflect.Type) bool {
	if from == to {
		return true
	}
	if from.AssignableTo(to) {
		return true
	}
	if from.ConvertibleTo(to) {
		return true
	}
	return false
}

func addArg(args *[]reflect.Value, val any, argType reflect.Type) error {
	valValue := reflect.ValueOf(val)
	if !valValue.IsValid() {
		*args = append(*args, reflect.Zero(argType))
		return nil
	}
	valType := valValue.Type()

	if !valType.AssignableTo(argType) {
		if !canConvert(valType, argType) {
			return &ChainError{Message: ErrArgTypeMismatch}
		}
		*args = append(*args, valValue.Convert(argType))
	} else {
		*args = append(*args, valValue)
	}
	return nil
}

func prepareArgs(values []any, fnType reflect.Type) ([]reflect.Value, error) {
	var args []reflect.Value
	argCount := fnType.NumIn()

	if len(values) > 0 {
		if argCount > 0 && len(values) == argCount {
			for i := range len(values) {
				if err := addArg(&args, values[i], fnType.In(i)); err != nil {
					return nil, err
				}
			}
		} else {
			// Check if the first argument is a function with slice parameter
			if argCount == 1 && fnType.In(0).Kind() == reflect.Slice {
				// Pass all values as a single slice argument
				sliceType := fnType.In(0)
				sliceValue := reflect.MakeSlice(sliceType, len(values), len(values))
				for i := range values {
					elemType := sliceType.Elem()
					val := values[i]
					valValue := reflect.ValueOf(val)

					if !valValue.IsValid() {
						sliceValue.Index(i).Set(reflect.Zero(elemType))
						continue
					}

					if !valValue.Type().AssignableTo(elemType) {
						if valValue.CanConvert(elemType) {
							valValue = valValue.Convert(elemType)
						} else {
							return nil, &ChainError{Message: ErrArgTypeMismatch}
						}
					}
					sliceValue.Index(i).Set(valValue)
				}
				args = append(args, sliceValue)
			} else {
				// Existing logic for single value or variadic
				currentValue := values[0]
				currentValueType := reflect.TypeOf(currentValue)
				currentValueValue := reflect.ValueOf(currentValue)

				if currentValueType == nil {
					if argCount > 0 {
						args = append(args, reflect.Zero(fnType.In(0)))
					}
				} else if currentValueType.Kind() == reflect.Slice || currentValueType.Kind() == reflect.Array {
					elemCount := currentValueValue.Len()
					if argCount > 0 && elemCount != argCount {
						return nil, &ChainError{Message: ErrArgCountMismatch}
					}

					for i := range elemCount {
						elem := currentValueValue.Index(i)
						if elem.Kind() == reflect.Interface {
							elem = elem.Elem()
						}
						args = append(args, elem)
					}
				} else {
					if argCount > 0 {
						if err := addArg(&args, currentValue, fnType.In(0)); err != nil {
							return nil, err
						}
					}
				}
			}
		}
	}

	if len(args) != argCount {
		return nil, &ChainError{Message: ErrArgCountMismatch}
	}

	return args, nil
}
