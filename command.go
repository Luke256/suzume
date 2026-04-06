package suzume

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"reflect"
	"slices"
	"syscall"
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

type commandHandler func(ctx context.Context, args ...string) error

// Runner is an interface that defines a Run method, which is used for commands that can be executed.
type Runner interface {
	Run(context.Context) error
}

// Defaulter is an interface that defines a Default method, which can be used to provide default values for command arguments.
type Defaulter[T Runner] interface {
	Default() T
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
		config:      defaultConfig(),
	}, nil
}

// MustNewCommand is a helper function that creates a new Command and panics if an error occurs.
// It is useful for cases where the command definition is static and should not fail at runtime.
func MustNewCommand(name, description string, runFunc any) *Command {
	cmd, err := NewCommand(name, description, runFunc)
	if err != nil {
		panic(err)
	}
	return cmd
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
		config:      defaultConfig(),
	}, nil
}

// MustUseCommand is a helper function that creates a new Command based on a Runner type and panics if an error occurs.
// It is useful for cases where the command definition is static and should not fail at runtime.
func MustUseCommand[T Runner](name, description string) *Command {
	cmd, err := UseCommand[T](name, description)
	if err != nil {
		panic(err)
	}
	return cmd
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

// RunContext executes the command with the given context and arguments.
func (cmd *Command) RunContext(ctx context.Context, args ...string) error {
	if ctx == nil {
		return fmt.Errorf("Context cannot be nil")
	}

	if args == nil {
		args = os.Args[1:]
	}

	if slices.Contains(args, "--help") || slices.Contains(args, "-h") {
		cmd.showHelp()
		return nil
	}

	cmdCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	err := cmd.handler(cmdCtx, args...)

	if err != nil {
		if errors.Is(err, ErrInvalidArgument) {
			fmt.Fprintln(cmd.config.ErrorLog, err)
			cmd.showHelp()
		}
		return err
	}

	return nil
}

// RunContextAndExit executes the command with the given context and arguments and exits the program with a non-zero status code if an error occurs.
func (cmd *Command) RunContextAndExit(ctx context.Context, args ...string) {
	if err := cmd.RunContext(ctx, args...); err != nil {
		os.Exit(1)
	}
}

// Run executes the command with a background context and the given arguments.
func (cmd *Command) Run(args ...string) error {
	return cmd.RunContext(newContext(), args...)
}

// RunAndExit executes the command with a background context and the given arguments and exits the program with a non-zero status code if an error occurs.
func (cmd *Command) RunAndExit(args ...string) {
	cmd.RunContextAndExit(newContext(), args...)
}

func newContext() context.Context {
	return context.Background()
}
