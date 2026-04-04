package mycli

import (
	"encoding"
	"fmt"
	"reflect"
	"strconv"
)

func parseArg(arg string, argType reflect.Type) (reflect.Value, error) {
	switch argType.Kind() {
	case reflect.String:
		return reflect.ValueOf(arg), nil
	case reflect.Int:
		v, err := strconv.Atoi(arg)
		if err != nil {
			return reflect.Value{}, fmt.Errorf("expected an integer, got %q", arg)
		}
		return reflect.ValueOf(v), nil
	case reflect.Int64:
		v, err := strconv.ParseInt(arg, 10, 64)
		if err != nil {
			return reflect.Value{}, fmt.Errorf("expected an integer, got %q", arg)
		}
		return reflect.ValueOf(v), nil
	case reflect.Float64:
		v, err := strconv.ParseFloat(arg, 64)
		if err != nil {
			return reflect.Value{}, fmt.Errorf("expected a float, got %q", arg)
		}
		return reflect.ValueOf(v), nil
	}

	textUnmarshalerType := reflect.TypeFor[encoding.TextUnmarshaler]()
	if reflect.PointerTo(argType).Implements(textUnmarshalerType) {
		value := reflect.New(argType)
		unmarshaler := value.Interface().(encoding.TextUnmarshaler)
		if err := unmarshaler.UnmarshalText([]byte(arg)); err != nil {
			return reflect.Value{}, fmt.Errorf("failed to parse argument: %v", err)
		}
		return value.Elem(), nil
	}

	if argType.Implements(textUnmarshalerType) {
		value := reflect.New(argType).Elem()
		unmarshaler := value.Interface().(encoding.TextUnmarshaler)
		if err := unmarshaler.UnmarshalText([]byte(arg)); err != nil {
			return reflect.Value{}, fmt.Errorf("failed to parse argument: %v", err)
		}
		return value, nil
	}

	return reflect.Value{}, fmt.Errorf("unsupported argument type: %v", argType)
}
