package suzume

import (
	"bytes"
	"errors"
	"reflect"
	"strings"
	"testing"
)

type captureRunner struct {
	Name    string   `cli:"0" usage:"Name"`
	Num     int      `cli:"num" short:"n" usage:"Number"`
	Morning bool     `cli:"morning" short:"m" usage:"Morning flag"`
	Tasks   []string `cli:"task" short:"t" usage:"Tasks"`
}

var lastCaptureRunner captureRunner

func (r captureRunner) Default() captureRunner {
	return captureRunner{
		Num: 5,
	}
}

func (r captureRunner) Run() error {
	lastCaptureRunner = r
	return nil
}

func TestNewCommand_EmptyNameReturnsError(t *testing.T) {
	_, err := NewCommand("", "desc", func() error { return nil })
	if err == nil {
		t.Fatalf("expected error when command name is empty")
	}
	if !strings.Contains(err.Error(), "Command name cannot be empty") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCommand_Run_HelpSkipsHandler(t *testing.T) {
	var called int

	cmd, err := NewCommand("ping", "Ping command", func() error {
		called++
		return nil
	})
	if err != nil {
		t.Fatalf("failed to create command: %v", err)
	}

	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.SetConfig(Config{inherit: true, Log: &out, ErrorLog: &errOut})

	if err := cmd.Run("--help"); err != nil {
		t.Fatalf("expected no error: %v", err)
	}

	if called != 0 {
		t.Fatalf("expected handler not to be called when help is requested")
	}
	if !strings.Contains(out.String(), "Usage: ping") {
		t.Fatalf("expected help output, got: %q", out.String())
	}
	if errOut.Len() != 0 {
		t.Fatalf("expected no stderr output, got: %q", errOut.String())
	}
}

func TestCommand_Run_InvalidArgumentShowsHelpAndError(t *testing.T) {
	cmd, err := NewCommand("count", "Count command", func(v int) error {
		return nil
	})
	if err != nil {
		t.Fatalf("failed to create command: %v", err)
	}

	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.SetConfig(Config{inherit: true, Log: &out, ErrorLog: &errOut})

	err = cmd.Run("oops")
	if !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("expected ErrInvalidArgument, got: %v", err)
	}

	if !strings.Contains(errOut.String(), "invalid argument") {
		t.Fatalf("expected invalid argument error output, got: %q", errOut.String())
	}
	if !strings.Contains(out.String(), "Usage: count") {
		t.Fatalf("expected help output after invalid argument, got: %q", out.String())
	}
}

func TestUseCommand_BindsValuesAndResetsBetweenRuns(t *testing.T) {
	lastCaptureRunner = captureRunner{}

	cmd, err := UseCommand[captureRunner]("notify", "Notify command")
	if err != nil {
		t.Fatalf("failed to create command: %v", err)
	}

	err = cmd.Run("alice", "--num", "2", "-m", "-t", "build", "test")
	if err != nil {
		t.Fatalf("first run failed: %v", err)
	}

	if lastCaptureRunner.Name != "alice" {
		t.Fatalf("expected Name=alice, got %q", lastCaptureRunner.Name)
	}
	if lastCaptureRunner.Num != 2 {
		t.Fatalf("expected Num=2, got %d", lastCaptureRunner.Num)
	}
	if !lastCaptureRunner.Morning {
		t.Fatalf("expected Morning=true")
	}
	if !reflect.DeepEqual(lastCaptureRunner.Tasks, []string{"build", "test"}) {
		t.Fatalf("unexpected tasks: %#v", lastCaptureRunner.Tasks)
	}

	err = cmd.Run("bob")
	if err != nil {
		t.Fatalf("second run failed: %v", err)
	}

	if lastCaptureRunner.Name != "bob" {
		t.Fatalf("expected Name=bob, got %q", lastCaptureRunner.Name)
	}
	if lastCaptureRunner.Num != 5 {
		t.Fatalf("expected Num to fall back to Default() value 5, got %d", lastCaptureRunner.Num)
	}
	if lastCaptureRunner.Morning {
		t.Fatalf("expected Morning=false on second run")
	}
	if len(lastCaptureRunner.Tasks) != 0 {
		t.Fatalf("expected Tasks to be empty on second run, got %#v", lastCaptureRunner.Tasks)
	}
}

func TestUseCommand_RejectsPointerRunnerType(t *testing.T) {
	_, err := UseCommand[*captureRunner]("notify", "Notify command")
	if err == nil {
		t.Fatalf("expected error for pointer runner type")
	}
	if !strings.Contains(err.Error(), "Runner type cannot be a pointer") {
		t.Fatalf("unexpected error: %v", err)
	}
}
