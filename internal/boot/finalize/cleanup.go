//go:build linux

package finalize

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

// Cleanup cleans /run/k3os contents (skipping active mountpoints like
// /run/k3os/kernel) and writes the mode file if Mode is set.
func (f *Finalizer) Cleanup() error {
	slog.Debug("finalize: cleanup")

	// Remove individual entries in /run/k3os rather than RemoveAll on the
	// directory itself. This avoids failing on active mountpoints (e.g.,
	// /run/k3os/kernel is a mounted squashfs for kernel modules).
	entries, err := f.FS.ReadDir("/run/k3os")
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read /run/k3os: %w", err)
	}
	for _, entry := range entries {
		name := entry.Name()
		// Skip the kernel squashfs mountpoint — it must remain mounted
		// for /lib/modules and /lib/firmware bind mounts to stay valid.
		if name == "kernel" {
			continue
		}
		path := filepath.Join("/run/k3os", name)
		if err := f.FS.RemoveAll(path); err != nil {
			slog.Debug("finalize: cleanup skip", "path", path, "error", err)
		}
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
