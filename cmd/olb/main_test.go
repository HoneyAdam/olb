// cmd/olb test package
package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/openloadbalancer/olb/internal/cli"
)

func TestCLICreation(t *testing.T) {
	// Test that CLI can be created
	c := cli.New("olb", "test-version")
	if c == nil {
		t.Fatal("Failed to create CLI")
	}
}

func TestCLIWithWriters(t *testing.T) {
	var out bytes.Buffer
	var err bytes.Buffer

	c := cli.NewWithWriters("olb", "test-version", &out, &err)
	if c == nil {
		t.Fatal("Failed to create CLI with writers")
	}
}

func TestCLIRun_Help(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer

	c := cli.NewWithWriters("olb", "test-version", &out, &errOut)

	// Register commands
	c.Register(&cli.StartCommand{})
	c.Register(&cli.StopCommand{})
	c.Register(&cli.ReloadCommand{})
	c.Register(&cli.StatusCommand{})
	c.Register(&cli.VersionCommand{})
	c.Register(&cli.ConfigCommand{})
	c.Register(&cli.BackendCommand{})
	c.Register(&cli.HealthCommand{})

	// Run with --help
	err := c.Run([]string{"--help"})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "olb") {
		t.Error("Help output should contain 'olb'")
	}
	if !strings.Contains(output, "version") {
		t.Error("Help output should contain 'version'")
	}
}

func TestCLIRun_Version(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer

	c := cli.NewWithWriters("olb", "1.0.0-test", &out, &errOut)

	// Register commands
	c.Register(&cli.VersionCommand{})

	// Run with --version
	err := c.Run([]string{"--version"})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "1.0.0-test") {
		t.Errorf("Version output should contain '1.0.0-test', got: %s", output)
	}
}

func TestCLIRun_UnknownCommand(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer

	c := cli.NewWithWriters("olb", "test-version", &out, &errOut)

	// Run with unknown command
	err := c.Run([]string{"unknown-command"})
	if err == nil {
		t.Error("Run should return error for unknown command")
	}
}

func TestCLIRun_NoArgs(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer

	c := cli.NewWithWriters("olb", "test-version", &out, &errOut)

	// Register commands
	c.Register(&cli.StartCommand{})
	c.Register(&cli.StopCommand{})
	c.Register(&cli.ReloadCommand{})
	c.Register(&cli.StatusCommand{})
	c.Register(&cli.VersionCommand{})
	c.Register(&cli.ConfigCommand{})
	c.Register(&cli.BackendCommand{})
	c.Register(&cli.HealthCommand{})

	// Run with no args (should show help)
	err := c.Run([]string{})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Commands:") {
		t.Error("Output should contain 'Commands:'")
	}
}

func TestCommandRegistration(t *testing.T) {
	c := cli.New("olb", "test")

	// Register all commands
	commands := []cli.Command{
		&cli.StartCommand{},
		&cli.StopCommand{},
		&cli.ReloadCommand{},
		&cli.StatusCommand{},
		&cli.VersionCommand{},
		&cli.ConfigCommand{},
		&cli.BackendCommand{},
		&cli.HealthCommand{},
	}

	for _, cmd := range commands {
		c.Register(cmd)
		if cmd.Name() == "" {
			t.Error("Command should have a name")
		}
		if cmd.Description() == "" {
			t.Error("Command should have a description")
		}
	}
}
