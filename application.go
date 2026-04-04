package mycli

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
	appPath		[]string
	name        string
	aliases     []string
	description string
	commands    []*Command
	apps		[]*App
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
	// コードからの指定がない場合はコマンドライン引数を使用する
	if args == nil {
		args = os.Args[1:]
	}

	// ヘルプ
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		app.showHelp()
		return nil
	}

	// コマンドを検索し、実行する
	cmd, cmdArgs, err := app.findCommand(args)
	if err == nil {
		return cmd.Run(cmdArgs...)
	}

	// サブコマンドを検索し、実行する
	subApp, subArgs, err := app.findSubApp(args)
	if err == nil {
		return subApp.Run(subArgs...)
	}

	return fmt.Errorf("%w: %s", ErrCommandNotFound, args[0])
}

func (app *App) showHelp() {
	out := os.Stdout
	name := app.name
	if name == "" {
		name = "app"
	}

	appPath := strings.Join(append(app.appPath, app.name), " ")
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

	fmt.Fprintln(out, "\nOptions:")
	fmt.Fprintln(out, "  -h, --help, help    Show help")
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
		if cmd.name == head || slices.Contains(cmd.aliases, head) {
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
		if subApp.name == head || slices.Contains(subApp.aliases, head) {
			subApp.appPath = append(app.appPath, app.name)
			return subApp, args[1:], nil
		}
	}

	return nil, nil, fmt.Errorf("%w: %s", ErrCommandNotFound, head)
}