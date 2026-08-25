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

// pascalToKebab converts a PascalCase string to kebab-case.
// For example, "HelloWorld" becomes "hello-world".
// APIKey -> api-key
func pascalToKebab(s string) string {
	runes := []rune(s)
	var sb strings.Builder

	for i, r := range runes {
		if i > 0 && unicode.IsUpper(r) {
			prev := runes[i-1]

			var next rune
			hasNext := i+1 < len(runes)
			if hasNext {
				next = runes[i+1]
			}

			if unicode.IsLower(prev) ||
			   unicode.IsDigit(prev) ||
				(unicode.IsUpper(prev) && hasNext && unicode.IsLower(next)) {
				sb.WriteRune('-')
			}
		}

		sb.WriteRune(unicode.ToLower(r))
	}

	return sb.String()
}

// 固定引数の関数をコマンドのハンドラー
// args: ["arg1", "arg2", ...]
func createFunctionHandler(runFunc any) ([]argSpec, commandHandler, error) {
	runValue := reflect.ValueOf(runFunc)
	if !runValue.IsValid() {
		return nil, nil, fmt.Errorf("runFunc cannot be nil")
	}
	if runValue.Kind() != reflect.Func {
		return nil, nil, fmt.Errorf("runFunc must be a function")
	}
	if runValue.IsNil() {
		return nil, nil, fmt.Errorf("runFunc cannot be nil")
	}

	v := runValue.Type()
	errorType := reflect.TypeFor[error]()
	if v.NumOut() > 1 || v.NumOut() == 1 && v.Out(0) != errorType {
		return nil, nil, fmt.Errorf("runFunc must return no values or a single error")
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
			if !supportsArgumentType(arg) {
				return nil, nil, fmt.Errorf("unsupported function handler argument type %v: argument %d", arg, i+1)
			}
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
		values, err := bindArgsToValues(args, argSpecs)
		if err != nil {
			return err
		}

		in := make([]reflect.Value, v.NumIn())
		for i := range v.NumIn() {
			if v.In(i) == reflect.TypeFor[context.Context]() {
				in[i] = reflect.ValueOf(ctx)
			}
		}

		for i, aspec := range argSpecs {
			if aspec.index >= 0 {
				in[aspec.index] = values[i]
			}
		}

		out := runValue.Call(in)
		if len(out) == 1 {
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
	identifiers := map[string]struct{}{
		helpArgSpec.name:  {},
		helpArgSpec.short: {},
	}

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
			if idx < 0 {
				return nil, nil, nil, fmt.Errorf("invalid positional argument index %d for field %s", idx, field.Name)
			}

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

		aliases := []string(nil)
		if aspec.short != "" {
			if aspec.short == aspec.name {
				return nil, nil, nil, fmt.Errorf("invalid argument tags for field %s: %w: %s", field.Name, ErrDuplicateIdentifier, aspec.name)
			}
			aliases = append(aliases, aspec.short)
		}
		if err := registerIdentifiers(identifiers, aspec.name, aliases...); err != nil {
			return nil, nil, nil, fmt.Errorf("invalid argument tags for field %s: %w", field.Name, err)
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
		values, err := bindArgsToValues(args, argSpecs)
		if err != nil {
			return err
		}

		runner, runnerValue := newRunner()
		for i, aspec := range argSpecs {
			if values[i].IsValid() {
				runnerValue.FieldByName(aspec.fieldName).Set(values[i])
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
