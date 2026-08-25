package suzume

import (
	"bytes"
	"context"
	"encoding"
	"errors"
	"os"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

type testContextKey string

type captureRunner struct {
	Command
	Name    string   `cli:"0" usage:"Name"`
	Num     int      `cli:"num" short:"n" usage:"Number"`
	Morning bool     `cli:"morning" short:"m" usage:"Morning flag"`
	Tasks   []string `cli:"task" short:"t" usage:"Tasks"`
}

var lastCaptureRunner captureRunner

func (r *captureRunner) Default() {
	r.Num = 5
}

func (r *captureRunner) Run(context.Context) error {
	lastCaptureRunner = *r
	return nil
}

type helpLevel int

type helpDefaultsRunner struct {
	Command
	Count   int       `cli:"count" short:"c" usage:"Number of retries"`
	Mode    string    `cli:"mode" usage:"Execution mode" default:"from configuration"`
	Token   string    `cli:"token" usage:"API token" default:""`
	Verbose bool      `cli:"verbose" short:"v" usage:"Verbose output"`
	Tags    []string  `cli:"tag" usage:"Tags"`
	Level   helpLevel `cli:"level" usage:"Execution level" default:"high"`
}

type helpAlignmentRunner struct {
	Command
	Source               string `cli:"0" usage:"Source file"`
	DestinationDirectory string `cli:"1" usage:"Destination directory"`
	Force                bool   `cli:"force" short:"f" usage:"Overwrite existing files"`
	Validate             bool   `cli:"validate-before-writing" usage:"Validate before writing"`
}

var helpDefaultsCalls int
var helpDefaultsRunCalled bool

func (r *helpDefaultsRunner) Default() {
	helpDefaultsCalls++
	r.Count = 3
	r.Mode = "safe"
	r.Token = "secret-token"
	r.Verbose = true
	r.Tags = []string{"stable", "fast"}
	r.Level = 2
}

func (*helpDefaultsRunner) Run(context.Context) error {
	helpDefaultsRunCalled = true
	return nil
}

type plainRunner struct {
	Value string `cli:"0"`
}

var lastPlainRunner plainRunner

func (*plainRunner) Default() {}

func (r *plainRunner) Run(context.Context) error {
	lastPlainRunner = *r
	return nil
}

type noOpRunner struct {
	Command
	Value string `cli:"0"`
}

type errorRunner struct {
	Command
}

type valuedHelpRunner struct {
	Command
	Value string `cli:"0"`
}

type concurrentBindingText string

type pointerArgumentText struct {
	value string
}

func (v *pointerArgumentText) UnmarshalText(text []byte) error {
	v.value = string(text)
	return nil
}

type pointerArgumentRunner struct {
	Command
	Value *pointerArgumentText `cli:"value"`
}

var pointerArgumentRunnerValue *pointerArgumentText

func (r *pointerArgumentRunner) Run(context.Context) error {
	pointerArgumentRunnerValue = r.Value
	return nil
}

var concurrentBindingStarted chan struct{}
var concurrentBindingRelease chan struct{}

func (v *concurrentBindingText) UnmarshalText(text []byte) error {
	if string(text) == "wait" {
		concurrentBindingStarted <- struct{}{}
		<-concurrentBindingRelease
	}
	*v = concurrentBindingText(text)
	return nil
}

type concurrentBindingResult struct {
	first  string
	second string
}

type concurrentBindingRunner struct {
	Command
	First  concurrentBindingText `cli:"0"`
	Second concurrentBindingText `cli:"1"`
}

var concurrentBindingResults chan concurrentBindingResult

func (r *concurrentBindingRunner) Run(context.Context) error {
	concurrentBindingResults <- concurrentBindingResult{string(r.First), string(r.Second)}
	return nil
}

var valuedHelpRunnerCalls int

func (*valuedHelpRunner) Run(context.Context) error {
	valuedHelpRunnerCalls++
	return nil
}

var errRunnerFailed = errors.New("runner failed")

func (*errorRunner) Run(context.Context) error {
	return errRunnerFailed
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

func TestNewCommand_RejectsInvalidFunctionHandlers(t *testing.T) {
	var nilHandler func()

	tests := []struct {
		name    string
		handler any
		want    string
	}{
		{name: "nil", handler: nil, want: "cannot be nil"},
		{name: "typed nil", handler: nilHandler, want: "cannot be nil"},
		{name: "not a function", handler: 1, want: "must be a function"},
		{name: "unsupported argument", handler: func(struct{}) {}, want: "unsupported function handler argument type"},
		{name: "non-error return", handler: func() int { return 0 }, want: "must return no values or a single error"},
		{name: "multiple returns containing error", handler: func() (int, error) { return 0, nil }, want: "must return no values or a single error"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewCommand("invalid", "Invalid handler", test.handler)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("expected error containing %q, got %v", test.want, err)
			}
		})
	}
}

