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

type commandHandler func(args ...string) error

// Runner is an interface that defines a Run method, which is used for commands that can be executed.
type Runner interface {
	Run() error
}

// Defaulter is an interface that defines a Default method, which can be used to provide default values for command arguments.
type Defaulter interface {
	Default() Defaulter
}

// Command represents a command in the CLI application, including its name, description, handler function, argument specifications, and configuration.
type Command struct {
	name        string
	aliases     []string
	description string
	handler     commandHandler
	argSpecs    []argSpec
	config      Config
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

// NewCommand creates a new Command with the given name, description, and handler function.
// The handler function can be any function that takes zero or more arguments and returns an error.
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
		config: defaultConfig(),
	}, nil
}

// UseCommand creates a new Command based on a Runner type.
// It uses reflection to create a handler function that calls the Run method of the Runner, and it generates argument specifications based on the fields of the Runner struct.
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
		config: defaultConfig(),
	}, nil
}

// Alias adds an alias for the command. If the alias name is empty, it is ignored.
func (cmd *Command) Alias(name string) *Command {
	if name == "" {
		return cmd
	}

	cmd.aliases = append(cmd.aliases, name)
	return cmd
}

// SetConfig sets the configuration for the command.
// This configuration will be used when the command is executed, and it can override the configuration inherited from the parent application.
func (cmd *Command) SetConfig(config Config) {
	cmd.config = config
}

// Run executes the command with the given arguments.
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
			fmt.Fprintln(cmd.config.ErrorLog, err)
			cmd.showHelp()
		}
		return err
	}

	return nil
}

// RunAndExit executes the command with the given arguments and exits the program with a non-zero status code if an error occurs.
func (cmd *Command) RunAndExit(args ...string) {
	if err := cmd.Run(args...); err != nil {
		os.Exit(1)
	}
}