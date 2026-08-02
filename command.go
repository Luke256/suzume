package suzume

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
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

type commandHandler func(ctx context.Context, args ...string) error

// CommandDefinition defines the lifecycle methods required by UseCommand.
type CommandDefinition interface {
	Default()
	Run(context.Context) error
}

// Command provides optional default behavior for command definitions used with UseCommand.
// Embed Command when a command does not need its own Default or Run implementation.
type Command struct{}

// Default leaves the command definition's zero values unchanged.
func (*Command) Default() {}

// Run performs no action.
func (*Command) Run(context.Context) error {
	return nil
}

// Executable represents a configured command in the CLI application.
type Executable struct {
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

// NewCommand creates a new Executable with the given name, description, and handler function.
// The handler function can be any function that takes zero or more arguments and returns an error.
func NewCommand(name, description string, runFunc any) (*Executable, error) {
	if name == "" {
		return nil, fmt.Errorf("Command name cannot be empty")
	}

	argSpecs, handler, err := createFunctionHandler(runFunc)
	if err != nil {
		return nil, err
	}

	return &Executable{
		name:        name,
		description: description,
		handler:     handler,
		argSpecs:    argSpecs,
		config:      defaultConfig(),
	}, nil
}

// MustNewCommand creates a new Executable and panics if an error occurs.
// It is useful for cases where the command definition is static and should not fail at runtime.
func MustNewCommand(name, description string, runFunc any) *Executable {
	cmd, err := NewCommand(name, description, runFunc)
	if err != nil {
		panic(err)
	}
	return cmd
}

// UseCommand creates a new Executable based on a CommandDefinition type.
// Its exported fields are used to generate argument specifications.
func UseCommand[T CommandDefinition](name, description string) (*Executable, error) {
	if name == "" {
		return nil, fmt.Errorf("Command name cannot be empty")
	}

	argSpecs, handler, err := createCommandHandler[T]()
	if err != nil {
		return nil, err
	}

	return &Executable{
		name:        name,
		description: description,
		handler:     handler,
		argSpecs:    argSpecs,
		config:      defaultConfig(),
	}, nil
}

// MustUseCommand creates a new Executable based on a command definition and panics if an error occurs.
func MustUseCommand[T CommandDefinition](name, description string) *Executable {
	cmd, err := UseCommand[T](name, description)
	if err != nil {
		panic(err)
	}
	return cmd
}

// Alias adds an alias for the command. If the alias name is empty, it is ignored.
func (cmd *Executable) Alias(name string) *Executable {
	if name == "" {
		return cmd
	}

	cmd.aliases = append(cmd.aliases, name)
	return cmd
}

// SetConfig sets the configuration for the command.
// This configuration will be used when the command is executed, and it can override the configuration inherited from the parent application.
func (cmd *Executable) SetConfig(config Config) {
	cmd.config = config
}

// RunContext executes the command with the given context and arguments.
func (cmd *Executable) RunContext(ctx context.Context, args ...string) error {
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

	var cmdCtx context.Context

	if len(cmd.config.IgnoreSignals) > 0 {
		c, stop := signal.NotifyContext(ctx, cmd.config.IgnoreSignals...)
		defer stop()
		cmdCtx = c
	} else {
		c, cancel := context.WithCancel(ctx)
		defer cancel()
		cmdCtx = c
	}

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
func (cmd *Executable) RunContextAndExit(ctx context.Context, args ...string) {
	if err := cmd.RunContext(ctx, args...); err != nil {
		os.Exit(1)
	}
}

// Run executes the command with a background context and the given arguments.
func (cmd *Executable) Run(args ...string) error {
	return cmd.RunContext(newContext(), args...)
}

// RunAndExit executes the command with a background context and the given arguments and exits the program with a non-zero status code if an error occurs.
func (cmd *Executable) RunAndExit(args ...string) {
	cmd.RunContextAndExit(newContext(), args...)
}

func newContext() context.Context {
	return context.Background()
}
