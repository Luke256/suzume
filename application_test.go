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

	app := NewApp("testapp", "A test application")
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

	app := NewApp("testapp", "A test application")
	cmd, err := NewCommand("notify", "notify command", func() error {
		called = true
		return nil
	})
	if err != nil {
		t.Fatalf("failed to create command: %v", err)
	}
	cmd.Alias("n")
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

	app := NewApp("mycli", "A test CLI")
	app.SetConfig(Config{inherit: true, Log: &out, ErrorLog: &errOut})

	if err := app.Run([]string{}...); err != nil {
		t.Fatalf("expected no error when no args are provided: %v", err)
	}

	help := out.String()
	if !strings.Contains(help, "Usage:\n  mycli [command] [args...]") {
		t.Fatalf("expected app help usage in output, got: %q", help)
	}
	if !strings.Contains(help, "help                 Show this help message") {
		t.Fatalf("expected builtin help command in output, got: %q", help)
	}
	if errOut.Len() != 0 {
		t.Fatalf("expected no error output, got: %q", errOut.String())
	}
}

func TestApp_Run_UnknownCommandWritesErrorAndHelp(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer

	app := NewApp("mycli", "A test CLI")
	app.SetConfig(Config{inherit: true, Log: &out, ErrorLog: &errOut})

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

func TestApp_Run_SubAppHelpShowsScopedPath(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer

	root := NewApp("root", "Root app")
	child := NewApp("child", "Child app")
	root.AddApp(child)
	root.SetConfig(Config{inherit: true, Log: &out, ErrorLog: &errOut})

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

	app := NewApp("root", "Root app")
	app.SetConfig(Config{inherit: true, Log: &out, ErrorLog: &errOut})

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

	if !strings.Contains(out.String(), "Usage: echo") {
		t.Fatalf("expected command help to be written to inherited app log, got: %q", out.String())
	}
	if errOut.Len() != 0 {
		t.Fatalf("expected no stderr output, got: %q", errOut.String())
	}
}

func TestApp_SubAppAlias(t *testing.T) {
	var called bool

	root := NewApp("root", "Root app")
	child := NewApp("child", "Child app")

	cmd, err := NewCommand("run", "Run command", func() error {
		called = true
		return nil
	})
	if err != nil {
		t.Fatalf("failed to create command: %v", err)
	}
	child.AddCommand(cmd)
	child.Alias("c")

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

	app := NewApp("testapp", "A test application")
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