package suzume

import (
	"bytes"
	"io"
	"os"
	"reflect"
	"testing"
)

var (
	_ func() config                   = DefaultConfig
	_ func(...ConfigOption) config    = NewConfig
	_ func(io.Writer) ConfigOption    = WithLog
	_ func(io.Writer) ConfigOption    = WithErrorLog
	_ func(...os.Signal) ConfigOption = WithIgnoreSignals
	_ func(*App, config)              = (*App).SetConfig
	_ func(*Executable, config)       = (*Executable).SetConfig
)

func TestDefaultConfig_UsesStandardStreams(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.log != os.Stdout {
		t.Fatalf("expected stdout log, got %T", cfg.log)
	}
	if cfg.errorLog != os.Stderr {
		t.Fatalf("expected stderr error log, got %T", cfg.errorLog)
	}
	if len(cfg.ignoreSignals) != 0 {
		t.Fatalf("expected no ignored signals, got %v", cfg.ignoreSignals)
	}
}

func TestNewConfig_StartsFromDefaults(t *testing.T) {
	cfg := NewConfig()

	if cfg.log != os.Stdout {
		t.Fatalf("expected stdout log, got %T", cfg.log)
	}
	if cfg.errorLog != os.Stderr {
		t.Fatalf("expected stderr error log, got %T", cfg.errorLog)
	}
	if len(cfg.ignoreSignals) != 0 {
		t.Fatalf("expected no ignored signals, got %v", cfg.ignoreSignals)
	}
}

func TestConfigOption_IsInterface(t *testing.T) {
	if kind := reflect.TypeFor[ConfigOption]().Kind(); kind != reflect.Interface {
		t.Fatalf("expected ConfigOption to be an interface, got %s", kind)
	}
}

func TestConfigOption_AppliesToConfig(t *testing.T) {
	var out bytes.Buffer
	cfg := DefaultConfig()
	option := WithLog(&out)

	option.apply(&cfg)

	if cfg.log != &out {
		t.Fatalf("expected option to configure log, got %T", cfg.log)
	}
}

func TestNewConfig_IgnoresNilOptions(t *testing.T) {
	var nilOption ConfigOption
	var out bytes.Buffer
	var errOut bytes.Buffer

	cfg := NewConfig(
		WithLog(&out),
		nilOption,
		WithErrorLog(&errOut),
	)

	if cfg.log != &out {
		t.Fatalf("expected configured log, got %T", cfg.log)
	}
	if cfg.errorLog != &errOut {
		t.Fatalf("expected configured error log, got %T", cfg.errorLog)
	}
}

func TestNewConfig_NilWritersKeepDefaults(t *testing.T) {
	cfg := NewConfig(
		WithLog(nil),
		WithErrorLog(nil),
	)

	if cfg.log != os.Stdout {
		t.Fatalf("expected nil log writer to keep stdout, got %T", cfg.log)
	}
	if cfg.errorLog != os.Stderr {
		t.Fatalf("expected nil error writer to keep stderr, got %T", cfg.errorLog)
	}
}

func TestNewConfig_AppliesOptionsInOrder(t *testing.T) {
	var firstOut bytes.Buffer
	var lastOut bytes.Buffer
	var firstErrOut bytes.Buffer
	var lastErrOut bytes.Buffer

	cfg := NewConfig(
		WithLog(&firstOut),
		WithErrorLog(&firstErrOut),
		WithIgnoreSignals(os.Interrupt),
		WithLog(&lastOut),
		WithErrorLog(&lastErrOut),
		WithIgnoreSignals(os.Kill),
	)

	if cfg.log != &lastOut {
		t.Fatalf("expected last log option to win, got %T", cfg.log)
	}
	if cfg.errorLog != &lastErrOut {
		t.Fatalf("expected last error log option to win, got %T", cfg.errorLog)
	}
	if len(cfg.ignoreSignals) != 1 || cfg.ignoreSignals[0] != os.Kill {
		t.Fatalf("expected last signal option to win, got %v", cfg.ignoreSignals)
	}
}

func TestWithIgnoreSignals_ClonesInput(t *testing.T) {
	signals := []os.Signal{os.Interrupt, os.Kill}
	option := WithIgnoreSignals(signals...)

	signals[0] = os.Kill
	cfg := NewConfig(option)
	signals[1] = os.Interrupt

	if len(cfg.ignoreSignals) != 2 {
		t.Fatalf("expected two ignored signals, got %v", cfg.ignoreSignals)
	}
	if cfg.ignoreSignals[0] != os.Interrupt || cfg.ignoreSignals[1] != os.Kill {
		t.Fatalf("expected cloned signal input, got %v", cfg.ignoreSignals)
	}
}

func TestConfigInheritance_RemainsUnresolvedWithoutExplicitConfig(t *testing.T) {
	app := MustNewApp("app", "App")
	cmd := MustNewCommand("run", "Run", func() error { return nil })

	if configuration := app.resolveConfig(nil); configuration != nil {
		t.Fatalf("expected unconfigured app to keep configuration unresolved, got %#v", configuration)
	}
	if configuration := cmd.resolveConfig(nil); configuration != nil {
		t.Fatalf("expected unconfigured command to keep configuration unresolved, got %#v", configuration)
	}

	inheritedConfig := NewConfig(WithLog(io.Discard))
	if configuration := app.resolveConfig(&inheritedConfig); configuration != &inheritedConfig {
		t.Fatal("expected app to pass inherited configuration through without materializing defaults")
	}
	if configuration := cmd.resolveConfig(&inheritedConfig); configuration != &inheritedConfig {
		t.Fatal("expected command to reuse inherited configuration without materializing defaults")
	}
}
