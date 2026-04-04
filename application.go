package suzume

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
)

var (
	ErrCommandNotFound = errors.New("Command not found")
)

type App struct {
	appPath     []string
	name        string
	aliases     []string
	description string
	commands    []*Command
	apps        []*App
}

func NewApp(name, description string) *App {
	return &App{
		name:        name,
		description: description,
	}
}

func (app *App) AddCommand(cmd *Command) {
	if cmd != nil {
		app.commands = append(app.commands, cmd)
	}
}

func (app *App) AddApp(subApp *App) {
	if subApp != nil {
		app.apps = append(app.apps, subApp)
	}
}

func (app *App) Alias(name string) *App {
	if name == "" {
		return app
	}

	app.aliases = append(app.aliases, name)
	return app
}

func (app *App) Run(args ...string) error {
	args = app.resolveArgs(args)

	if shouldShowAppHelp(args) {
		app.showHelp()
		return nil
	}

	if cmd, cmdArgs, err := app.findCommand(args); err == nil {
		return cmd.Run(cmdArgs...)
	}

	subApp, subArgs, err := app.findSubApp(args)
	if err != nil {
		if errors.Is(err, ErrCommandNotFound) {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err.Error())
			app.showHelp()
		}
		return err
	}

	return subApp.Run(subArgs...)
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

	return args[0] == "help"
}

func (app *App) showHelp() {
	out := os.Stdout
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
