package suzume

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

func TestApp_AddRejectsIdentifierCollisions(t *testing.T) {
	app := MustNewApp("root", "Root app")
	command := MustNewCommand("run", "Run command", func() error { return nil })
	if err := command.Alias("r"); err != nil {
		t.Fatalf("failed to add alias: %v", err)
	}
	if err := app.AddCommand(command); err != nil {
		t.Fatalf("failed to add initial command: %v", err)
	}

	duplicate := MustNewApp("other", "Other app")
	if err := duplicate.Alias("r"); err != nil {
		t.Fatalf("failed to add alias: %v", err)
	}
	if err := app.AddApp(duplicate); !errors.Is(err, ErrDuplicateIdentifier) {
		t.Fatalf("expected duplicate identifier error, got %v", err)
	}
}

func TestApp_AddAcceptsDuplicateAliases(t *testing.T) {
	tests := []struct {
		name       string
		aliases    []string
		identifier string
	}{
		{name: "name and alias", aliases: []string{"run"}, identifier: "run"},
		{name: "aliases", aliases: []string{"r", "r"}, identifier: "r"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app := MustNewApp("root", "Root app")
			command := MustNewCommand("run", "Run command", func() error { return nil })
			for _, alias := range test.aliases {
				if err := command.Alias(alias); err != nil {
					t.Fatalf("failed to add alias: %v", err)
				}
			}

			if err := app.AddCommand(command); err != nil {
				t.Fatalf("expected duplicate aliases to be accepted, got %v", err)
			}
			if got := app.commands[0].aliases; !slices.Equal(got, test.aliases) {
				t.Fatalf("expected aliases to remain %v, got %v", test.aliases, got)
			}

			replacement := MustNewCommand(test.identifier, "Replacement", func() error { return nil })
			if err := app.AddCommand(replacement); !errors.Is(err, ErrDuplicateIdentifier) {
				t.Fatalf("expected registered identifier %q to be reserved, got %v", test.identifier, err)
			}
		})
	}
}

func TestApp_AddRejectsReservedHelpIdentifier(t *testing.T) {
	tests := []struct {
		name string
		add  func(*App) error
	}{
		{
			name: "command name",
			add: func(app *App) error {
				return app.AddCommand(MustNewCommand("help", "Help", func() error { return nil }))
			},
		},
		{
			name: "command alias",
			add: func(app *App) error {
				command := MustNewCommand("run", "Run", func() error { return nil })
				if err := command.Alias("help"); err != nil {
					return err
				}
				return app.AddCommand(command)
			},
		},
		{
			name: "app name",
			add: func(app *App) error {
				return app.AddApp(MustNewApp("help", "Help"))
			},
		},
		{
			name: "app alias",
			add: func(app *App) error {
				child := MustNewApp("child", "Child")
				if err := child.Alias("help"); err != nil {
					return err
				}
				return app.AddApp(child)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.add(MustNewApp("root", "Root")); !errors.Is(err, ErrDuplicateIdentifier) {
				t.Fatalf("expected duplicate identifier error, got %v", err)
			}
		})
	}
}

func TestConstructorsRejectHyphenPrefixedNames(t *testing.T) {
	tests := []struct {
		name   string
		create func() error
	}{
		{
			name: "app",
			create: func() error {
				_, err := NewApp("--help", "Help")
				return err
			},
		},
		{
			name: "new command",
			create: func() error {
				_, err := NewCommand("-run", "Run", func() error { return nil })
				return err
			},
		},
		{
			name: "use command",
			create: func() error {
				_, err := UseCommand[*Command]("-run", "Run")
				return err
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.create()
			if err == nil || !strings.Contains(err.Error(), "cannot start with '-'") {
				t.Fatalf("expected invalid identifier error, got %v", err)
			}
		})
	}
}

func TestAliasRejectsHyphenPrefixedNames(t *testing.T) {
	tests := []struct {
		name  string
		alias func() error
	}{
		{
			name: "command alias",
			alias: func() error {
				command := MustNewCommand("run", "Run", func() error { return nil })
				return command.Alias("-r")
			},
		},
		{
			name: "app alias",
			alias: func() error {
				return MustNewApp("child", "Child").Alias("--child")
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.alias()
			if err == nil || !strings.Contains(err.Error(), "cannot start with '-'") {
				t.Fatalf("expected invalid identifier error, got %v", err)
			}
		})
	}
}

func TestApp_AddStoresImmutableValues(t *testing.T) {
	app := MustNewApp("root", "Root app")
	command := MustNewCommand("run", "Run command", func() error { return nil })
	if err := command.Alias("r"); err != nil {
		t.Fatalf("failed to add alias: %v", err)
	}
	if err := app.AddCommand(command); err != nil {
		t.Fatalf("failed to add command: %v", err)
	}
	if err := command.Alias("late-command"); err != nil {
		t.Fatalf("failed to add alias: %v", err)
	}

	child := MustNewApp("child", "Child app")
	if err := child.Alias("c"); err != nil {
		t.Fatalf("failed to add alias: %v", err)
	}
	if err := app.AddApp(child); err != nil {
		t.Fatalf("failed to add app: %v", err)
	}
	if err := child.Alias("late-app"); err != nil {
		t.Fatalf("failed to add alias: %v", err)
	}

	for _, identifier := range []string{"r", "c"} {
		if _, _, err := app.findCommand([]string{identifier}); err != nil {
			if _, _, appErr := app.findSubApp([]string{identifier}); appErr != nil {
				t.Fatalf("expected registered identifier %q to remain available", identifier)
			}
		}
	}
	for _, identifier := range []string{"late-command", "late-app"} {
		if _, _, err := app.findCommand([]string{identifier}); err == nil {
			t.Fatalf("post-registration alias %q changed the registered command", identifier)
		}
		if _, _, err := app.findSubApp([]string{identifier}); err == nil {
			t.Fatalf("post-registration alias %q changed the registered app", identifier)
		}
	}
}
