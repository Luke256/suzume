package suzume

import (
	"encoding"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

func supportsArgumentType(argType reflect.Type) bool {
	textUnmarshalerType := reflect.TypeFor[encoding.TextUnmarshaler]()
	if reflect.PointerTo(argType).Implements(textUnmarshalerType) || argType.Implements(textUnmarshalerType) {
		return true
	}

	switch argType.Kind() {
	case reflect.String,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64,
		reflect.Complex64, reflect.Complex128:
		return true
	default:
		return false
	}
}

func parseArg(arg string, argType reflect.Type) (reflect.Value, error) {
	textUnmarshalerType := reflect.TypeFor[encoding.TextUnmarshaler]()
	if reflect.PointerTo(argType).Implements(textUnmarshalerType) {
		value := reflect.New(argType)
		unmarshaler := value.Interface().(encoding.TextUnmarshaler)
		if err := unmarshaler.UnmarshalText([]byte(arg)); err != nil {
			return reflect.Value{}, fmt.Errorf("%w: failed to parse argument: %w", ErrInvalidArgument, err)
		}
		return value.Elem(), nil
	}

	if argType.Implements(textUnmarshalerType) {
		value := reflect.New(argType).Elem()
		unmarshaler := value.Interface().(encoding.TextUnmarshaler)
		if err := unmarshaler.UnmarshalText([]byte(arg)); err != nil {
			return reflect.Value{}, fmt.Errorf("%w: failed to parse argument: %w", ErrInvalidArgument, err)
		}
		return value, nil
	}

	value := reflect.New(argType).Elem()

	switch argType.Kind() {
	case reflect.String:
		value.SetString(arg)
	case reflect.Bool:
		v, err := strconv.ParseBool(arg)
		if err != nil {
			return reflect.Value{}, fmt.Errorf("%w: expected a boolean, got %q", ErrInvalidArgument, arg)
		}
		value.SetBool(v)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v, err := strconv.ParseInt(arg, 10, argType.Bits())
		if err != nil {
			return reflect.Value{}, fmt.Errorf("%w: expected an integer, got %q", ErrInvalidArgument, arg)
		}
		value.SetInt(v)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		v, err := strconv.ParseUint(arg, 10, argType.Bits())
		if err != nil {
			return reflect.Value{}, fmt.Errorf("%w: expected an unsigned integer, got %q", ErrInvalidArgument, arg)
		}
		value.SetUint(v)
	case reflect.Float32, reflect.Float64:
		v, err := strconv.ParseFloat(arg, argType.Bits())
		if err != nil {
			return reflect.Value{}, fmt.Errorf("%w: expected a float, got %q", ErrInvalidArgument, arg)
		}
		value.SetFloat(v)
	case reflect.Complex64, reflect.Complex128:
		v, err := strconv.ParseComplex(arg, argType.Bits())
		if err != nil {
			return reflect.Value{}, fmt.Errorf("%w: expected a complex number, got %q", ErrInvalidArgument, arg)
		}
		value.SetComplex(v)
	default:
		return reflect.Value{}, fmt.Errorf("unsupported argument type: %v", argType)
	}

	return value, nil
}

// 引数列を、argSpecsに対応する実行ローカルなvaluesに割り当てる
func bindArgsToValues(args []string, argSpecs []argSpec) ([]reflect.Value, error) {
	values := make([]reflect.Value, len(argSpecs))
	targetIndex := -1
	var positionalIndex int

	readArg := func(arg string, specIndex int) error {
		aspec := argSpecs[specIndex]
		if strings.Contains(arg, "=") {
			parts := strings.SplitN(arg, "=", 2)
			valueType := aspec.typeInfo
			if valueType.Kind() == reflect.Slice {
				valueType = valueType.Elem()
			}

			value, err := parseArg(parts[1], valueType)
			if err != nil {
				return fmt.Errorf("%w: failed to parse option %q: %w", ErrInvalidArgument, parts[0], err)
			}

			if aspec.typeInfo.Kind() == reflect.Slice {
				values[specIndex] = reflect.Append(reflect.MakeSlice(aspec.typeInfo, 0, 1), value)
			} else {
				values[specIndex] = value
			}
		} else if aspec.typeInfo.Kind() == reflect.Bool {
			values[specIndex] = reflect.New(aspec.typeInfo).Elem()
			values[specIndex].SetBool(true)
		} else {
			targetIndex = specIndex
		}
		return nil
	}

	for _, arg := range args {
		if targetIndex < 0 {
			if specIndex, ok := getArgSpecIndexByFlag(argSpecs, arg); ok {
				// オプション引数

				if err := readArg(arg, specIndex); err != nil {
					return nil, err
				}
			} else {
				// 位置引数

				if positionalIndex >= len(argSpecs) || argSpecs[positionalIndex].index < 0 {
					return nil, fmt.Errorf("%w: unexpected positional argument %q", ErrInvalidArgument, arg)
				}

				value, err := parseArg(arg, argSpecs[positionalIndex].typeInfo)
				if err != nil {
					return nil, fmt.Errorf("%w: failed to parse argument %d: %w", ErrInvalidArgument, positionalIndex+1, err)
				}
				values[positionalIndex] = value
				positionalIndex++
			}
		} else if argSpecs[targetIndex].typeInfo.Kind() == reflect.Slice {
			if specIndex, ok := getArgSpecIndexByFlag(argSpecs, arg); ok {
				// オプション引数

				if err := readArg(arg, specIndex); err != nil {
					return nil, err
				}
			} else {
				// スライスの追加
				value, err := parseArg(arg, argSpecs[targetIndex].typeInfo.Elem())
				if err != nil {
					return nil, fmt.Errorf("%w: failed to parse argument %q: %w", ErrInvalidArgument, arg, err)
				}

				if !values[targetIndex].IsValid() {
					values[targetIndex] = reflect.MakeSlice(argSpecs[targetIndex].typeInfo, 0, 0)
				}

				values[targetIndex] = reflect.Append(values[targetIndex], value)
			}
		} else {
			// オプション引数
			value, err := parseArg(arg, argSpecs[targetIndex].typeInfo)
			if err != nil {
				return nil, fmt.Errorf("%w: failed to parse argument %q: %w", ErrInvalidArgument, arg, err)
			}
			values[targetIndex] = value
			targetIndex = -1
		}
	}

	if positionalIndex < len(argSpecs) && argSpecs[positionalIndex].index >= 0 {
		return nil, fmt.Errorf("%w: missing required positional argument: %s", ErrInvalidArgument, argSpecs[positionalIndex].name)
	}

	if targetIndex >= 0 && argSpecs[targetIndex].typeInfo.Kind() != reflect.Slice {
		return nil, fmt.Errorf("%w: missing value for option: %s", ErrInvalidArgument, argSpecs[targetIndex].name)
	}

	return values, nil
}

func getArgSpecIndexByFlag(argSpecs []argSpec, arg string) (int, bool) {
	if !strings.HasPrefix(arg, "-") {
		return 0, false
	}
	if strings.Contains(arg, "=") {
		parts := strings.SplitN(arg, "=", 2)
		arg = parts[0]
	}

	arg = strings.TrimLeft(arg, "-")
	for i := range argSpecs {
		if argSpecs[i].name == arg || argSpecs[i].short == arg {
			return i, true
		}
	}

	return 0, false
}
