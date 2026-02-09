package flow

import (
	"reflect"
)

type condCompiler struct {
	fnValue    reflect.Value
	fnType     reflect.Type
	argCount   int
	isVariadic bool
}

func newCondCompiler(cond any) *condCompiler {
	c := condCompilerPool.Get()
	c.fnValue = reflect.ValueOf(cond)
	c.fnType = c.fnValue.Type()
	c.argCount = c.fnType.NumIn()
	c.isVariadic = c.fnType.IsVariadic()
	return c
}

func (c *condCompiler) eval(results []any) bool {
	args := reflectValueSlicePool.Get(c.argCount)
	defer reflectValueSlicePool.Put(args)

	if c.isVariadic && len(results) > 0 {
		sliceType := c.fnType.In(c.argCount - 1).Elem()
		slice := reflect.MakeSlice(reflect.SliceOf(sliceType), 0, len(results))
		for _, result := range results {
			slice = reflect.Append(slice, reflect.ValueOf(result))
		}
		args = append(args, slice)
	} else if c.argCount > 0 {
		resultCount := min(len(results), c.argCount)
		for i := range resultCount {
			args = append(args, reflect.ValueOf(results[i]))
		}
		for i := resultCount; i < c.argCount; i++ {
			args = append(args, reflect.Zero(c.fnType.In(i)))
		}
	}

	var condResult []reflect.Value
	if c.isVariadic {
		condResult = c.fnValue.CallSlice(args)
	} else {
		condResult = c.fnValue.Call(args)
	}

	if len(condResult) > 0 {
		if condResult[0].Kind() == reflect.Bool {
			return condResult[0].Bool()
		}
		if condResult[0].Kind() == reflect.Interface && !condResult[0].IsNil() {
			if b, ok := condResult[0].Elem().Interface().(bool); ok {
				return b
			}
		}
	}
	return true
}

func (g *Graph) compileCondition(cond any) CondFunc {
	if cond == nil {
		return nil
	}

	if c, ok := cond.(CondFunc); ok {
		return c
	}

	if b, ok := cond.(bool); ok {
		if b {
			return nil
		}
		return func([]any) bool { return false }
	}

	fnValue := reflect.ValueOf(cond)
	fnType := fnValue.Type()

	if fnType.Kind() != reflect.Func {
		return nil
	}

	comp := newCondCompiler(cond)
	return comp.eval
}

func (g *Graph) compileNodeCall(node *Node) func([]any) ([]any, error) {
	if node.fn == nil {
		return func(inputs []any) ([]any, error) {
			return inputs, nil
		}
	}

	fnValue := node.fnValue
	argCount := node.argCount
	sliceArg := node.sliceArg
	sliceElemType := node.sliceElemType
	hasError := node.hasErrorReturn
	argTypes := node.argTypes

	return func(inputs []any) ([]any, error) {
		args := reflectValueSlicePool.Get(argCount)
		defer reflectValueSlicePool.Put(args)

		if len(inputs) > 0 {
			if argCount > 0 && len(inputs) == argCount { //nolint:gocritic
				for i := range len(inputs) {
					input := inputs[i]
					if input == nil {
						args = append(args, reflect.Zero(argTypes[i]))
						continue
					}
					val := reflect.ValueOf(input)
					if !val.Type().AssignableTo(argTypes[i]) {
						if val.CanConvert(argTypes[i]) {
							val = val.Convert(argTypes[i])
						} else {
							return nil, &FlowError{Message: ErrArgTypeMismatch}
						}
					}
					args = append(args, val)
				}
			} else if sliceArg {
				sliceValue := reflect.MakeSlice(argTypes[0], len(inputs), len(inputs))
				for i := range inputs {
					val := reflect.ValueOf(inputs[i])
					if !val.Type().AssignableTo(sliceElemType) {
						if val.CanConvert(sliceElemType) {
							val = val.Convert(sliceElemType)
						} else {
							return nil, &FlowError{Message: ErrArgTypeMismatch}
						}
					}
					sliceValue.Index(i).Set(val)
				}
				args = append(args, sliceValue)
			} else if len(inputs) > 0 {
				currentValue := inputs[0]
				currentValueType := reflect.TypeOf(currentValue)
				currentValueValue := reflect.ValueOf(currentValue)

				switch {
				case currentValueType == nil:
					if argCount > 0 {
						args = append(args, reflect.Zero(argTypes[0]))
					}
				case currentValueType.Kind() == reflect.Slice || currentValueType.Kind() == reflect.Array:
					elemCount := currentValueValue.Len()
					if argCount > 0 && elemCount != argCount {
						return nil, &FlowError{Message: ErrArgCountMismatch}
					}
					for i := range elemCount {
						elem := currentValueValue.Index(i)
						if elem.Kind() == reflect.Interface {
							elem = elem.Elem()
						}
						args = append(args, elem)
					}
				case argCount > 0:
					val := currentValueValue
					if !val.Type().AssignableTo(argTypes[0]) {
						if val.CanConvert(argTypes[0]) {
							val = val.Convert(argTypes[0])
						} else {
							return nil, &FlowError{Message: ErrArgTypeMismatch}
						}
					}
					args = append(args, val)
				}
			}
		}

		if len(args) != argCount {
			return nil, &FlowError{Message: ErrArgCountMismatch}
		}

		results := fnValue.Call(args)

		if hasError {
			errValue := results[len(results)-1]
			if !errValue.IsNil() {
				return nil, errValue.Interface().(error)
			}
			results = results[:len(results)-1]
		}

		out := make([]any, len(results))
		for i, r := range results {
			out[i] = r.Interface()
		}
		return out, nil
	}
}
