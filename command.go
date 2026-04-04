package mycli

import (
	"encoding"
	"errors"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

const ()

var (
	ErrInvalidArgument = errors.New("invalid argument")
	helpArgSpec        = argSpec{
		index: -1,
		name:  "help",
		short: "h",
		usage: "Show this help message",
	}
)

type Runner interface {
	Run() error
}

type Defaulter interface {
	Default() Defaulter
}

type Command struct {
	name        string
	aliases     []string
	description string
	handler     func(args ...string) error
	argSpecs    []argSpec
}

type argSpec struct {
	index int
	name  string
	short string
	usage string
}

func NewCommand(name, description string, runFunc any) (*Command, error) {
	if name == "" {
		return nil, fmt.Errorf("Command name cannot be empty")
	}

	argSpecs, handler := createFunctionHandler(runFunc)
	return &Command{
		name:        name,
		description: description,
		handler:     handler,
		argSpecs:    argSpecs,
	}, nil
}

func UseCommand[T Runner](name, description string) (*Command, error) {
	if name == "" {
		return nil, fmt.Errorf("Command name cannot be empty")
	}

	argSpecs, handler, err := createRunnerHandler[T]()
	if err != nil {
		return nil, err
	}

	return &Command{
		name:        name,
		description: description,
		handler:     handler,
		argSpecs:    argSpecs,
	}, nil
}

func (cmd *Command) Alias(name string) *Command {
	cmd.aliases = append(cmd.aliases, name)
	return cmd
}

func (cmd *Command) Run(args ...string) error {
	if args == nil {
		args = os.Args[1:]
	}

	// ヘルプ
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h") {
		cmd.showHelp()
		return nil
	}

	err := cmd.handler(args...)
	if err != nil {
		if errors.Is(err, ErrInvalidArgument) {
			fmt.Printf("%v\n", err)
			cmd.showHelp()
		}
		return err
	}
	return nil
}

func (cmd *Command) showHelp() {
	// argSpecsをソート
	sort.Slice(cmd.argSpecs, func(i, j int) bool {
		if cmd.argSpecs[i].index == -1 {
			return false
		}
		if cmd.argSpecs[j].index == -1 {
			return true
		}
		return cmd.argSpecs[i].index < cmd.argSpecs[j].index
	})

	var numArguments int
	var numOptions int

	fmt.Printf("Usage: %s", cmd.name)
	for _, arg := range cmd.argSpecs {
		if arg.index != -1 {
			fmt.Printf(" <%s>", arg.name)
			numArguments++
		} else {
			if arg.short != "" {
				fmt.Printf(" [-%s|--%s]", arg.short, arg.name)
			} else if arg.name != "" {
				fmt.Printf(" [--%s]", arg.name)
			}
			numOptions++
		}
	}
	fmt.Println()

	if cmd.description != "" {
		fmt.Println(cmd.description)
	}

	if numArguments > 0 {
		fmt.Println("\nArguments:")
		for _, arg := range cmd.argSpecs {
			if arg.index != -1 {
				fmt.Printf("  %s\t%s\n", arg.name, arg.usage)
			}
		}
	}

	if numOptions > 0 {
		fmt.Println("\nOptions:")
		for _, arg := range cmd.argSpecs {
			if arg.index == -1 {
				if arg.short != "" {
					fmt.Printf("  -%s, --%s\t%s\n", arg.short, arg.name, arg.usage)
				} else if arg.name != "" {
					fmt.Printf("      --%s\t%s\n", arg.name, arg.usage)
				}
			}
		}
	}
}

// 固定引数の関数をコマンドのハンドラー
// args: ["arg1", "arg2", ...]
func createFunctionHandler(runFunc any) ([]argSpec, func(args ...string) error) {
	v := reflect.ValueOf(runFunc)
	if v.Kind() != reflect.Func {
		panic(fmt.Sprintf("Expected a function, got %T (%v)", runFunc, runFunc))
	}

	numArgs := v.Type().NumIn()

	argSpecs := make([]argSpec, numArgs+1)

	for i := range numArgs {
		// 固定引数としてスライスは受け付けない
		if v.Type().In(i).Kind() == reflect.Slice {
			panic(fmt.Sprintf("Slice fields cannot be used as positional arguments: argument %d", i+1))
		}
		// Boolはフラグとして受け取るべき
		if v.Type().In(i).Kind() == reflect.Bool {
			panic("Boolean arguments should be handled as flags")
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
			return nil
		}

		return nil
	}
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
			// 固定引数としてスライスは受け付けない
			if field.Type.Kind() == reflect.Slice {
				panic(fmt.Sprintf("Slice fields cannot be used as positional arguments: %s", field.Name))
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

		// コマンドライン引数でrunnerのフィールドを上書き
		if err := bindArgsToStruct(args, &runner); err != nil {
			return err
		}

		return runner.Run()
	}, nil
}

