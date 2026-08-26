package suzume

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"reflect"
	"strings"
)

var (
	ErrInvalidArgument = errors.New("invalid argument")
	ErrNilContext      = errors.New("context cannot be nil")
	helpArgSpec        = argSpec{
		index:    -1,
		name:     "help",
		short:    "h",
		usage:    "Show this help message",
		typeInfo: reflect.TypeFor[bool](),
	}
)

type commandHandler func(ctx context.Context, args ...string) error

type defaultValuesProvider func() map[string]any

// CommandDefinition defines the lifecycle methods required by UseCommand.
type CommandDefinition interface {
	// Default sets the command definition's default values. It is also called when
	// generating help and may be called more than once, so it must be idempotent and
	// must not have side effects beyond assigning default values.
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
	name          string
	aliases       []string
	description   string
	handler       commandHandler
	argSpecs      []argSpec
	defaultValues defaultValuesProvider
	config        *Config
}

type argSpec struct {
	index          int
	name           string
	short          string
	usage          string
	fieldName      string
	defaultText    string
	hasDefaultText bool
	typeInfo       reflect.Type
}

// NewCommand creates a new Executable with the given name, description, and handler function.
// The handler may accept supported positional argument types and context.Context, and must return
// either no values or a single error.
// Command names beginning with a hyphen are rejected.
func NewCommand(name, description string, runFunc any) (*Executable, error) {
	if name == "" {
		return nil, fmt.Errorf("Command name cannot be empty")
	}
	if err := validateIdentifier(name); err != nil {
		return nil, err
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
// Argument identifiers and positional indexes must be unique, and positional indexes must be non-negative.
// Command names beginning with a hyphen are rejected.
func UseCommand[T CommandDefinition](name, description string) (*Executable, error) {
	if name == "" {
		return nil, fmt.Errorf("Command name cannot be empty")
	}
	if err := validateIdentifier(name); err != nil {
		return nil, err
	}

	argSpecs, handler, defaultValues, err := createCommandHandler[T]()
	if err != nil {
		return nil, err
	}

	return &Executable{
		name:          name,
		description:   description,
		handler:       handler,
		argSpecs:      argSpecs,
		defaultValues: defaultValues,
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
// Alias names beginning with a hyphen are rejected.
func (cmd *Executable) Alias(name string) error {
	if name == "" {
		return nil
	}
	if err := validateIdentifier(name); err != nil {
		return err
	}

	cmd.aliases = append(cmd.aliases, name)
	return nil
}

// SetConfig sets the command's Config.
// This configuration will be used when the command is executed, and it can override the configuration inherited from the parent application.
func (cmd *Executable) SetConfig(configuration Config) {
	cmd.config = &configuration
}

// RunContext executes the command with the given context and arguments.
func (cmd *Executable) RunContext(ctx context.Context, args ...string) error {
	return cmd.runContext(ctx, nil, cmd.name, args...)
}

func (cmd *Executable) runContext(ctx context.Context, inheritedConfig *Config, commandPath string, args ...string) error {
	if ctx == nil {
		return ErrNilContext
	}

	config := materializeConfig(cmd.resolveConfig(inheritedConfig))
	if args == nil {
		args = os.Args[1:]
	}

	showHelp, invalidHelpArg := inspectHelpArgs(args)
	if invalidHelpArg != "" {
		return cmd.handleRunError(unknownOptionError(invalidHelpArg), config, commandPath)
	}
	if showHelp {
		cmd.showHelp(config, commandPath)
		return nil
	}

	var cmdCtx context.Context

	if len(config.handleSignals) > 0 {
		c, stop := signal.NotifyContext(ctx, config.handleSignals...)
		defer stop()
		cmdCtx = c
	} else {
		c, cancel := context.WithCancel(ctx)
		defer cancel()
		cmdCtx = c
	}

	err := cmd.handler(cmdCtx, args...)
	if err != nil {
		return cmd.handleRunError(err, config, commandPath)
	}

	return nil
}

func (cmd *Executable) handleRunError(err error, config Config, commandPath string) error {
	if errors.Is(err, ErrInvalidArgument) {
		fmt.Fprintln(config.errorLog, err)
		cmd.showHelp(config, commandPath)
	}
	return err
}

func inspectHelpArgs(args []string) (showHelp bool, invalidArg string) {
	for _, arg := range args {
		if arg == "--" {
			break
		}
		if isValuedHelpArg(arg) {
			return false, arg
		}
		if arg == "--help" || arg == "-h" {
			showHelp = true
		}
	}
	return showHelp, ""
}

func isValuedHelpArg(arg string) bool {
	return strings.HasPrefix(arg, "--help=") || strings.HasPrefix(arg, "-h=")
}

func unknownOptionError(arg string) error {
	return fmt.Errorf("%w: unknown option %q", ErrInvalidArgument, arg)
}

func (cmd *Executable) resolveConfig(inheritedConfig *Config) *Config {
	if cmd.config != nil {
		return cmd.config
	}
	return inheritedConfig
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
