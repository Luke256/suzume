package mycli

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"unicode"
)

// 固定引数の関数をコマンドのハンドラー
// args: ["arg1", "arg2", ...]
func createFunctionHandler(runFunc any) ([]argSpec, func(args ...string) error, error) {
	v := reflect.ValueOf(runFunc)
	if !v.IsValid() || v.Kind() != reflect.Func {
		return nil, nil, fmt.Errorf("expected a function, got %T", runFunc)
	}

	numArgs := v.Type().NumIn()
	argSpecs := make([]argSpec, numArgs+1)

	for i := range numArgs {
		inputType := v.Type().In(i)
		if inputType.Kind() == reflect.Slice {
			return nil, nil, fmt.Errorf("slice fields cannot be used as positional arguments: argument %d", i+1)
		}
		if inputType.Kind() == reflect.Bool {
			return nil, nil, fmt.Errorf("boolean arguments should be handled as flags")
		}
		argSpecs[i] = argSpec{
			index: i,
			name:  fmt.Sprintf("arg%d", i+1),
		}
	}
	argSpecs[numArgs] = helpArgSpec

	return argSpecs, func(args ...string) error {
		if numArgs != len(args) {
			return fmt.Errorf("%w: expected %d arguments, got %d", ErrInvalidArgument, numArgs, len(args))
		}

		in := make([]reflect.Value, numArgs)
		for i := range numArgs {
			argValue, err := parseArg(args[i], v.Type().In(i))
			if err != nil {
				return fmt.Errorf("%w: argument %d: %v", ErrInvalidArgument, i+1, err)
			}
			in[i] = argValue
		}

		out := v.Call(in)
		if len(out) == 1 && out[0].Type() == reflect.TypeFor[error]() {
			if !out[0].IsNil() {
				return out[0].Interface().(error)
			}
		}

		return nil
	}, nil
}

func pascalToSnake(s string) string {
	var result []string
	for i, r := range s {
		if i > 0 && unicode.IsUpper(r) {
			result = append(result, "_")
		}
		result = append(result, string(unicode.ToLower(r)))
	}
	return strings.Join(result, "")
}

// args: ["arg1", "arg2", ... , "--flag", "--opt=value", "--opt", "value", "-o", "value", ...]
func createRunnerHandler[T Runner]() ([]argSpec, func(args ...string) error, error) {
	v := reflect.TypeFor[T]()

	// Tがポインタの場合はエラー
	if v.Kind() == reflect.Pointer {
		return nil, nil, fmt.Errorf("Runner type cannot be a pointer: %v", v)
	}

	argSpecs := make([]argSpec, v.NumField()+1)
	argSpecs[v.NumField()] = helpArgSpec

	for i := range v.NumField() {
		field := v.Field(i)
		if idx, err := strconv.Atoi(field.Tag.Get("cli")); err == nil {
			if field.Type.Kind() == reflect.Slice {
				return nil, nil, fmt.Errorf("slice fields cannot be used as positional arguments: %s", field.Name)
			}
			argSpecs[i] = argSpec{
				index: idx,
				name:  pascalToSnake(field.Name),
				usage: field.Tag.Get("usage"),
			}
		} else {
			argSpecs[i] = argSpec{
				index: -1,
				name:  field.Tag.Get("cli"),
				short: field.Tag.Get("short"),
				usage: field.Tag.Get("usage"),
			}
		}
	}

	return argSpecs, func(args ...string) error {
		var runner T
		if defaulter, ok := any(runner).(Defaulter); ok {
			runner = any(defaulter.Default()).(T)
		}

		if err := bindArgsToStruct(args, &runner); err != nil {
			return err
		}

		return runner.Run()
	}, nil
}