func bindArgsToStruct[T any](args []string, runner *T) error {
	v := reflect.ValueOf(runner).Elem()

	type optionState struct {
		field    reflect.Value
		flag	 string
		resolved bool
	}

	var optionFor optionState
	var positionalIndex int
	var numPositional int
	for i := range v.NumField() {
		if _, err := strconv.Atoi(v.Type().Field(i).Tag.Get("cli")); err == nil {
			numPositional++
		}
	}

	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			if parts := strings.SplitN(arg, "=", 2); len(parts) == 2 {
				// --opt=value
				field, ok := findFieldByFlag(v, parts[0])
				if !ok {
					return fmt.Errorf("%w: unknown option %q", ErrInvalidArgument, parts[0])
				}
				if field.Type().Kind() == reflect.Bool {
					return fmt.Errorf("%w: flag %q should not have a value", ErrInvalidArgument, parts[0])
				}
				argValue, err := parseArg(parts[1], field.Type())
				if err != nil {
					return fmt.Errorf("%w: option %q: %v", ErrInvalidArgument, parts[0], err)
				}
				field.Set(argValue)
			} else {
				// --opt value or -o value
				field, ok := findFieldByFlag(v, arg)
				if !ok {
					return fmt.Errorf("%w: unknown option %q", ErrInvalidArgument, arg)
				}
				// ブールフラグは値を取らない
				if field.Type().Kind() == reflect.Bool {
					field.Set(reflect.ValueOf(true))
				} else {
					optionFor.field = field
					optionFor.flag = arg
					optionFor.resolved = false
				}
			}
		} else if optionFor.field.IsValid() {
			// オプション
			if optionFor.field.Type().Kind() == reflect.Slice {
				// スライスの中身として追加
				elemType := optionFor.field.Type().Elem()
				argValue, err := parseArg(arg, elemType)
				if err != nil {
					return fmt.Errorf("%w: option %q: %v", ErrInvalidArgument, optionFor.field.Type().Name(), err)
				}
				optionFor.field.Set(reflect.Append(optionFor.field, argValue))
			} else {
				// 単一の値としてセット
				argValue, err := parseArg(arg, optionFor.field.Type())
				if err != nil {
					return fmt.Errorf("%w: option %q: %v", ErrInvalidArgument, optionFor.field.Type().Name(), err)
				}
				optionFor.field.Set(argValue)
				optionFor.field = reflect.Value{}
				optionFor.resolved = true
			}
		} else {
			// 位置引数
			field, ok := findFieldByPositionalIndex(v, positionalIndex)
			if !ok {
				return fmt.Errorf("%w: unexpected positional argument %q", ErrInvalidArgument, arg)
			}
			argValue, err := parseArg(arg, field.Type())
			if err != nil {
				return fmt.Errorf("%w: argument %d: %v", ErrInvalidArgument, positionalIndex, err)
			}
			field.Set(argValue)
			positionalIndex++
		}
	}

	if positionalIndex < numPositional {
		return fmt.Errorf("%w: expected %d positional arguments, got %d", ErrInvalidArgument, numPositional, positionalIndex)
	}

	if optionFor.field.IsValid() && !optionFor.resolved {
		return fmt.Errorf("%w: option %q requires a value", ErrInvalidArgument, optionFor.flag)
	}

	return nil
}

func findFieldByFlag(v reflect.Value, flag string) (reflect.Value, bool) {
	flag = strings.TrimLeft(flag, "-")

	for i := range v.NumField() {
		field := v.Type().Field(i)
		if field.Tag.Get("cli") == flag || field.Tag.Get("short") == flag {
			return v.Field(i), true
		}
	}

	return reflect.Value{}, false
}

// cliタグに対応する整数が指定されているフィールド
func findFieldByPositionalIndex(v reflect.Value, index int) (reflect.Value, bool) {
	for i := range v.NumField() {
		field := v.Type().Field(i)
		if idx, err := strconv.Atoi(field.Tag.Get("cli")); err == nil && idx == index {
			return v.Field(i), true
		}
	}
	return reflect.Value{}, false
}

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
	default:
		if argType.Implements(reflect.TypeFor[encoding.TextUnmarshaler]()) {
			v := reflect.New(argType).Interface().(encoding.TextUnmarshaler)
			if err := v.UnmarshalText([]byte(arg)); err != nil {
				return reflect.Value{}, fmt.Errorf("failed to parse argument: %v", err)
			}
			return reflect.ValueOf(v).Elem(), nil
		}
		panic(fmt.Sprintf("Unsupported argument type: %v", argType))
	}
}
