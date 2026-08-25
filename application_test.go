package suzume

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

func TestApp_Run_ExecutesCommand(t *testing.T) {
	t.Parallel()

	var val int

	app := MustNewApp("testapp", "A test application")
	cmd, err := NewCommand("hoge", "test command", func() error {
		val = 42
		return nil
	})
	if err != nil {
		t.Fatalf("failed to create command: %v", err)
	}

	app.AddCommand(cmd)

	err = app.Run("hoge")
	if err != nil {
		t.Fatalf("failed to run app: %v", err)
	}
	if val != 42 {
		t.Errorf("expected val to be 42, got %d", val)
	}
}

func TestApp_Run_ResolvesCommandAlias(t *testing.T) {
	var called bool

	app := MustNewApp("testapp", "A test application")
	cmd, err := NewCommand("notify", "notify command", func() error {
		called = true
		return nil
	})
	if err != nil {
		t.Fatalf("failed to create command: %v", err)
	}
	if err := cmd.Alias("n"); err != nil {
		t.Fatalf("failed to add alias: %v", err)
	}
	app.AddCommand(cmd)

	if err := app.Run("n"); err != nil {
		t.Fatalf("failed to run app with alias: %v", err)
	}

	if !called {
		t.Fatalf("expected alias to execute command handler")
	}
}

func TestApp_Run_ShowsHelpOnNoArgs(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer

	app := MustNewApp("mycli", "A test CLI")
	app.SetConfig(NewConfig(WithLog(&out), WithErrorLog(&errOut)))

	if err := app.Run([]string{}...); err != nil {
		t.Fatalf("expected no error when no args are provided: %v", err)
	}

	help := out.String()
	if !strings.Contains(help, "Usage:\n  mycli [command] [args...]") {
		t.Fatalf("expected app help usage in output, got: %q", help)
	}
	if !strings.Contains(help, "help  Show this help message") {
		t.Fatalf("expected builtin help command in output, got: %q", help)
	}
	if errOut.Len() != 0 {
		t.Fatalf("expected no error output, got: %q", errOut.String())
	}
}

func TestApp_Run_UnknownCommandWritesErrorAndHelp(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer

	app := MustNewApp("mycli", "A test CLI")
	app.SetConfig(NewConfig(WithLog(&out), WithErrorLog(&errOut)))

	err := app.Run("missing")
	if !errors.Is(err, ErrCommandNotFound) {
		t.Fatalf("expected ErrCommandNotFound, got: %v", err)
	}

	if !strings.Contains(errOut.String(), "Error: Command not found: missing") {
		t.Fatalf("expected unknown command error in stderr, got: %q", errOut.String())
	}
	if !strings.Contains(out.String(), "Usage:\n  mycli [command] [args...]") {
		t.Fatalf("expected app help in stdout, got: %q", out.String())
	}
}

func TestApp_Run_RejectsValuedHelpOptions(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		invalidArg string
	}{
		{name: "long false", args: []string{"--help=false"}, invalidArg: "--help=false"},
		{name: "long true", args: []string{"--help=true"}, invalidArg: "--help=true"},
		{name: "long empty", args: []string{"--help="}, invalidArg: "--help="},
		{name: "short false", args: []string{"-h=false"}, invalidArg: "-h=false"},
		{name: "after long help", args: []string{"--help", "--help=false"}, invalidArg: "--help=false"},
		{name: "after short help", args: []string{"-h", "-h=true"}, invalidArg: "-h=true"},
		{name: "after help command", args: []string{"help", "--help="}, invalidArg: "--help="},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var out bytes.Buffer
			var errOut bytes.Buffer
			app := MustNewApp("mycli", "A test CLI")
			app.SetConfig(NewConfig(WithLog(&out), WithErrorLog(&errOut)))

			err := app.Run(test.args...)
			if !errors.Is(err, ErrInvalidArgument) {
				t.Fatalf("expected ErrInvalidArgument, got: %v", err)
			}
			if errors.Is(err, ErrCommandNotFound) {
				t.Fatalf("expected an invalid option rather than an unknown command: %v", err)
			}
			if !strings.Contains(errOut.String(), "unknown option") || !strings.Contains(errOut.String(), test.invalidArg) {
				t.Fatalf("expected unknown option error for %q, got: %q", test.invalidArg, errOut.String())
			}
			if !strings.Contains(out.String(), "Usage:\n  mycli [command] [args...]") {
				t.Fatalf("expected app help after invalid option, got: %q", out.String())
			}
		})
	}
}

func TestApp_Run_SubAppRejectsValuedHelpOption(t *testing.T) {
	for _, args := range [][]string{
		{"child", "--help=false"},
		{"child", "--help", "--help=false"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var out bytes.Buffer
			var errOut bytes.Buffer
			root := MustNewApp("root", "Root app")
			child := MustNewApp("child", "Child app")
			root.AddApp(child)
			root.SetConfig(NewConfig(WithLog(&out), WithErrorLog(&errOut)))

			err := root.Run(args...)
			if !errors.Is(err, ErrInvalidArgument) {
				t.Fatalf("expected ErrInvalidArgument, got: %v", err)
			}
			if !strings.Contains(errOut.String(), `unknown option "--help=false"`) {
				t.Fatalf("expected unknown option error, got: %q", errOut.String())
			}
			if !strings.Contains(out.String(), "Usage:\n  root child [command] [args...]") {
				t.Fatalf("expected scoped sub-app help after invalid option, got: %q", out.String())
			}
		})
	}
}

