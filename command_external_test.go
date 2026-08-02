package suzume_test

import (
	"context"
	"testing"

	"github.com/Luke256/suzume"
)

type embeddedCommand struct {
	suzume.Command
	Value string `cli:"0"`
}

var lastEmbeddedCommand embeddedCommand

func (c *embeddedCommand) Default() {
	c.Value = "default"
}

func (c *embeddedCommand) Run(context.Context) error {
	lastEmbeddedCommand = *c
	return nil
}

type plainCommand struct {
	Value string `cli:"0"`
}

var lastPlainCommand plainCommand

func (*plainCommand) Default() {}

func (c *plainCommand) Run(context.Context) error {
	lastPlainCommand = *c
	return nil
}

func TestUseCommand_WithEmbeddedCommand(t *testing.T) {
	cmd, err := suzume.UseCommand[*embeddedCommand]("embedded", "Embedded command")
	if err != nil {
		t.Fatalf("failed to create command: %v", err)
	}

	if err := cmd.Run("provided"); err != nil {
		t.Fatalf("failed to run command: %v", err)
	}
	if lastEmbeddedCommand.Value != "provided" {
		t.Fatalf("expected argument to override default, got %q", lastEmbeddedCommand.Value)
	}
}

func TestUseCommand_WithoutEmbeddedCommand(t *testing.T) {
	cmd, err := suzume.UseCommand[*plainCommand]("plain", "Plain command")
	if err != nil {
		t.Fatalf("failed to create command: %v", err)
	}

	if err := cmd.Run("provided"); err != nil {
		t.Fatalf("failed to run command: %v", err)
	}
	if lastPlainCommand.Value != "provided" {
		t.Fatalf("expected bound value, got %q", lastPlainCommand.Value)
	}
}
