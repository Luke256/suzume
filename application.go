package suzume

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
)

var (
	ErrCommandNotFound = errors.New("Command not found")
)

// App represents a CLI App that can contain commands and sub-applications.
type App struct {
	// appPath is the path of parent applications leading to this app, used for help message generation.
	appPath []string

	// name is the name of the application, used for command matching and help message generation.
	name string

	// aliases are alternative names for the application, used for command matching.
	aliases []string

	// description is a brief description of the application, shown in the help message.
	description string

	// commands is the list of commands directly under this application.
	commands []Executable

	// apps is the list of sub-applications directly under this application.
	apps []App

	// config holds an explicitly assigned configuration. A nil value inherits from the parent.
	config *Config

	// identifiers holds the set of identifiers for this application, used for collision detection.
	identifiers map[string]struct{}
}

// NewApp creates a new App with the given name and description.
// App names beginning with a hyphen are rejected.
func NewApp(name, description string) (*App, error) {
	if err := validateIdentifier(name); err != nil {
		return nil, err
	}

	return &App{
		name:        name,
		description: description,
		identifiers: map[string]struct{}{
			"help": {},
		},
	}, nil
}

// MustNewApp creates a new App and panics if an error occurs.
func MustNewApp(name, description string) *App {
	app, err := NewApp(name, description)
	if err != nil {
		panic(err)
	}
	return app
}

// AddCommand adds a value copy of a command to the application. If the command is nil, it is ignored.
// Its registration identifiers are fixed at the time of the call.
// It returns ErrDuplicateIdentifier when an identifier is already registered.
func (app *App) AddCommand(cmd *Executable) error {
	if cmd == nil {
		return nil
	}

	if err := registerIdentifiers(app.identifiers, cmd.name, cmd.aliases...); err != nil {
		return fmt.Errorf("failed to register command: %w", err)
	}

	registered := *cmd
	app.commands = append(app.commands, registered)

	return nil
}

// AddApp adds a value copy of a sub-application. If the sub-application is nil, it is ignored.
// Its registration identifiers are fixed at the time of the call.
// It returns ErrDuplicateIdentifier when an identifier is already registered.
func (app *App) AddApp(subApp *App) error {
	if subApp == nil {
		return nil
	}

	if err := registerIdentifiers(app.identifiers, subApp.name, subApp.aliases...); err != nil {
		return fmt.Errorf("failed to register app: %w", err)
	}

	registered := *subApp
	app.apps = append(app.apps, registered)
	return nil
}

// Alias adds an alias for the application. If the alias name is empty, it is ignored.
// Alias names beginning with a hyphen are rejected.
func (app *App) Alias(name string) error {
	if name == "" {
		return nil
	}
	if err := validateIdentifier(name); err != nil {
		return err
	}

	app.aliases = append(app.aliases, name)
	return nil
}

// SetConfig sets the application's Config. This configuration will be inherited by sub-applications and commands unless they have their own configuration set.
func (app *App) SetConfig(configuration Config) {
	app.config = &configuration
}

// RunContext executes the application with the given context and arguments.
// It first checks if the arguments indicate that the help message should be shown, then it tries to find a matching command or sub-application to execute.
// If no matching command or sub-application is found, it returns an error.
func (app *App) RunContext(ctx context.Context, args ...string) error {
	return app.runContext(ctx, nil, args...)
}

func (app *App) runContext(ctx context.Context, inheritedConfig *Config, args ...string) error {
	configuration := app.resolveConfig(inheritedConfig)
	args = app.resolveArgs(args)

	showHelp, invalidHelpArg := inspectAppHelpArgs(args)
	if invalidHelpArg != "" {
		resolvedConfig := materializeConfig(configuration)
		err := unknownOptionError(invalidHelpArg)
		fmt.Fprintln(resolvedConfig.errorLog, err)
		app.showHelp(resolvedConfig)
		return err
	}
	if showHelp {
		app.showHelp(materializeConfig(configuration))
		return nil
	}

	if cmd, cmdArgs, err := app.findCommand(args); err == nil {
		return cmd.runContext(ctx, configuration, app.fullPath()+" "+cmd.name, cmdArgs...)
	}

	subApp, subArgs, err := app.findSubApp(args)
	if err != nil {
		if errors.Is(err, ErrCommandNotFound) {
			resolvedConfig := materializeConfig(configuration)
			fmt.Fprintf(resolvedConfig.errorLog, "Error: %s\n", err.Error())
			app.showHelp(resolvedConfig)
		}
		return err
	}

	return subApp.runContext(ctx, configuration, subArgs...)
}

