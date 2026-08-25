package suzume_test

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/Luke256/suzume"
)

func TestConfigAPI_NewConfigIsUsableFromExternalPackage(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	options := []suzume.ConfigOption{
		suzume.WithLog(&out),
		suzume.WithErrorLog(&errOut),
		suzume.WithIgnoreSignals(os.Interrupt),
	}
	cfg := suzume.NewConfig(options...)

	app := suzume.MustNewApp("external", "External config API")
	app.SetConfig(cfg)

	if err := app.Run([]string{}...); err != nil {
		t.Fatalf("expected app help to succeed: %v", err)
	}
	if !strings.Contains(out.String(), "Usage:\n  external [command] [args...]") {
		t.Fatalf("expected configured app log, got: %q", out.String())
	}
	if errOut.Len() != 0 {
		t.Fatalf("expected no stderr output, got: %q", errOut.String())
	}
}

func TestConfigAPI_ConfiguresExecutableFromExternalPackage(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd := suzume.MustNewCommand("external", "External config API", func() error { return nil })
	cmd.SetConfig(suzume.NewConfig(
		suzume.WithLog(&out),
		suzume.WithErrorLog(&errOut),
	))

	if err := cmd.Run("--help"); err != nil {
		t.Fatalf("expected command help to succeed: %v", err)
	}
	if !strings.Contains(out.String(), "Usage: external") {
		t.Fatalf("expected configured command log, got: %q", out.String())
	}
	if errOut.Len() != 0 {
		t.Fatalf("expected no stderr output, got: %q", errOut.String())
	}
}

func TestConfigAPI_DefaultConfigIsUsableFromExternalPackage(t *testing.T) {
	var called bool
	cmd := suzume.MustNewCommand("run", "Run command", func() error {
		called = true
		return nil
	})
	cmd.SetConfig(suzume.DefaultConfig())

	app := suzume.MustNewApp("external", "External config API")
	app.SetConfig(suzume.DefaultConfig())
	app.AddCommand(cmd)

	if err := app.Run("run"); err != nil {
		t.Fatalf("expected command to run with default config: %v", err)
	}
	if !called {
		t.Fatalf("expected command handler to run")
	}
}
