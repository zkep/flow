package flow

import (
	"context"
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

var (
	errorType = reflect.TypeOf((*error)(nil)).Elem()
	argsPool  = NewSlicePool[reflect.Value](128, 32)
)

type (
	task struct {
		name     string
		values   []reflect.Value
		fnValue  reflect.Value
		argTypes []reflect.Type
		do       bool
	}

	Chain struct {
		err       error
		values    []reflect.Value
		stepNames map[string]int
		handlers  []*task
	}
)

func NewChain() *Chain {
	return &Chain{
		values:    make([]reflect.Value, 0, 8),
		stepNames: make(map[string]int, 8),
		handlers:  make([]*task, 0, 8),
	}
}

func (c *Chain) Add(name string, fn any) *Chain {
	if c.err != nil {
		return c
	}
	fnValue := reflect.ValueOf(fn)
	fnType := fnValue.Type()
	var argTypes []reflect.Type
	var values []reflect.Value
	var t task
	if fnType.Kind() == reflect.Func {
		argCount := fnType.NumIn()
		argTypes = make([]reflect.Type, argCount)
		for i := range argCount {
			argTypes[i] = fnType.In(i)
		}
	} else {
		argTypes = []reflect.Type{fnType}
		values = []reflect.Value{fnValue}
	}
	t = task{name: name, fnValue: fnValue, argTypes: argTypes, values: values}
	c.stepNames[name] = len(c.handlers)
	c.handlers = append(c.handlers, &t)
	return c
}

func (c *Chain) Run() error {
	if c.err != nil {
		return c.err
	}
	return c.RunWithContext(context.Background())
}

func (c *Chain) RunWithContext(ctx context.Context) error {
	if c.err != nil {
		return c.err
	}
	for i := range c.handlers {
		if !c.handlers[i].do {
			select {
			case <-ctx.Done():
				c.err = &ChainError{Message: fmt.Sprintf("execution canceled: %v", ctx.Err())}
				return c.err
			default:
			}
			c.values = c.call(c.handlers[i].fnValue, c.handlers[i].argTypes, c.values)
			if c.err != nil {
				return c.err
			}
			c.handlers[i].do = true
		}
		c.handlers[i].values = c.values
	}
	return c.err
}

func (c *Chain) call(fnValue reflect.Value, argTypes []reflect.Type, values []reflect.Value) []reflect.Value {
	if c.err != nil {
		return values
	}
	fnType := fnValue.Type()

	if fnType.Kind() != reflect.Func {
		c.handleNonFunctionType(fnValue, fnType)
		return c.values
	}

	args, err := prepareArgsWithType(values, argTypes)
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

	argsPool.Put(args)

	if c.err != nil {
		return values
	}

	outCount := fnType.NumOut()
	if len(results) > outCount {
		c.err = &ChainError{Message: ErrFunctionPanicked}
		return values
	}

	hasError := outCount > 0 && fnType.Out(outCount-1).Implements(errorType)
	resultCount := len(results)
	if hasError {
		resultCount--
	}

	if resultCount <= 0 {
		if hasError && len(results) > 0 {
			errValue := results[len(results)-1]
			if !errValue.IsNil() {
				c.err = errValue.Interface().(error)
			}
		}
		return values
	}

	newValues := make([]reflect.Value, resultCount)
	for i := 0; i < resultCount; i++ {
		newValues[i] = results[i]
	}

	if hasError && len(results) > resultCount {
		errValue := results[resultCount]
		if !errValue.IsNil() {
			c.err = errValue.Interface().(error)
		}
	}

	return newValues
}

func (c *Chain) handleNonFunctionType(value reflect.Value, valueType reflect.Type) {
	if valueType.Kind() == reflect.Slice || valueType.Kind() == reflect.Array {
		c.values = make([]reflect.Value, value.Len())
		for i := range value.Len() {
			elem := value.Index(i)
			if elem.Kind() == reflect.Interface {
				elem = elem.Elem()
			}
			c.values[i] = elem
		}
	} else {
		c.values = []reflect.Value{value}
	}
}

func (c *Chain) Values(name string) ([]any, error) {
	if idx, ok := c.stepNames[name]; ok {
		if idx < len(c.handlers) {
			values := make([]any, len(c.handlers[idx].values))
			for i := range len(c.handlers[idx].values) {
				values[i] = c.handlers[idx].values[i].Interface()
			}
			return values, nil
		}
	}
	return nil, &ChainError{Message: ErrStepNotFound}
}

func (c *Chain) Value(name string) (any, error) {
	if idx, ok := c.stepNames[name]; ok {
		if idx < len(c.handlers) {
			if len(c.handlers[idx].values) > 0 {
				return c.handlers[idx].values[0].Interface(), nil
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
		values:    make([]reflect.Value, 0),
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

func addArg(args *[]reflect.Value, val reflect.Value, argType reflect.Type) error {
	if !val.IsValid() {
		*args = append(*args, reflect.Zero(argType))
		return nil
	}
	valType := val.Type()
	if !valType.AssignableTo(argType) {
		if !canConvert(valType, argType) {
			return &ChainError{Message: ErrArgTypeMismatch}
		}
		*args = append(*args, val.Convert(argType))
	} else {
		*args = append(*args, val)
	}
	return nil
}

func prepareArgsWithType(values []reflect.Value, argTypes []reflect.Type) ([]reflect.Value, error) {
	argCount := len(argTypes)
	if argCount == 0 {
		if len(values) > 0 {
			return nil, &ChainError{Message: ErrArgCountMismatch}
		}
		return nil, nil
	}

	args := argsPool.Get(argCount)
	if len(values) > 0 {
		if argCount > 0 && len(values) == argCount {
			for i := range len(values) {
				if err := addArg(&args, values[i], argTypes[i]); err != nil {
					argsPool.Put(args)
					return nil, err
				}
			}
		} else {
			if argCount == 1 && argTypes[0].Kind() == reflect.Slice {
				sliceType := argTypes[0]
				sliceValue := reflect.MakeSlice(sliceType, len(values), len(values))
				for i := range values {
					elemType := sliceType.Elem()
					val := values[i].Interface()
					valValue := reflect.ValueOf(val)

					if !valValue.IsValid() {
						sliceValue.Index(i).Set(reflect.Zero(elemType))
						continue
					}

					if !valValue.Type().AssignableTo(elemType) {
						if valValue.CanConvert(elemType) {
							valValue = valValue.Convert(elemType)
						} else {
							argsPool.Put(args)
							return nil, &ChainError{Message: ErrArgTypeMismatch}
						}
					}
					sliceValue.Index(i).Set(valValue)
				}
				args = append(args, sliceValue)
			} else {
				currentValue := values[0].Interface()
				currentValueType := reflect.TypeOf(currentValue)
				currentValueValue := reflect.ValueOf(currentValue)

				if currentValueType == nil {
					if argCount > 0 {
						args = append(args, reflect.Zero(argTypes[0]))
					}
				} else if currentValueType.Kind() == reflect.Slice || currentValueType.Kind() == reflect.Array {
					elemCount := currentValueValue.Len()
					if argCount > 0 && elemCount != argCount {
						argsPool.Put(args)
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
						if err := addArg(&args, reflect.ValueOf(currentValue), argTypes[0]); err != nil {
							argsPool.Put(args)
							return nil, err
						}
					}
				}
			}
		}
	}

	if len(args) != argCount {
		argsPool.Put(args)
		return nil, &ChainError{Message: ErrArgCountMismatch}
	}

	return args, nil
}
