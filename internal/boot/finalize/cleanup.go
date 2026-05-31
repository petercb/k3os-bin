//go:build linux

package finalize

import (
	"fmt"
	"log/slog"
)

// Cleanup removes /run/k3os and, if Mode is set, recreates it and writes
// the mode file.
func (f *Finalizer) Cleanup() error {
	slog.Debug("finalize: cleanup")

	if err := f.FS.RemoveAll("/run/k3os"); err != nil {
		return fmt.Errorf("remove /run/k3os: %w", err)
	}

	if f.Mode != "" {
		if err := f.FS.MkdirAll("/run/k3os", 0o755); err != nil {
			return fmt.Errorf("mkdir /run/k3os: %w", err)
		}
		if err := f.FS.WriteFile("/run/k3os/mode", []byte(f.Mode+"\n"), 0o644); err != nil {
			return fmt.Errorf("write /run/k3os/mode: %w", err)
		}
	}

	return nil
}