func TestApp_Run_SubAppHelpShowsScopedPath(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer

	root := MustNewApp("root", "Root app")
	child := MustNewApp("child", "Child app")
	root.AddApp(child)
	root.SetConfig(NewConfig(WithLog(&out), WithErrorLog(&errOut)))

	if err := root.Run("child"); err != nil {
		t.Fatalf("expected no error when showing sub app help: %v", err)
	}

	help := out.String()
	if !strings.Contains(help, "root child") {
		t.Fatalf("expected scoped app path in help output, got: %q", help)
	}
	if errOut.Len() != 0 {
		t.Fatalf("expected no stderr output, got: %q", errOut.String())
	}
}

func TestApp_Run_InheritsConfigToCommand(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer

	app := MustNewApp("root", "Root app")
	app.SetConfig(NewConfig(WithLog(&out), WithErrorLog(&errOut)))

	cmd, err := NewCommand("echo", "Echo command", func(name string) error {
		return nil
	})
	if err != nil {
		t.Fatalf("failed to create command: %v", err)
	}
	app.AddCommand(cmd)

	if err := app.Run("echo", "--help"); err != nil {
		t.Fatalf("expected no error when showing command help: %v", err)
	}

	if !strings.Contains(out.String(), "Usage: root echo") {
		t.Fatalf("expected command help to be written to inherited app log, got: %q", out.String())
	}
	if errOut.Len() != 0 {
		t.Fatalf("expected no stderr output, got: %q", errOut.String())
	}
}

func TestApp_Run_InheritsErrorConfigToCommand(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer

	app := MustNewApp("root", "Root app")
	app.SetConfig(NewConfig(WithLog(&out), WithErrorLog(&errOut)))
	app.AddCommand(MustNewCommand("count", "Count command", func(int) error { return nil }))

	err := app.Run("count", "invalid")
	if !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("expected ErrInvalidArgument, got: %v", err)
	}
	if !strings.Contains(errOut.String(), "invalid argument") {
		t.Fatalf("expected inherited error log to receive the error, got: %q", errOut.String())
	}
	if !strings.Contains(out.String(), "Usage: root count") {
		t.Fatalf("expected inherited log to receive command help, got: %q", out.String())
	}
}

func TestApp_Run_InheritsConfigThroughNestedApps(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer

	root := MustNewApp("root", "Root app")
	root.SetConfig(NewConfig(WithLog(&out), WithErrorLog(&errOut)))

	child := MustNewApp("child", "Child app")
	grandchild := MustNewApp("grandchild", "Grandchild app")
	cmd := MustNewCommand("echo", "Echo command", func() error { return nil })

	grandchild.AddCommand(cmd)
	child.AddApp(grandchild)
	root.AddApp(child)

	if err := root.Run("child", "grandchild", "echo", "--help"); err != nil {
		t.Fatalf("expected no error when showing nested command help: %v", err)
	}

	if !strings.Contains(out.String(), "Usage: root child grandchild echo") {
		t.Fatalf("expected nested command help in inherited root log, got: %q", out.String())
	}
	if errOut.Len() != 0 {
		t.Fatalf("expected no stderr output, got: %q", errOut.String())
	}
}

func TestApp_Run_CommandHelpShowsFullPathAndOptionValues(t *testing.T) {
	var out bytes.Buffer

	root := MustNewApp("root", "Root app")
	root.SetConfig(NewConfig(WithLog(&out)))
	child := MustNewApp("child", "Child app")
	cmd := MustUseCommand[*struct {
		Command
		Name    string   `cli:"0"`
		Count   int      `cli:"count" short:"c"`
		Tags    []string `cli:"tag" short:"t"`
		Verbose bool     `cli:"verbose" short:"v"`
	}]("run", "Run command")

	if err := child.AddCommand(cmd); err != nil {
		t.Fatalf("failed to add command: %v", err)
	}
	if err := root.AddApp(child); err != nil {
		t.Fatalf("failed to add sub app: %v", err)
	}
	if err := root.Run("child", "run", "--help"); err != nil {
		t.Fatalf("expected no error when showing command help: %v", err)
	}

	help := out.String()
	for _, expected := range []string{
		"Usage: root child run",
		"[-c|--count <value>]",
		"[-t|--tag <value...>]",
		"[-v|--verbose]",
	} {
		if !strings.Contains(help, expected) {
			t.Errorf("expected help to contain %q, got: %q", expected, help)
		}
	}
	if strings.Contains(help, "[-v|--verbose <value>]") {
		t.Errorf("expected boolean option not to show a value, got: %q", help)
	}
}