func (app *App) resolveConfig(inheritedConfig *Config) *Config {
	if app.config != nil {
		return app.config
	}
	return inheritedConfig
}

// RunContextAndExit executes the application with the given context and arguments and exits the process with code 1 if an error occurs.
func (app *App) RunContextAndExit(ctx context.Context, args ...string) {
	if err := app.RunContext(ctx, args...); err != nil {
		os.Exit(1)
	}
}

// Run executes the application with a background context and the given arguments.
func (app *App) Run(args ...string) error {
	return app.RunContext(newContext(), args...)
}

// RunAndExit executes the application with a background context and the given arguments and exits the process with code 1 if an error occurs.
func (app *App) RunAndExit(args ...string) {
	app.RunContextAndExit(newContext(), args...)
}

func (app *App) resolveArgs(args []string) []string {
	if args == nil {
		return os.Args[1:]
	}
	return args
}

func inspectAppHelpArgs(args []string) (showHelp bool, invalidArg string) {
	if len(args) == 0 {
		return true, ""
	}
	if isValuedHelpArg(args[0]) {
		return false, args[0]
	}
	if args[0] != "help" && args[0] != "--help" && args[0] != "-h" {
		return false, ""
	}

	for _, arg := range args[1:] {
		if isValuedHelpArg(arg) {
			return false, arg
		}
	}
	return true, ""
}

func (app *App) showHelp(configuration Config) {
	out := configuration.log
	appPath := app.fullPath()
	fmt.Fprintf(out, "%s\n\n", appPath)
	if app.description != "" {
		fmt.Fprintf(out, "%s\n", app.description)
	}

	fmt.Fprintf(out, "\nUsage:\n  %s [command] [args...]\n", appPath)

	if len(app.aliases) > 0 {
		fmt.Fprintf(out, "\nAliases:\n  %s\n", strings.Join(app.aliases, ", "))
	}

	commandItems := make([]helpItem, 0, len(app.commands)+1)
	for _, cmd := range app.commands {
		commandItems = append(commandItems, helpItem{
			label:       formatNameWithAliases(cmd.name, cmd.aliases),
			description: cmd.description,
		})
	}
	commandItems = append(commandItems, helpItem{
		label:       "help",
		description: "Show this help message",
	})

	fmt.Fprintln(out, "\nCommands:")
	writeHelpItems(out, commandItems)

	if len(app.apps) > 0 {
		subcommandItems := make([]helpItem, 0, len(app.apps))
		for _, subApp := range app.apps {
			subcommandItems = append(subcommandItems, helpItem{
				label:       formatNameWithAliases(subApp.name, subApp.aliases),
				description: subApp.description,
			})
		}

		fmt.Fprintln(out, "\nSubcommands:")
		writeHelpItems(out, subcommandItems)
	}
}

func (app *App) fullPath() string {
	names := append(app.parentPath(), app.displayName())
	return strings.Join(names, " ")
}

func (app *App) parentPath() []string {
	if len(app.appPath) == 0 {
		return nil
	}
	return append([]string(nil), app.appPath...)
}

func (app *App) displayName() string {
	if app.name == "" {
		return "app"
	}
	return app.name
}

func formatNameWithAliases(name string, aliases []string) string {
	if len(aliases) == 0 {
		return name
	}

	return fmt.Sprintf("%s (%s)", name, strings.Join(aliases, ", "))
}

func (app *App) findCommand(args []string) (*Executable, []string, error) {
	if len(args) == 0 {
		return nil, nil, ErrCommandNotFound
	}

	var head string = args[0]

	for i := range app.commands {
		cmd := &app.commands[i]
		if matchesName(cmd.name, cmd.aliases, head) {
			return cmd, args[1:], nil
		}
	}

	return nil, nil, fmt.Errorf("%w: %s", ErrCommandNotFound, head)
}

func (app *App) findSubApp(args []string) (*App, []string, error) {
	if len(args) == 0 {
		return nil, nil, ErrCommandNotFound
	}

	var head string = args[0]

	for i := range app.apps {
		subApp := &app.apps[i]
		if matchesName(subApp.name, subApp.aliases, head) {
			return app.scopedSubApp(subApp), args[1:], nil
		}
	}

	return nil, nil, fmt.Errorf("%w: %s", ErrCommandNotFound, head)
}

func matchesName(name string, aliases []string, head string) bool {
	return name == head || slices.Contains(aliases, head)
}

func (app *App) scopedSubApp(subApp *App) *App {
	scoped := *subApp
	scoped.appPath = append(app.parentPath(), app.displayName())
	return &scoped
}
