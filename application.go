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

// App represents a CLI application that can contain commands and sub-applications.
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
	commands []*Command

	// apps is the list of sub-applications directly under this application.
	apps []*App

	// config holds the configuration for the application, such as log output destinations.
	config Config
}

// NewApp creates a new App with the given name and description, and initializes it with the default configuration.
func NewApp(name, description string) *App {
	return &App{
		name:        name,
		description: description,
		config:      defaultConfig(),
	}
}

// AddCommand adds a command to the application. If the command is nil, it is ignored.
func (app *App) AddCommand(cmd *Command) {
	if cmd != nil {
		app.commands = append(app.commands, cmd)
	}
}

// AddApp adds a sub-application to the application. If the sub-application is nil, it is ignored.
func (app *App) AddApp(subApp *App) {
	if subApp != nil {
		app.apps = append(app.apps, subApp)
	}
}

// Alias adds an alias for the application. If the alias name is empty, it is ignored.
func (app *App) Alias(name string) *App {
	if name == "" {
		return app
	}

	app.aliases = append(app.aliases, name)
	return app
}

// SetConfig sets the configuration for the application. This configuration will be inherited by sub-applications and commands unless they have their own configuration set.
func (app *App) SetConfig(config Config) {
	app.config = config
}

// RunContext executes the application with the given context and arguments.
// It first checks if the arguments indicate that the help message should be shown, then it tries to find a matching command or sub-application to execute.
// If no matching command or sub-application is found, it returns an error.
func (app *App) RunContext(ctx context.Context, args ...string) error {
	args = app.resolveArgs(args)

	if shouldShowAppHelp(args) {
		app.showHelp()
		return nil
	}

	if cmd, cmdArgs, err := app.findCommand(args); err == nil {
		if cmd.config.inherit {
			cmd.config = app.config
		}
		return cmd.RunContext(ctx, cmdArgs...)
	}

	subApp, subArgs, err := app.findSubApp(args)
	if err != nil {
		if errors.Is(err, ErrCommandNotFound) {
			fmt.Fprintf(app.config.ErrorLog, "Error: %s\n", err.Error())
			app.showHelp()
		}
		return err
	}

	if subApp.config.inherit {
		subApp.config = app.config
	}
	return subApp.RunContext(ctx, subArgs...)
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

func shouldShowAppHelp(args []string) bool {
	if len(args) == 0 {
		return true
	}

	return args[0] == "help" || args[0] == "--help" || args[0] == "-h"
}

func (app *App) showHelp() {
	out := app.config.Log
	appPath := app.fullPath()
	fmt.Fprintf(out, "%s\n\n", appPath)
	if app.description != "" {
		fmt.Fprintf(out, "%s\n", app.description)
	}

	fmt.Fprintf(out, "\nUsage:\n  %s [command] [args...]\n", appPath)

	if len(app.aliases) > 0 {
		fmt.Fprintf(out, "\nAliases:\n  %s\n", strings.Join(app.aliases, ", "))
	}

	if len(app.commands) > 0 {
		fmt.Fprintln(out, "\nCommands:")
		for _, cmd := range app.commands {
			fmt.Fprintf(out, "  %-20s %s\n", formatNameWithAliases(cmd.name, cmd.aliases), cmd.description)
		}
	}

	if len(app.apps) > 0 {
		fmt.Fprintln(out, "\nSubcommands:")
		for _, subApp := range app.apps {
			fmt.Fprintf(out, "  %-20s %s\n", formatNameWithAliases(subApp.name, subApp.aliases), subApp.description)
		}
	}

	fmt.Fprintln(out, "  help                 Show this help message")
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

func (app *App) findCommand(args []string) (*Command, []string, error) {
	if len(args) == 0 {
		return nil, nil, ErrCommandNotFound
	}

	var head string = args[0]

	for _, cmd := range app.commands {
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

	for _, subApp := range app.apps {
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
