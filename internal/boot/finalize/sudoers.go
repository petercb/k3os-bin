//go:build linux

package finalize

import (
	"fmt"
	"log/slog"
)

// SetupSudoers writes the sudoers configuration for the sudo group and
// rancher user to /etc/sudoers.d/sudo.
func (f *Finalizer) SetupSudoers() error {
	slog.Debug("finalize: setting up sudoers")

	content := "%sudo   ALL = (ALL) ALL\nrancher ALL = (ALL) NOPASSWD: ALL\n"

	if err := f.FS.MkdirAll("/etc/sudoers.d", 0o755); err != nil {
		return fmt.Errorf("mkdir /etc/sudoers.d: %w", err)
	}
	if err := f.FS.WriteFile("/etc/sudoers.d/sudo", []byte(content), 0o440); err != nil {
		return fmt.Errorf("write /etc/sudoers.d/sudo: %w", err)
	}

	return nil
}
