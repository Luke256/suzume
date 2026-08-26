package suzume

import (
	"encoding"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

func supportsArgumentType(argType reflect.Type) bool {
	if argType.Kind() == reflect.Interface {
		return false
	}

	textUnmarshalerType := reflect.TypeFor[encoding.TextUnmarshaler]()
	if reflect.PointerTo(argType).Implements(textUnmarshalerType) || argType.Implements(textUnmarshalerType) {
		return true
	}

	switch argType.Kind() {
	case reflect.String,
		reflect.Bool,
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
		if argType.Kind() == reflect.Pointer {
			value = reflect.New(argType.Elem())
		}
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

type argumentParser struct {
	args            []string
	specs           []argSpec
	values          []reflect.Value
	positionalSpecs []int
	argIndex        int
	positionalIndex int
	parseOptions    bool
}

// 引数列を、argSpecsに対応する実行ローカルなvaluesに割り当てる
func bindArgsToValues(args []string, argSpecs []argSpec) ([]reflect.Value, error) {
	parser := argumentParser{
		args:         args,
		specs:        argSpecs,
		values:       make([]reflect.Value, len(argSpecs)),
		parseOptions: true,
	}
	for i := range argSpecs {
		if argSpecs[i].index >= 0 {
			parser.positionalSpecs = append(parser.positionalSpecs, i)
		}
	}

	if err := parser.parse(); err != nil {
		return nil, err
	}
	return parser.values, nil
}

func (p *argumentParser) parse() error {
	for p.argIndex < len(p.args) {
		arg := p.args[p.argIndex]
		if p.parseOptions && arg == "--" {
			p.parseOptions = false
			p.argIndex++
			continue
		}

		specIndex, matched := p.matchOption(arg)
		if p.parseOptions && matched {
			if err := p.parseOption(specIndex); err != nil {
				return err
			}
			continue
		}

		if p.parseOptions && looksLikeOption(arg) && !p.canBindNegativeNumber(arg) {
			return unknownOptionError(arg)
		}
		if err := p.parsePositional(arg); err != nil {
			return err
		}
	}

	if p.positionalIndex < len(p.positionalSpecs) {
		spec := p.specs[p.positionalSpecs[p.positionalIndex]]
		return fmt.Errorf("%w: missing required positional argument: %s", ErrInvalidArgument, spec.name)
	}
	return nil
}

func (p *argumentParser) parseOption(specIndex int) error {
	spec := p.specs[specIndex]
	valueType := spec.typeInfo
	if valueType.Kind() == reflect.Slice {
		valueType = valueType.Elem()
	}

	option, inlineValue, hasInlineValue := strings.Cut(p.args[p.argIndex], "=")
	if hasInlineValue {
		value, err := parseOptionValue(inlineValue, valueType, option)
		if err != nil {
			return err
		}
		p.values[specIndex] = appendOptionValue(p.values[specIndex], spec.typeInfo, value)
		p.argIndex++
		return nil
	}
	if spec.typeInfo.Kind() == reflect.Bool {
		value := reflect.New(spec.typeInfo).Elem()
		value.SetBool(true)
		p.values[specIndex] = value
		p.argIndex++
		return nil
	}
	if spec.typeInfo.Kind() == reflect.Slice {
		return p.parseSliceOption(specIndex, valueType, option)
	}

	nextIndex := p.argIndex + 1
	if nextIndex >= len(p.args) || !p.canConsumeOptionValue(p.args[nextIndex], valueType) {
		return missingOptionValueError(spec)
	}
	value, err := parseOptionValue(p.args[nextIndex], valueType, option)
	if err != nil {
		return err
	}
	p.values[specIndex] = value
	p.argIndex = nextIndex + 1
	return nil
}

func (p *argumentParser) parseSliceOption(specIndex int, valueType reflect.Type, option string) error {
	spec := p.specs[specIndex]
	p.argIndex++
	valueCount := 0
	for p.argIndex < len(p.args) && p.canConsumeOptionValue(p.args[p.argIndex], valueType) {
		value, err := parseOptionValue(p.args[p.argIndex], valueType, option)
		if err != nil {
			return err
		}
		p.values[specIndex] = appendOptionValue(p.values[specIndex], spec.typeInfo, value)
		p.argIndex++
		valueCount++
	}
	if valueCount == 0 {
		return missingOptionValueError(spec)
	}
	return nil
}

func (p *argumentParser) parsePositional(arg string) error {
	if p.positionalIndex >= len(p.positionalSpecs) {
		return fmt.Errorf("%w: unexpected positional argument %q", ErrInvalidArgument, arg)
	}

	specIndex := p.positionalSpecs[p.positionalIndex]
	value, err := parseArg(arg, p.specs[specIndex].typeInfo)
	if err != nil {
		return fmt.Errorf("%w: failed to parse argument %d: %w", ErrInvalidArgument, p.positionalIndex+1, err)
	}
	p.values[specIndex] = value
	p.positionalIndex++
	p.argIndex++
	return nil
}

// matchOption はargに該当するオプションのインデックスを返します
func (p *argumentParser) matchOption(arg string) (int, bool) {
	var name string
	long := strings.HasPrefix(arg, "--")
	if long {
		name = strings.TrimPrefix(arg, "--")
	} else if strings.HasPrefix(arg, "-") && arg != "-" {
		name = strings.TrimPrefix(arg, "-")
	} else {
		return 0, false
	}

	name, _, _ = strings.Cut(name, "=")
	for i, spec := range p.specs {
		if spec.index != optionsIndex {
			continue
		}
		if long && spec.name == name || !long && spec.short != "" && spec.short == name {
			return i, true
		}
	}
	return 0, false
}

func (p *argumentParser) canConsumeOptionValue(arg string, valueType reflect.Type) bool {
	if arg == "--" {
		return false
	}
	if _, ok := p.matchOption(arg); ok {
		return false
	}
	if !looksLikeOption(arg) {
		return true
	}
	return !strings.HasPrefix(arg, "--") && isNumber(valueType)
}

func (p *argumentParser) canBindNegativeNumber(arg string) bool {
	if p.positionalIndex >= len(p.positionalSpecs) {
		return false
	}
	typeInfo := p.specs[p.positionalSpecs[p.positionalIndex]].typeInfo
	return !strings.HasPrefix(arg, "--") && isNumber(typeInfo)
}

func parseOptionValue(arg string, valueType reflect.Type, option string) (reflect.Value, error) {
	value, err := parseArg(arg, valueType)
	if err != nil {
		return reflect.Value{}, fmt.Errorf("%w: failed to parse option %q: %w", ErrInvalidArgument, option, err)
	}
	return value, nil
}

func missingOptionValueError(spec argSpec) error {
	return fmt.Errorf("%w: missing value for option: %s", ErrInvalidArgument, spec.name)
}

func isNumber(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128:
		return true
	default:
		return false
	}
}

func appendOptionValue(current reflect.Value, optionType reflect.Type, value reflect.Value) reflect.Value {
	if optionType.Kind() != reflect.Slice {
		return value
	}
	if !current.IsValid() {
		current = reflect.MakeSlice(optionType, 0, 1)
	}
	return reflect.Append(current, value)
}

func looksLikeOption(arg string) bool {
	return len(arg) > 1 && arg[0] == '-'
}
