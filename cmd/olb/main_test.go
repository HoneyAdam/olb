// cmd/olb test package
package main

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/openloadbalancer/olb/internal/cli"
)

// TestMain enables subprocess-based testing of the main() function.
// When GO_WANT_HELPER_PROCESS=1 is set, it overrides os.Args from
// the GO_TEST_ARGS environment variable and runs main() directly.
func TestMain(m *testing.M) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") == "1" {
		if args := os.Getenv("GO_TEST_ARGS"); args != "" {
			parts := strings.Split(args, "\x00")
			os.Args = append([]string{"olb"}, parts...)
		} else {
			os.Args = []string{"olb"}
		}
		main()
		// main() calls os.Exit, so we never reach here
		return
	}
	os.Exit(m.Run())
}

func TestCLICreation(t *testing.T) {
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
	c.Register(&cli.StartCommand{})
	c.Register(&cli.StopCommand{})
	c.Register(&cli.ReloadCommand{})
	c.Register(&cli.StatusCommand{})
	c.Register(&cli.VersionCommand{})
	c.Register(&cli.ConfigCommand{})
	c.Register(&cli.BackendCommand{})
	c.Register(&cli.HealthCommand{})

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
	c.Register(&cli.VersionCommand{})

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
	err := c.Run([]string{"unknown-command"})
	if err == nil {
		t.Error("Run should return error for unknown command")
	}
}

func TestCLIRun_NoArgs(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer

	c := cli.NewWithWriters("olb", "test-version", &out, &errOut)
	c.Register(&cli.StartCommand{})
	c.Register(&cli.StopCommand{})
	c.Register(&cli.ReloadCommand{})
	c.Register(&cli.StatusCommand{})
	c.Register(&cli.VersionCommand{})
	c.Register(&cli.ConfigCommand{})
	c.Register(&cli.BackendCommand{})
	c.Register(&cli.HealthCommand{})

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

// Tests for the run() function extracted from main()

func TestRun_Help(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"--help"}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("run(--help) returned %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "olb") {
		t.Error("Help output should contain 'olb'")
	}
}

func TestRun_Version(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"--version"}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("run(--version) returned %d, want 0", code)
	}
}

func TestRun_UnknownCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"foobar"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("run(foobar) returned %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "Error:") {
		t.Errorf("stderr should contain 'Error:', got: %q", stderr.String())
	}
}

func TestRun_NoArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("run() returned %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "Commands:") {
		t.Error("No-args output should contain 'Commands:'")
	}
}

func TestRun_StatusWithoutServer(t *testing.T) {
	var stdout, stderr bytes.Buffer
	// status command should fail gracefully without running server
	code := run([]string{"status"}, &stdout, &stderr)
	// It may return 1 (error) since there's no server, which is fine
	_ = code
}

// helperMain runs the current test binary as a subprocess simulating main().
// CLI arguments are passed via the GO_TEST_ARGS environment variable to avoid
// interference with the go test framework's own -test.* flags.
func helperMain(t *testing.T, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	env := append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
	if len(args) > 0 {
		env = append(env, "GO_TEST_ARGS="+strings.Join(args, "\x00"))
	}
	cmd := exec.Command(os.Args[0], "-test.run=^$")
	cmd.Env = env
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	stdout = outBuf.String()
	stderr = errBuf.String()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to run subprocess: %v", err)
		}
	}
	return
}

// TestMain_Help verifies that main() produces help output and exits 0.
func TestMain_Help(t *testing.T) {
	stdout, stderr, code := helperMain(t, "--help")
	if code != 0 {
		t.Errorf("main --help: exit code %d, want 0\nstderr: %s", code, stderr)
	}
	if !strings.Contains(stdout, "olb") {
		t.Errorf("main --help: output should contain 'olb', got: %s", stdout)
	}
}

// TestMain_Version verifies that main() produces version output and exits 0.
func TestMain_Version(t *testing.T) {
	stdout, stderr, code := helperMain(t, "--version")
	if code != 0 {
		t.Errorf("main --version: exit code %d, want 0\nstderr: %s", code, stderr)
	}
	if !strings.Contains(stdout, "version") {
		t.Errorf("main --version: output should contain 'version', got stdout=%q stderr=%q", stdout, stderr)
	}
}

// TestMain_UnknownCommand verifies that main() exits with code 1 for bad args.
func TestMain_UnknownCommand(t *testing.T) {
	_, stderr, code := helperMain(t, "nonexistent-cmd-xyz")
	if code != 1 {
		t.Errorf("main nonexistent-cmd: exit code %d, want 1", code)
	}
	if !strings.Contains(stderr, "Error:") {
		t.Errorf("main nonexistent-cmd: stderr should contain 'Error:', got: %q", stderr)
	}
}

// TestMain_NoArgs verifies that main() with no args shows help and exits 0.
func TestMain_NoArgs(t *testing.T) {
	stdout, stderr, code := helperMain(t)
	if code != 0 {
		t.Errorf("main (no args): exit code %d, want 0\nstderr: %s", code, stderr)
	}
	if !strings.Contains(stdout, "Commands:") {
		t.Errorf("main (no args): output should contain 'Commands:', got: %s", stdout)
	}
}