func TestApp_Run_ExplicitConfigOverridesInheritedConfig(t *testing.T) {
	var rootOut bytes.Buffer
	var childOut bytes.Buffer
	var commandOut bytes.Buffer
	var errOut bytes.Buffer

	root := MustNewApp("root", "Root app")
	root.SetConfig(NewConfig(WithLog(&rootOut), WithErrorLog(&errOut)))

	child := MustNewApp("child", "Child app")
	child.SetConfig(NewConfig(WithLog(&childOut), WithErrorLog(&errOut)))
	grandchild := MustNewApp("grandchild", "Grandchild app")

	inherited := MustNewCommand("inherited", "Inherited command", func() error { return nil })
	overridden := MustNewCommand("overridden", "Overridden command", func() error { return nil })
	overridden.SetConfig(NewConfig(WithLog(&commandOut), WithErrorLog(&errOut)))

	grandchild.AddCommand(inherited)
	grandchild.AddCommand(overridden)
	child.AddApp(grandchild)
	root.AddApp(child)

	if err := root.Run("child", "grandchild", "inherited", "--help"); err != nil {
		t.Fatalf("expected inherited command help to succeed: %v", err)
	}
	if err := root.Run("child", "grandchild", "overridden", "--help"); err != nil {
		t.Fatalf("expected overridden command help to succeed: %v", err)
	}

	if rootOut.Len() != 0 {
		t.Fatalf("expected child config to override root config, got root output: %q", rootOut.String())
	}
	if !strings.Contains(childOut.String(), "Usage: root child grandchild inherited") {
		t.Fatalf("expected unconfigured descendants to inherit child log, got: %q", childOut.String())
	}
	if strings.Contains(childOut.String(), "Usage: root child grandchild overridden") {
		t.Fatalf("expected command config to override child config, got child output: %q", childOut.String())
	}
	if !strings.Contains(commandOut.String(), "Usage: root child grandchild overridden") {
		t.Fatalf("expected explicitly configured command log, got: %q", commandOut.String())
	}
	if errOut.Len() != 0 {
		t.Fatalf("expected no stderr output, got: %q", errOut.String())
	}
}

func TestApp_Run_SharedCommandUsesEachAppConfig(t *testing.T) {
	var firstOut bytes.Buffer
	var firstErrOut bytes.Buffer
	var secondOut bytes.Buffer
	var secondErrOut bytes.Buffer

	first := MustNewApp("first", "First app")
	first.SetConfig(NewConfig(WithLog(&firstOut), WithErrorLog(&firstErrOut)))
	second := MustNewApp("second", "Second app")
	second.SetConfig(NewConfig(WithLog(&secondOut), WithErrorLog(&secondErrOut)))

	shared := MustNewCommand("shared", "Shared command", func() error { return nil })
	first.AddCommand(shared)
	second.AddCommand(shared)

	if err := first.Run("shared", "--help"); err != nil {
		t.Fatalf("expected first app run to succeed: %v", err)
	}
	if err := second.Run("shared", "--help"); err != nil {
		t.Fatalf("expected second app run to succeed: %v", err)
	}
	if err := first.Run("shared", "--help"); err != nil {
		t.Fatalf("expected repeated first app run to succeed: %v", err)
	}

	if got := strings.Count(firstOut.String(), "Usage: first shared"); got != 2 {
		t.Fatalf("expected first app log to receive two runs, got %d: %q", got, firstOut.String())
	}
	if got := strings.Count(secondOut.String(), "Usage: second shared"); got != 1 {
		t.Fatalf("expected second app log to receive one run, got %d: %q", got, secondOut.String())
	}
	if firstErrOut.Len() != 0 || secondErrOut.Len() != 0 {
		t.Fatalf("expected no stderr output, got first %q and second %q", firstErrOut.String(), secondErrOut.String())
	}
}

func TestApp_SubAppAlias(t *testing.T) {
	var called bool

	root := MustNewApp("root", "Root app")
	child := MustNewApp("child", "Child app")

	cmd, err := NewCommand("run", "Run command", func() error {
		called = true
		return nil
	})
	if err != nil {
		t.Fatalf("failed to create command: %v", err)
	}
	child.AddCommand(cmd)
	if err := child.Alias("c"); err != nil {
		t.Fatalf("failed to add alias: %v", err)
	}

	root.AddApp(child)

	if err := root.Run("c", "run"); err != nil {
		t.Fatalf("failed to run sub app with alias: %v", err)
	}

	if !called {
		t.Fatalf("expected sub app alias to execute command handler")
	}
}

func TestApp_Context(t *testing.T) {
	var gotVal int

	app := MustNewApp("testapp", "A test application")
	cmd := MustNewCommand("hoge", "test command", func(ctx context.Context) error {
		gotVal = ctx.Value("key").(int)
		return nil
	})

	app.AddCommand(cmd)

	ctx := context.WithValue(context.Background(), "key", 123)
	err := app.RunContext(ctx, "hoge")
	if err != nil {
		t.Fatalf("failed to run command with context: %v", err)
	}
	if gotVal != 123 {
		t.Fatalf("expected context value 123, got %d", gotVal)
	}
}
