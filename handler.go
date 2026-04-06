package suzume

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

const (
	optionsIndex = -1
	contextIndex = -2
)

func pascalToKebab(s string) string {
	var result []string
	for i, r := range s {
		if i > 0 && unicode.IsUpper(r) {
			result = append(result, "-")
		}
		result = append(result, string(unicode.ToLower(r)))
	}
	return strings.Join(result, "")
}

// 固定引数の関数をコマンドのハンドラー
// args: ["arg1", "arg2", ...]
func createFunctionHandler(runFunc any) ([]argSpec, commandHandler, error) {
	v := reflect.TypeOf(runFunc)
	if v.Kind() != reflect.Func {
		return nil, nil, fmt.Errorf("runFunc must be a function")
	}

	argSpecs := make([]argSpec, v.NumIn()+1)
	argSpecs[v.NumIn()] = helpArgSpec
	var argIndex int = 1

	for i := range v.NumIn() {
		arg := v.In(i)

		if arg.Kind() == reflect.Slice {
			return nil, nil, fmt.Errorf("slice arguments cannot be used in function handlers: argument %d", i+1)
		}

		if arg.Kind() == reflect.Bool {
			return nil, nil, fmt.Errorf("boolean arguments cannot be used in function handlers: argument %d", i+1)
		}

		if arg == reflect.TypeFor[context.Context]() {
			argSpecs[i] = argSpec{
				index:    contextIndex,
				name:     "",
				typeInfo: arg,
			}
		} else {
			argSpecs[i] = argSpec{
				index:    i,
				name:     fmt.Sprintf("arg%d", argIndex),
				typeInfo: arg,
			}
			argIndex++
		}
	}

	sortArgSpecs(argSpecs)

	return argSpecs, func(ctx context.Context, args ...string) error {
		if err := bindArgsToValues(args, argSpecs); err != nil {
			return err
		}

		in := make([]reflect.Value, v.NumIn())
		for i := range v.NumIn() {
			if v.In(i) == reflect.TypeFor[context.Context]() {
				in[i] = reflect.ValueOf(ctx)
			}
		}

		for _, aspec := range argSpecs {
			if aspec.index >= 0 {
				in[aspec.index] = aspec.value
			}
		}

		out := reflect.ValueOf(runFunc).Call(in)
		if len(out) == 1 && out[0].Type() == reflect.TypeFor[error]() {
			if !out[0].IsNil() {
				return out[0].Interface().(error)
			}
			return nil
		}
		return nil
	}, nil
}

// args: ["arg1", "arg2", ... , "--flag", "--opt=value", "--opt", "value", "-o", "value", ...]
func createRunnerHandler[T Runner]() ([]argSpec, commandHandler, error) {
	v := reflect.TypeFor[T]()

	// Tは構造体の値型でなければならない
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
				index:     idx,
				name:      pascalToKebab(field.Name),
				usage:     field.Tag.Get("usage"),
				fieldName: field.Name,
				typeInfo:  field.Type,
			}
		} else {
			argSpecs[i] = argSpec{
				index:     optionsIndex,
				name:      field.Tag.Get("cli"),
				short:     field.Tag.Get("short"),
				usage:     field.Tag.Get("usage"),
				fieldName: field.Name,
				typeInfo:  field.Type,
			}

			if argSpecs[i].name == "" {
				argSpecs[i].name = pascalToKebab(field.Name)
			}
		}
	}

	sortArgSpecs(argSpecs)

	return argSpecs, func(ctx context.Context, args ...string) error {
		var runner T
		if defaulter, ok := any(runner).(Defaulter[T]); ok {
			runner = any(defaulter.Default()).(T)
		}

		if err := bindArgsToValues(args, argSpecs); err != nil {
			return err
		}

		v := reflect.ValueOf(&runner)
		for _, aspec := range argSpecs {
			if aspec.value.IsValid() {
				v.Elem().FieldByName(aspec.fieldName).Set(aspec.value)
			}
		}

		return runner.Run(ctx)
	}, nil
}

func sortArgSpecs(argSpecs []argSpec) {
	sort.Slice(argSpecs, func(i, j int) bool {
		if argSpecs[i].index < 0 {
			return false
		}
		if argSpecs[j].index < 0 {
			return true
		}
		return argSpecs[i].index < argSpecs[j].index
	})
}