func TestCommand_Run_BindsPointerTextUnmarshaler(t *testing.T) {
	var got *pointerArgumentText
	cmd := MustNewCommand("pointer", "Pointer argument", func(value *pointerArgumentText) {
		got = value
	})

	if err := cmd.Run("hello"); err != nil {
		t.Fatalf("failed to run command: %v", err)
	}
	if got == nil || got.value != "hello" {
		t.Fatalf("expected allocated value hello, got %#v", got)
	}
}

func TestUseCommand_Run_BindsPointerTextUnmarshaler(t *testing.T) {
	pointerArgumentRunnerValue = nil
	cmd := MustUseCommand[*pointerArgumentRunner]("pointer", "Pointer option")

	if err := cmd.Run("--value", "hello"); err != nil {
		t.Fatalf("failed to run command: %v", err)
	}
	if got := pointerArgumentRunnerValue; got == nil || got.value != "hello" {
		t.Fatalf("expected allocated value hello, got %#v", got)
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
	cmd.SetConfig(NewConfig(WithLog(&out), WithErrorLog(&errOut)))

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

func TestCommand_Run_RejectsValuedHelpOptions(t *testing.T) {
	for _, arg := range []string{"--help=false", "--help=true", "--help=", "-h=false"} {
		t.Run(arg, func(t *testing.T) {
			var called bool
			cmd := MustNewCommand("ping", "Ping command", func(string) error {
				called = true
				return nil
			})

			assertValuedHelpOptionRejected(t, cmd, arg)
			if called {
				t.Fatal("expected function handler not to be called")
			}
		})
	}
}

func TestUseCommand_Run_RejectsValuedHelpOptions(t *testing.T) {
	for _, arg := range []string{"--help=false", "--help=true", "--help=", "-h=false"} {
		t.Run(arg, func(t *testing.T) {
			valuedHelpRunnerCalls = 0
			cmd := MustUseCommand[*valuedHelpRunner]("ping", "Ping command")

			assertValuedHelpOptionRejected(t, cmd, arg)
			if valuedHelpRunnerCalls != 0 {
				t.Fatalf("expected command runner not to be called, got %d calls", valuedHelpRunnerCalls)
			}
		})
	}
}

func assertValuedHelpOptionRejected(t *testing.T, cmd *Executable, arg string) {
	t.Helper()

	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.SetConfig(NewConfig(WithLog(&out), WithErrorLog(&errOut)))

	err := cmd.Run(arg)
	if !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("expected ErrInvalidArgument, got: %v", err)
	}
	if !strings.Contains(errOut.String(), "unknown option") || !strings.Contains(errOut.String(), arg) {
		t.Fatalf("expected unknown option error for %q, got: %q", arg, errOut.String())
	}
	if !strings.Contains(out.String(), "Usage: ping") {
		t.Fatalf("expected command help after invalid option, got: %q", out.String())
	}
}

func TestUseCommand_HelpShowsOptionDefaultValues(t *testing.T) {
	helpDefaultsCalls = 0
	helpDefaultsRunCalled = false

	cmd, err := UseCommand[*helpDefaultsRunner]("run", "Run command")
	if err != nil {
		t.Fatalf("failed to create command: %v", err)
	}
	if helpDefaultsCalls != 0 {
		t.Fatalf("expected Default not to be called during command creation")
	}

	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.SetConfig(NewConfig(WithLog(&out), WithErrorLog(&errOut)))

	if err := cmd.Run("--help"); err != nil {
		t.Fatalf("expected no error: %v", err)
	}

	help := out.String()
	for _, expected := range []string{
		"  -c, --count    Number of retries (default: 3)",
		"      --mode     Execution mode (default: from configuration)",
		"      --tag      Tags (default: [stable fast])",
		"      --level    Execution level (default: high)",
	} {
		if !strings.Contains(help, expected) {
			t.Errorf("expected help to contain %q, got: %q", expected, help)
		}
	}
	for _, unexpected := range []string{
		"API token (default:",
		"secret-token",
		"Verbose output (default:",
		"Execution mode (default: safe)",
		"Execution level (default: 2)",
		"Show this help message (default:",
	} {
		if strings.Contains(help, unexpected) {
			t.Errorf("expected help not to contain %q, got: %q", unexpected, help)
		}
	}
	if helpDefaultsCalls != 1 {
		t.Fatalf("expected Default to be called once for help, got %d", helpDefaultsCalls)
	}
	if helpDefaultsRunCalled {
		t.Fatalf("expected Run not to be called when help is requested")
	}
	if errOut.Len() != 0 {
		t.Fatalf("expected no stderr output, got: %q", errOut.String())
	}
}

func TestUseCommand_HelpAlignsDescriptions(t *testing.T) {
	cmd, err := UseCommand[*helpAlignmentRunner]("copy", "Copy files")
	if err != nil {
		t.Fatalf("failed to create command: %v", err)
	}

	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.SetConfig(NewConfig(WithLog(&out), WithErrorLog(&errOut)))

	if err := cmd.Run("--help"); err != nil {
		t.Fatalf("expected no error: %v", err)
	}

	help := out.String()
	descriptionColumn := func(description string) int {
		for _, line := range strings.Split(help, "\n") {
			if column := strings.Index(line, description); column >= 0 {
				return column
			}
		}
		return -1
	}

	sourceColumn := descriptionColumn("Source file")
	destinationColumn := descriptionColumn("Destination directory")
	if sourceColumn < 0 || sourceColumn != destinationColumn {
		t.Fatalf("expected argument descriptions to align, got: %q", help)
	}

	forceColumn := descriptionColumn("Overwrite existing files")
	validateColumn := descriptionColumn("Validate before writing")
	if forceColumn < 0 || forceColumn != validateColumn {
		t.Fatalf("expected option descriptions to align, got: %q", help)
	}

	if strings.Contains(help, "\t") {
		t.Fatalf("expected tabs to be expanded in help output, got: %q", help)
	}
	if errOut.Len() != 0 {
		t.Fatalf("expected no stderr output, got: %q", errOut.String())
	}
}

func TestUseCommand_InvalidArgumentCallsDefaultOnce(t *testing.T) {
	helpDefaultsCalls = 0
	helpDefaultsRunCalled = false

	cmd, err := UseCommand[*helpDefaultsRunner]("run", "Run command")
	if err != nil {
		t.Fatalf("failed to create command: %v", err)
	}

	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.SetConfig(NewConfig(WithLog(&out), WithErrorLog(&errOut)))

	err = cmd.Run("--count", "invalid")
	if !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("expected ErrInvalidArgument, got: %v", err)
	}
	if helpDefaultsCalls != 1 {
		t.Fatalf("expected Default to be called once, got %d", helpDefaultsCalls)
	}
	if helpDefaultsRunCalled {
		t.Fatalf("expected Run not to be called for invalid arguments")
	}
	if !strings.Contains(out.String(), "Usage: run") {
		t.Fatalf("expected help output, got: %q", out.String())
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
	cmd.SetConfig(NewConfig(WithLog(&out), WithErrorLog(&errOut)))

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

func TestCommand_RunContext_PassesContextToHandler(t *testing.T) {
	const key testContextKey = "request-id"

	var gotValue string

	cmd, err := NewCommand("ctx", "Context command", func(ctx context.Context) error {
		value, _ := ctx.Value(key).(string)
		gotValue = value
		return nil
	})
	if err != nil {
		t.Fatalf("failed to create command: %v", err)
	}

	ctx := context.WithValue(context.Background(), key, "req-123")

	err = cmd.RunContext(ctx, []string{}...)
	if err != nil {
		t.Fatalf("expected no error: %v", err)
	}

	if gotValue != "req-123" {
		t.Fatalf("expected context value req-123, got %q", gotValue)
	}

	if ctx.Err() != nil {
		t.Fatalf("expected context not to be cancelled, got error: %v", ctx.Err())
	}
}

func TestCommand_RunContext_BindsArgsAndContext(t *testing.T) {
	const key testContextKey = "trace"

	var gotName string
	var gotTrace string

	cmd, err := NewCommand("ctx-arg", "Context and arg command", func(ctx context.Context, name string) error {
		gotName = name
		gotTrace, _ = ctx.Value(key).(string)
		return nil
	})
	if err != nil {
		t.Fatalf("failed to create command: %v", err)
	}

	ctx := context.WithValue(context.Background(), key, "trace-xyz")

	err = cmd.RunContext(ctx, "alice")
	if err != nil {
		t.Fatalf("expected no error: %v", err)
	}

	if gotName != "alice" {
		t.Fatalf("expected positional argument alice, got %q", gotName)
	}
	if gotTrace != "trace-xyz" {
		t.Fatalf("expected context trace trace-xyz, got %q", gotTrace)
	}
}

func TestCommand_Run_UsesBackgroundContextForContextHandler(t *testing.T) {
	var cmdCtx context.Context

	cmd, err := NewCommand("ctx-run", "Run with context", func(ctx context.Context) error {
		cmdCtx = ctx
		return nil
	})
	if err != nil {
		t.Fatalf("failed to create command: %v", err)
	}

	err = cmd.Run([]string{}...)
	if err != nil {
		t.Fatalf("expected no error: %v", err)
	}

	if cmdCtx == nil {
		t.Fatalf("expected context to be passed to handler")
	}

	if cmdCtx.Err() == nil {
		t.Fatalf("expected context to be cancelled after handler returns")
	}
}

func TestUseCommand_BindsValuesAndResetsBetweenRuns(t *testing.T) {
	lastCaptureRunner = captureRunner{}

	cmd, err := UseCommand[*captureRunner]("notify", "Notify command")
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

func TestCommand_Run_ConcurrentArgumentBinding(t *testing.T) {
	results := make(chan concurrentBindingResult, 2)
	cmd := MustNewCommand("concurrent", "Concurrent command", func(first, second concurrentBindingText) {
		results <- concurrentBindingResult{string(first), string(second)}
	})

	assertConcurrentArgumentBinding(t, cmd.Run, results)
}

func TestUseCommand_Run_ConcurrentArgumentBinding(t *testing.T) {
	results := make(chan concurrentBindingResult, 2)
	concurrentBindingResults = results
	cmd := MustUseCommand[*concurrentBindingRunner]("concurrent", "Concurrent command")

	assertConcurrentArgumentBinding(t, cmd.Run, results)
}

func assertConcurrentArgumentBinding(t *testing.T, run func(...string) error, results <-chan concurrentBindingResult) {
	t.Helper()

	concurrentBindingStarted = make(chan struct{})
	concurrentBindingRelease = make(chan struct{})

	firstRun := make(chan error, 1)
	go func() {
		firstRun <- run("first", "wait")
	}()

	select {
	case <-concurrentBindingStarted:
	case <-time.After(10 * time.Second):
		t.Fatal("first run did not reach the binding barrier")
	}

	if err := run("second", "ready"); err != nil {
		t.Fatalf("second run failed: %v", err)
	}
	close(concurrentBindingRelease)

	select {
	case err := <-firstRun:
		if err != nil {
			t.Fatalf("first run failed: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("first run did not finish")
	}

	want := map[concurrentBindingResult]bool{
		{first: "first", second: "wait"}:   true,
		{first: "second", second: "ready"}: true,
	}
	for range 2 {
		select {
		case got := <-results:
			if !want[got] {
				t.Fatalf("arguments leaked between concurrent runs: %#v", got)
			}
			delete(want, got)
		case <-time.After(10 * time.Second):
			t.Fatal("handler did not return a result")
		}
	}
	if len(want) != 0 {
		t.Fatalf("missing concurrent run results: %#v", want)
	}
}

func TestUseCommand_DoesNotRequireEmbeddedCommand(t *testing.T) {
	lastPlainRunner = plainRunner{}

	cmd, err := UseCommand[*plainRunner]("plain", "Plain runner")
	if err != nil {
		t.Fatalf("failed to create command: %v", err)
	}
	if err := cmd.Run("value"); err != nil {
		t.Fatalf("failed to run command: %v", err)
	}
	if lastPlainRunner.Value != "value" {
		t.Fatalf("expected bound value, got %q", lastPlainRunner.Value)
	}
}

func TestUseCommand_RejectsArgumentTagCollisions(t *testing.T) {
	tests := []struct {
		name   string
		create func() error
	}{
		{
			name: "long names",
			create: func() error {
				_, err := UseCommand[*struct {
					Command
					First  string `cli:"value"`
					Second string `cli:"value"`
				}]("test", "Test command")
				return err
			},
		},
		{
			name: "short names",
			create: func() error {
				_, err := UseCommand[*struct {
					Command
					First  string `cli:"first" short:"v"`
					Second string `cli:"second" short:"v"`
				}]("test", "Test command")
				return err
			},
		},
		{
			name: "long and short names",
			create: func() error {
				_, err := UseCommand[*struct {
					Command
					First  string `cli:"v"`
					Second string `cli:"second" short:"v"`
				}]("test", "Test command")
				return err
			},
		},
		{
			name: "long and short name on the same field",
			create: func() error {
				_, err := UseCommand[*struct {
					Command
					Value string `cli:"v" short:"v"`
				}]("test", "Test command")
				return err
			},
		},
		{
			name: "built-in long help name",
			create: func() error {
				_, err := UseCommand[*struct {
					Command
					Help bool `cli:"help"`
				}]("test", "Test command")
				return err
			},
		},
		{
			name: "built-in short help name",
			create: func() error {
				_, err := UseCommand[*struct {
					Command
					Help bool `cli:"show-help" short:"h"`
				}]("test", "Test command")
				return err
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.create(); !errors.Is(err, ErrDuplicateIdentifier) {
				t.Fatalf("expected duplicate identifier error, got %v", err)
			}
		})
	}
}

func TestUseCommand_RejectsUnsupportedFieldTypes(t *testing.T) {
	tests := []struct {
		name   string
		create func() error
	}{
		{
			name: "positional argument",
			create: func() error {
				_, err := UseCommand[*struct {
					Command
					Value map[string]string `cli:"0"`
				}]("test", "Test command")
				return err
			},
		},
		{
			name: "option",
			create: func() error {
				_, err := UseCommand[*struct {
					Command
					Value map[string]string `cli:"value"`
				}]("test", "Test command")
				return err
			},
		},
		{
			name: "slice option element",
			create: func() error {
				_, err := UseCommand[*struct {
					Command
					Value []map[string]string `cli:"value"`
				}]("test", "Test command")
				return err
			},
		},
		{
			name: "interface option",
			create: func() error {
				_, err := UseCommand[*struct {
					Command
					Value encoding.TextUnmarshaler `cli:"value"`
				}]("test", "Test command")
				return err
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.create()
			if err == nil || !strings.Contains(err.Error(), "unsupported command field type") {
				t.Fatalf("expected unsupported field type error, got %v", err)
			}
		})
	}
}

func TestUseCommand_RejectsInvalidOptionIdentifiers(t *testing.T) {
	tests := []struct {
		name   string
		create func() error
	}{
		{
			name: "hyphen-prefixed long name",
			create: func() error {
				_, err := UseCommand[*struct {
					Command
					Value string `cli:"-bad"`
				}]("test", "Test command")
				return err
			},
		},
		{
			name: "valued long name",
			create: func() error {
				_, err := UseCommand[*struct {
					Command
					Value string `cli:"bad=value"`
				}]("test", "Test command")
				return err
			},
		},
		{
			name: "hyphen-prefixed short name",
			create: func() error {
				_, err := UseCommand[*struct {
					Command
					Value string `cli:"value" short:"-v"`
				}]("test", "Test command")
				return err
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.create(); !errors.Is(err, ErrInvalidIdentifier) {
				t.Fatalf("expected invalid identifier error, got %v", err)
			}
		})
	}
}

func TestUseCommand_SortsSparsePositionalIndexes(t *testing.T) {
	cmd, err := UseCommand[*struct {
		Command
		Zero  string `cli:"0"`
		Five  string `cli:"5"`
		Three string `cli:"3"`
		Seven string `cli:"7"`
	}]("test", "Test command")
	if err != nil {
		t.Fatalf("expected sparse positional indexes to be accepted, got %v", err)
	}

	var got []int
	for _, spec := range cmd.argSpecs {
		if spec.index >= 0 {
			got = append(got, spec.index)
		}
	}
	if want := []int{0, 3, 5, 7}; !reflect.DeepEqual(got, want) {
		t.Fatalf("expected positional indexes %v, got %v", want, got)
	}
}

func TestUseCommand_AllowsDuplicatePositionalIndexes(t *testing.T) {
	_, err := UseCommand[*struct {
		Command
		First  string `cli:"0"`
		Second string `cli:"0"`
	}]("test", "Test command")
	if err != nil {
		t.Fatalf("expected duplicate positional indexes to be accepted with unspecified order, got %v", err)
	}
}

func TestUseCommand_RejectsNegativePositionalIndex(t *testing.T) {
	_, err := UseCommand[*struct {
		Command
		Value string `cli:"-1"`
	}]("test", "Test command")
	if err == nil {
		t.Fatal("expected negative positional index error")
	}
}

func TestUseCommand_EmbeddedCommandIsNotAnArgument(t *testing.T) {
	cmd, err := UseCommand[*noOpRunner]("noop", "No-op command")
	if err != nil {
		t.Fatalf("failed to create command: %v", err)
	}

	if len(cmd.argSpecs) != 2 {
		t.Fatalf("expected value and help argument specs, got %d", len(cmd.argSpecs))
	}
	if findSpecByName(cmd.argSpecs, "command") != nil {
		t.Fatalf("embedded Command must not be exposed as an option")
	}
	if err := cmd.Run("value"); err != nil {
		t.Fatalf("default Run implementation failed: %v", err)
	}
}

func TestUseCommand_PropagatesRunError(t *testing.T) {
	cmd, err := UseCommand[*errorRunner]("error", "Error command")
	if err != nil {
		t.Fatalf("failed to create command: %v", err)
	}

	err = cmd.Run([]string{}...)
	if !errors.Is(err, errRunnerFailed) {
		t.Fatalf("expected runner error, got %v", err)
	}
}

func TestContext_Done(t *testing.T) {
	done := make(chan struct{})
	result := make(chan int)

	signalTestCommand := func(ctx context.Context) {
		select {
		case <-ctx.Done():
			result <- 1
		case <-done:
			result <- 2
		}
	}

	cmd, err := NewCommand("signal", "Signal test command", signalTestCommand)
	if err != nil {
		t.Fatalf("unexpected error during creating command: %v", err)
	}

	// finish normally
	go func() {
		cmd.Run([]string{}...)
	}()
	done <- struct{}{}

	select {
	case r := <-result:
		if r != 2 {
			t.Fatalf("Expected result is 1 for done case, but got %d", r)
		}
	case <-time.After(10 * time.Second):
		t.Fatalf("signalTestCommand does not finished in 10 seconds.")
	}

	// finish by context cancellation
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		cmd.RunContext(ctx, []string{}...)
	}()

	cancel()

	select {
	case r := <-result:
		if r != 1 {
			t.Fatalf("Expected result is 2 for cancel case, but got %d", r)
		}
	case <-time.After(10 * time.Second):
		t.Fatalf("signalTestCommand does not finished in 10 seconds.")
	}
}

func TestCommand_RunContext_SignalHandling(t *testing.T) {
	t.Run("Cancel context without signal config", func(t *testing.T) {
		done := make(chan struct{})
		result := make(chan int)

		cmd, err := NewCommand("sig-none", "Test no signal", func(ctx context.Context) {
			select {
			case <-ctx.Done():
				result <- 1
			case <-done:
				result <- 2
			}
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			cmd.RunContext(ctx, []string{}...)
		}()

		cancel()

		r := <-result
		if r != 1 {
			t.Fatalf("Expected context to be cancelled, got %d", r)
		}
	})

	t.Run("Signal with config", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("Skipping signal test on Windows")
		}

		done := make(chan struct{})
		result := make(chan int)

		cmd, err := NewCommand("sig-configured", "Test with signal", func(ctx context.Context) {
			select {
			case <-ctx.Done():
				result <- 1
			case <-done:
				result <- 2
			}
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var out bytes.Buffer
		cmd.SetConfig(NewConfig(
			WithIgnoreSignals(os.Interrupt),
			WithLog(&out),
			WithErrorLog(&out),
		))

		ready := make(chan struct{})

		go func() {
			ready <- struct{}{}
			cmd.Run([]string{}...)
		}()

		select {
		case <-ready:
		case <-time.After(10 * time.Second):
			t.Fatalf("sig-configured does not started in 10 seconds.")
		}

		proc, err := os.FindProcess(os.Getpid())
		if err != nil {
			t.Fatal(err)
		}
		proc.Signal(os.Interrupt)

		r := <-result
		if r != 1 {
			t.Fatalf("Expected context to be cancelled by signal, got %d", r)
		}
	})
}
