package suzume

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

type optionState struct {
	field      reflect.Value
	flag       string
	valuesRead int
}

func (state *optionState) clear() {
	state.field = reflect.Value{}
	state.flag = ""
	state.valuesRead = 0
}

func bindArgsToStruct[T any](args []string, runner *T, argSpecs []argSpec) error {
	v := reflect.ValueOf(runner).Elem()

	var pendingOption optionState
	positionalIndex := 0
	numPositional := countPositionalFields(v)

	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			if err := finalizePendingOption(&pendingOption); err != nil {
				return err
			}
			if err := bindOptionArg(v, arg, &pendingOption, argSpecs); err != nil {
				return err
			}
			continue
		}

		if pendingOption.field.IsValid() {
			if err := assignOptionValue(arg, &pendingOption); err != nil {
				return err
			}
			continue
		}

		field, ok := findFieldByPositionalIndex(v, positionalIndex, argSpecs)
		if !ok {
			return fmt.Errorf("%w: unexpected positional argument %q", ErrInvalidArgument, arg)
		}

		argValue, err := parseArg(arg, field.Type())
		if err != nil {
			return fmt.Errorf("%w: argument %d: %v", ErrInvalidArgument, positionalIndex+1, err)
		}

		field.Set(argValue)
		positionalIndex++
	}

	if positionalIndex < numPositional {
		return fmt.Errorf("%w: expected %d positional arguments, got %d", ErrInvalidArgument, numPositional, positionalIndex)
	}

	if err := finalizePendingOption(&pendingOption); err != nil {
		return err
	}

	return nil
}

func countPositionalFields(v reflect.Value) int {
	count := 0
	for i := range v.NumField() {
		if _, err := strconv.Atoi(v.Type().Field(i).Tag.Get("cli")); err == nil {
			count++
		}
	}
	return count
}

func bindOptionArg(v reflect.Value, arg string, pendingOption *optionState, argSpecs []argSpec) error {
	if parts := strings.SplitN(arg, "=", 2); len(parts) == 2 {
		field, ok := findFieldByFlag(v, parts[0], argSpecs)
		if !ok {
			return fmt.Errorf("%w: unknown option %q", ErrInvalidArgument, parts[0])
		}
		if field.Type().Kind() == reflect.Bool {
			return fmt.Errorf("%w: flag %q should not have a value", ErrInvalidArgument, parts[0])
		}
		if field.Type().Kind() == reflect.Slice {
			elemType := field.Type().Elem()
			argValue, err := parseArg(parts[1], elemType)
			if err != nil {
				return fmt.Errorf("%w: option %q: %v", ErrInvalidArgument, parts[0], err)
			}
			field.Set(reflect.Append(field, argValue))
			return nil
		}
		argValue, err := parseArg(parts[1], field.Type())
		if err != nil {
			return fmt.Errorf("%w: option %q: %v", ErrInvalidArgument, parts[0], err)
		}
		field.Set(argValue)
		return nil
	}

	field, ok := findFieldByFlag(v, arg, argSpecs)
	if !ok {
		return fmt.Errorf("%w: unknown option %q", ErrInvalidArgument, arg)
	}

	if field.Type().Kind() == reflect.Bool {
		field.SetBool(true)
		return nil
	}

	pendingOption.field = field
	pendingOption.flag = arg
	pendingOption.valuesRead = 0
	return nil
}

func assignOptionValue(arg string, pendingOption *optionState) error {
	if pendingOption.field.Type().Kind() == reflect.Slice {
		elemType := pendingOption.field.Type().Elem()
		argValue, err := parseArg(arg, elemType)
		if err != nil {
			return fmt.Errorf("%w: option %q: %v", ErrInvalidArgument, pendingOption.flag, err)
		}
		pendingOption.field.Set(reflect.Append(pendingOption.field, argValue))
		pendingOption.valuesRead++
		return nil
	}

	argValue, err := parseArg(arg, pendingOption.field.Type())
	if err != nil {
		return fmt.Errorf("%w: option %q: %v", ErrInvalidArgument, pendingOption.flag, err)
	}
	pendingOption.field.Set(argValue)
	pendingOption.valuesRead = 1
	pendingOption.clear()
	return nil
}

func finalizePendingOption(pendingOption *optionState) error {
	if !pendingOption.field.IsValid() {
		return nil
	}
	if pendingOption.valuesRead == 0 {
		return fmt.Errorf("%w: option %q requires a value", ErrInvalidArgument, pendingOption.flag)
	}
	pendingOption.clear()
	return nil
}

func findFieldByFlag(v reflect.Value, flag string, argSpecs []argSpec) (reflect.Value, bool) {
	flag = strings.TrimLeft(flag, "-")

	for _, spec := range argSpecs {
		if spec.name == flag || spec.short == flag {
			return v.FieldByName(spec.fieldName), true
		}
	}

	return reflect.Value{}, false
}

// cliタグに対応する整数が指定されているフィールド
func findFieldByPositionalIndex(v reflect.Value, index int, argSpecs []argSpec) (reflect.Value, bool) {
	for _, spec := range argSpecs {
		if spec.index == index {
			return v.FieldByName(spec.fieldName), true
		}
	}
	return reflect.Value{}, false
}
