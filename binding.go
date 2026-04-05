package suzume

import (
	"encoding"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

func parseArg(arg string, argType reflect.Type) (reflect.Value, error) {
	switch argType.Kind() {
	case reflect.String:
		return reflect.ValueOf(arg), nil
	case reflect.Int:
		v, err := strconv.Atoi(arg)
		if err != nil {
			return reflect.Value{}, fmt.Errorf("%w: expected an integer, got %q", ErrInvalidArgument, arg)
		}
		return reflect.ValueOf(v), nil
	case reflect.Int64:
		v, err := strconv.ParseInt(arg, 10, 64)
		if err != nil {
			return reflect.Value{}, fmt.Errorf("%w: expected an integer, got %q", ErrInvalidArgument, arg)
		}
		return reflect.ValueOf(v), nil
	case reflect.Float64:
		v, err := strconv.ParseFloat(arg, 64)
		if err != nil {
			return reflect.Value{}, fmt.Errorf("%w: expected a float, got %q", ErrInvalidArgument, arg)
		}
		return reflect.ValueOf(v), nil
	case reflect.Bool:
		v, err := strconv.ParseBool(arg)
		if err != nil {
			return reflect.Value{}, fmt.Errorf("%w: expected a boolean, got %q", ErrInvalidArgument, arg)
		}
		return reflect.ValueOf(v), nil
	}

	textUnmarshalerType := reflect.TypeFor[encoding.TextUnmarshaler]()
	if reflect.PointerTo(argType).Implements(textUnmarshalerType) {
		value := reflect.New(argType)
		unmarshaler := value.Interface().(encoding.TextUnmarshaler)
		if err := unmarshaler.UnmarshalText([]byte(arg)); err != nil {
			return reflect.Value{}, fmt.Errorf("%w: failed to parse argument: %v", ErrInvalidArgument, err)
		}
		return value.Elem(), nil
	}

	if argType.Implements(textUnmarshalerType) {
		value := reflect.New(argType).Elem()
		unmarshaler := value.Interface().(encoding.TextUnmarshaler)
		if err := unmarshaler.UnmarshalText([]byte(arg)); err != nil {
			return reflect.Value{}, fmt.Errorf("%w: failed to parse argument: %v", ErrInvalidArgument, err)
		}
		return value, nil
	}

	return reflect.Value{}, fmt.Errorf("unsupported argument type: %v", argType)
}

// 引数列を、argSpecsのvaluesに割り当てる
func bindArgsToValues(args []string, argSpecs []argSpec) error {
	for i := range argSpecs {
		argSpecs[i].value = reflect.Value{}
	}

	var targetArg *argSpec
	var positionalIndex int

	readArg := func(arg string, aspec *argSpec) error {
		if strings.Contains(arg, "=") {
			parts := strings.SplitN(arg, "=", 2)

			value, err := parseArg(parts[1], aspec.typeInfo)
			if err != nil {
				return fmt.Errorf("%w: failed to parse option %q: %v", ErrInvalidArgument, parts[0], err)
			}

			if aspec.typeInfo.Kind() == reflect.Slice {
				if !aspec.value.IsValid() {
					aspec.value = reflect.MakeSlice(aspec.typeInfo, 0, 0)
				}
				aspec.value = reflect.Append(aspec.value, value)
			} else {
				aspec.value = value
			}
		} else if aspec.typeInfo.Kind() == reflect.Bool {
			aspec.value = reflect.ValueOf(true)
		} else {
			targetArg = aspec
		}
		return nil
	}

	for _, arg := range args {
		if targetArg == nil {
			if aspec, ok := getArgSpecByFlag(argSpecs, arg); ok {
				// オプション引数

				if err := readArg(arg, aspec); err != nil {
					return err
				}
			} else {
				// 位置引数

				if positionalIndex >= len(argSpecs) || argSpecs[positionalIndex].index == -1 {
					return fmt.Errorf("%w: unexpected positional argument %q", ErrInvalidArgument, arg)
				}

				value, err := parseArg(arg, argSpecs[positionalIndex].typeInfo)
				if err != nil {
					return fmt.Errorf("%w: failed to parse argument %d: %v", ErrInvalidArgument, positionalIndex+1, err)
				}
				argSpecs[positionalIndex].value = value
				positionalIndex++
			}
		} else if targetArg.typeInfo.Kind() == reflect.Slice {
			if aspec, ok := getArgSpecByFlag(argSpecs, arg); ok {
				// オプション引数

				if err := readArg(arg, aspec); err != nil {
					return err
				}
			} else {
				// スライスの追加
				value, err := parseArg(arg, targetArg.typeInfo.Elem())
				if err != nil {
					return fmt.Errorf("%w: failed to parse argument %q: %v", ErrInvalidArgument, arg, err)
				}

				if !targetArg.value.IsValid() {
					targetArg.value = reflect.MakeSlice(targetArg.typeInfo, 0, 0)
				}

				targetArg.value = reflect.Append(targetArg.value, value)
			}
		} else {
			// オプション引数
			value, err := parseArg(arg, targetArg.typeInfo)
			if err != nil {
				return fmt.Errorf("%w: failed to parse argument %q: %v", ErrInvalidArgument, arg, err)
			}
			targetArg.value = value
			targetArg = nil
		}
	}

	if positionalIndex < len(argSpecs) && argSpecs[positionalIndex].index != -1 {
		return fmt.Errorf("%w: missing required positional argument: %s", ErrInvalidArgument, argSpecs[positionalIndex].name)
	}

	if targetArg != nil && targetArg.typeInfo.Kind() != reflect.Slice {
		return fmt.Errorf("%w: missing value for option: %s", ErrInvalidArgument, targetArg.name)
	}

	return nil
}

func getArgSpecByFlag(argSpecs []argSpec, arg string) (*argSpec, bool) {
	if !strings.HasPrefix(arg, "-") {
		return nil, false
	}
	if strings.Contains(arg, "=") {
		parts := strings.SplitN(arg, "=", 2)
		arg = parts[0]
	}

	arg = strings.TrimLeft(arg, "-")
	for i := range argSpecs {
		if argSpecs[i].name == arg || argSpecs[i].short == arg {
			return &argSpecs[i], true
		}
	}

	return nil, false
}
