package flow

import "reflect"

const (
	ErrDeferredTaskNotFunction = "deferred task is not a function"
	ErrDeferredArgTypeMismatch = "argument type mismatch in deferred function"
	ErrArgTypeMismatch         = "argument type mismatch"
	ErrArgCountMismatch        = "argument count mismatch"
	ErrNotFunction             = "argument is not a function"
	ErrFunctionPanicked        = "function panicked"
)

type deferredTask struct {
	fn     any
	values []any
}

type Chain struct {
	values    []any
	history   [][]any
	stepNames map[string]int
	err       error
	deferred  []deferredTask
}

func NewChain(initial ...any) *Chain {
	var vs []any
	if len(initial) > 0 {
		vs = initial
	}
	history := make([][]any, 0)
	if len(vs) > 0 {
		history = append(history, vs)
	}
	return &Chain{
		values:    vs,
		history:   history,
		stepNames: make(map[string]int),
		deferred:  make([]deferredTask, 0),
	}
}

func (c *Chain) Defer(fn any) *Chain {
	if c.err != nil {
		return c
	}
	c.deferred = append(c.deferred, deferredTask{
		fn:     fn,
		values: c.values,
	})
	return c
}

func (c *Chain) Run() error {
	for _, task := range c.deferred {
		fnValue := reflect.ValueOf(task.fn)
		fnType := fnValue.Type()

		if fnType.Kind() != reflect.Func {
			return &ChainError{Message: ErrDeferredTaskNotFunction}
		}

		var args []reflect.Value
		argCount := fnType.NumIn()
		valueCount := len(task.values)

		isVariadic := fnType.IsVariadic()

		if isVariadic && argCount == 1 {
			argType := fnType.In(0)
			if argType.Kind() == reflect.Slice {
				sliceType := argType
				sliceValue := reflect.MakeSlice(sliceType, valueCount, valueCount)

				for i := range valueCount {
					elemType := sliceType.Elem()
					val := task.values[i]
					valValue := reflect.ValueOf(val)

					if !valValue.Type().AssignableTo(elemType) {
						if valValue.CanConvert(elemType) {
							valValue = valValue.Convert(elemType)
						} else {
							return &ChainError{Message: ErrDeferredArgTypeMismatch}
						}
					}
					sliceValue.Index(i).Set(valValue)
				}

				args = append(args, sliceValue)
			}
		} else {
			for i := range argCount {
				if i < valueCount {
					argType := fnType.In(i)
					val := task.values[i]
					valValue := reflect.ValueOf(val)

					if !valValue.Type().AssignableTo(argType) {
						if valValue.CanConvert(argType) {
							valValue = valValue.Convert(argType)
						} else {
							return &ChainError{Message: ErrDeferredArgTypeMismatch}
						}
					}
					args = append(args, valValue)
				} else {
					argType := fnType.In(i)
					args = append(args, reflect.Zero(argType))
				}
			}
		}

		var results []reflect.Value
		if isVariadic {
			results = fnValue.CallSlice(args)
		} else {
			results = fnValue.Call(args)
		}

		if fnType.NumOut() > 0 {
			lastOutType := fnType.Out(fnType.NumOut() - 1)
			if lastOutType.Implements(reflect.TypeOf((*error)(nil)).Elem()) {
				if len(results) == fnType.NumOut() {
					errValue := results[fnType.NumOut()-1]
					if !errValue.IsNil() {
						return errValue.Interface().(error)
					}
				}
			}
		}
	}
	return nil
}

func (c *Chain) Values() []any {
	return c.values
}

func (c *Chain) Call(fn any) *Chain {
	if c.err != nil {
		return c
	}

	fnValue := reflect.ValueOf(fn)
	fnType := fnValue.Type()

	if fnType.Kind() != reflect.Func {
		c.handleNonFunctionType(fn, fnValue, fnType)
		c.history = append(c.history, c.values)
		return c
	}

	args, err := prepareArgs(c.values, fnType)
	if err != nil {
		c.err = err
		return c
	}

	results := fnValue.Call(args)

	if len(results) > fnType.NumOut() {
		c.err = &ChainError{Message: ErrFunctionPanicked}
		return c
	}

	c.values = make([]any, 0, len(results))
	for _, result := range results {
		c.values = append(c.values, result.Interface())
	}

	c.history = append(c.history, c.values)

	if fnType.NumOut() > 1 {
		lastOutType := fnType.Out(fnType.NumOut() - 1)
		if lastOutType.Implements(reflect.TypeOf((*error)(nil)).Elem()) {
			if len(results) == fnType.NumOut() {
				errValue := results[fnType.NumOut()-1]
				if !errValue.IsNil() {
					c.err = errValue.Interface().(error)
				}
			}
		}
	}

	return c
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

func (c *Chain) Value() any {
	if len(c.values) > 0 {
		return c.values[0]
	}
	return nil
}

func (c *Chain) Error() error {
	return c.err
}

func (c *Chain) History() [][]any {
	return c.history
}

func (c *Chain) Name(name string) *Chain {
	if c.err != nil {
		return c
	}
	stepIndex := len(c.history) - 1
	if stepIndex >= 0 {
		c.stepNames[name] = stepIndex
	}
	return c
}

func (c *Chain) Use(steps ...any) *Chain {
	if c.err != nil {
		return c
	}

	newChain := &Chain{
		values:    make([]any, 0),
		history:   c.history,
		stepNames: c.stepNames,
		err:       c.err,
		deferred:  c.deferred,
	}

	for _, step := range steps {
		switch v := step.(type) {
		case int:
			if v >= 0 && v < len(c.history) {
				newChain.values = append(newChain.values, c.history[v]...)
			}
		case string:
			if idx, ok := c.stepNames[v]; ok {
				newChain.values = append(newChain.values, c.history[idx]...)
			}
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
	return false
}

func addArg(args *[]reflect.Value, val any, argType reflect.Type) error {
	valValue := reflect.ValueOf(val)
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
			for i := 0; i < len(values); i++ {
				if err := addArg(&args, values[i], fnType.In(i)); err != nil {
					return nil, err
				}
			}
		} else {
			currentValue := values[0]
			currentValueType := reflect.TypeOf(currentValue)
			currentValueValue := reflect.ValueOf(currentValue)

			if currentValueType.Kind() == reflect.Slice || currentValueType.Kind() == reflect.Array {
				elemCount := currentValueValue.Len()
				if argCount > 0 && elemCount != argCount {
					return nil, &ChainError{Message: ErrArgCountMismatch}
				}

				for i := 0; i < elemCount; i++ {
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
				} else {
					args = append(args, currentValueValue)
				}
			}
		}
	}

	if len(args) != argCount {
		return nil, &ChainError{Message: ErrArgCountMismatch}
	}

	return args, nil
}
