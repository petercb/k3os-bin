// Package osimpl provides production operating-system adapters for interfaces.
package osimpl

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// ShellRunner implements iface.CommandRunner using real exec.Command calls.
type ShellRunner struct{}

// Run executes a command with inherited stdout and stderr.
func (ShellRunner) Run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// RunOutput executes a command and returns its stdout as a trimmed string.
func (ShellRunner) RunOutput(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// RunWithStdin executes a command with the provided stdin content.
func (ShellRunner) RunWithStdin(stdin string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdin = strings.NewReader(stdin)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// RunShell executes a command string through sh.
func (ShellRunner) RunShell(command string) error {
	cmd := exec.Command("sh", "-c", command)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run %s: %w", command, err)
	}
	return nil
}

// RunWithEnv executes a command with additional environment variables.
func (ShellRunner) RunWithEnv(env []string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
