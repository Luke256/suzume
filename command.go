package suzume

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"slices"
)

var (
	ErrInvalidArgument = errors.New("invalid argument")
	helpArgSpec        = argSpec{
		index:    -1,
		name:     "help",
		short:    "h",
		usage:    "Show this help message",
		typeInfo: reflect.TypeFor[bool](),
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
	index     int
	name      string
	short     string
	usage     string
	fieldName string
	value     reflect.Value
	typeInfo  reflect.Type
}

func NewCommand(name, description string, runFunc any) (*Command, error) {
	if name == "" {
		return nil, fmt.Errorf("Command name cannot be empty")
	}

	argSpecs, handler, err := createFunctionHandler(runFunc)
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
	if name == "" {
		return cmd
	}

	cmd.aliases = append(cmd.aliases, name)
	return cmd
}

func (cmd *Command) Run(args ...string) error {
	if args == nil {
		args = os.Args[1:]
	}

	if slices.Contains(args, "--help") || slices.Contains(args, "-h") {
		cmd.showHelp()
		return nil
	}

	err := cmd.handler(args...)
	if err != nil {
		if errors.Is(err, ErrInvalidArgument) {
			fmt.Fprintln(os.Stderr, err)
			cmd.showHelp()
		}
		return err
	}

	return nil
}
