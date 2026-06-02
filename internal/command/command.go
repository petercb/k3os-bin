// Package command provides utilities for executing system commands and managing passwords.
package command

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"

	"github.com/petercb/k3os-bin/internal/iface/osimpl"
	"github.com/petercb/k3os-bin/internal/shadow"
)

// ExecuteCommand runs a list of shell commands sequentially, stopping on first failure.
func ExecuteCommand(commands []string) error {
	for _, cmd := range commands {
		slog.Debug("running command", "cmd", cmd)
		c := exec.Command("sh", "-c", cmd)
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		if err := c.Run(); err != nil {
			return fmt.Errorf("failed to run %s: %w", cmd, err)
		}
	}
	return nil
}

// SetPassword sets the password for the rancher user in /etc/shadow.
func SetPassword(password string) error {
	if password == "" {
		return nil
	}
	s := shadow.Setter{}
	return s.SetPassword(osimpl.OSFileSystem{}, "rancher", password)
}
