package flow

import "reflect"

var (
	errorType = reflect.TypeOf((*error)(nil)).Elem()
)

type FlowError struct {
	Message string
}

func (e *FlowError) Error() string {
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
			return &FlowError{Message: ErrArgTypeMismatch}
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
			return nil, &FlowError{Message: ErrArgCountMismatch}
		}
		return nil, nil
	}

	args := reflectValueSlicePool.Get(argCount)
	if len(values) > 0 {
		if argCount > 0 && len(values) == argCount {
			for i := range len(values) {
				if err := addArg(&args, values[i], argTypes[i]); err != nil {
					reflectValueSlicePool.Put(args)
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
							reflectValueSlicePool.Put(args)
							return nil, &FlowError{Message: ErrArgTypeMismatch}
						}
					}
					sliceValue.Index(i).Set(valValue)
				}
				args = append(args, sliceValue)
			} else {
				currentValue := values[0].Interface()
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
						reflectValueSlicePool.Put(args)
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
					if err := addArg(&args, reflect.ValueOf(currentValue), argTypes[0]); err != nil {
						reflectValueSlicePool.Put(args)
						return nil, err
					}
				}
			}
		}
	}

	if len(args) != argCount {
		reflectValueSlicePool.Put(args)
		return nil, &FlowError{Message: ErrArgCountMismatch}
	}

	return args, nil
}
