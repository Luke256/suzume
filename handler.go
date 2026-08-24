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
func createCommandHandler[T CommandDefinition]() ([]argSpec, commandHandler, defaultValuesProvider, error) {
	commandDefinitionType := reflect.TypeFor[T]()
	if commandDefinitionType.Kind() != reflect.Pointer || commandDefinitionType.Elem().Kind() != reflect.Struct {
		return nil, nil, nil, fmt.Errorf("Command definition type must be a pointer to a struct: %v", commandDefinitionType)
	}
	structType := commandDefinitionType.Elem()

	commandType := reflect.TypeFor[Command]()
	argSpecs := make([]argSpec, 0, structType.NumField()+1)

	for i := range structType.NumField() {
		field := structType.Field(i)

		if field.Anonymous && field.Type == commandType {
			continue
		}
		if !field.IsExported() {
			continue
		}

		var aspec argSpec

		if idx, err := strconv.Atoi(field.Tag.Get("cli")); err == nil {
			// positional argument

			if field.Type.Kind() == reflect.Slice {
				return nil, nil, nil, fmt.Errorf("slice fields cannot be used as positional arguments: %s", field.Name)
			}

			aspec = argSpec{
				index:     idx,
				name:      pascalToKebab(field.Name),
				usage:     field.Tag.Get("usage"),
				fieldName: field.Name,
				typeInfo:  field.Type,
			}
		} else {
			// optional argument

			aspec = argSpec{
				index:     optionsIndex,
				name:      field.Tag.Get("cli"),
				short:     field.Tag.Get("short"),
				usage:     field.Tag.Get("usage"),
				fieldName: field.Name,
				typeInfo:  field.Type,
			}
			aspec.defaultText, aspec.hasDefaultText = field.Tag.Lookup("default")

			if aspec.name == "" {
				aspec.name = pascalToKebab(field.Name)
			}
		}

		argSpecs = append(argSpecs, aspec)
	}

	argSpecs = append(argSpecs, helpArgSpec)

	sortArgSpecs(argSpecs)

	newRunner := func() (T, reflect.Value) {
		runnerPointer := reflect.New(structType)
		runner := runnerPointer.Interface().(T)
		runner.Default()
		return runner, runnerPointer.Elem()
	}

	optionFields := make([]string, 0)
	for _, aspec := range argSpecs {
		if aspec.index == optionsIndex && aspec.fieldName != "" &&
			aspec.typeInfo.Kind() != reflect.Bool && !aspec.hasDefaultText {
			optionFields = append(optionFields, aspec.fieldName)
		}
	}

	var defaultValues defaultValuesProvider
	if len(optionFields) > 0 {
		defaultValues = func() map[string]any {
			_, runnerValue := newRunner()
			values := make(map[string]any, len(optionFields))
			for _, fieldName := range optionFields {
				values[fieldName] = runnerValue.FieldByName(fieldName).Interface()
			}
			return values
		}
	}

	return argSpecs, func(ctx context.Context, args ...string) error {
		if err := bindArgsToValues(args, argSpecs); err != nil {
			return err
		}

		runner, runnerValue := newRunner()
		for _, aspec := range argSpecs {
			if aspec.value.IsValid() {
				runnerValue.FieldByName(aspec.fieldName).Set(aspec.value)
			}
		}

		return runner.Run(ctx)
	}, defaultValues, nil
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
