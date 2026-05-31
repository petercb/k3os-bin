//go:build linux

package finalize

import (
	"fmt"
	"log/slog"
)

// SetupMounts bind-mounts /.base/boot to /boot and /.base/k3os/system to
// /k3os/system (read-only), then unmounts /.base (repeatedly, to handle
// live double-mount scenarios).
func (f *Finalizer) SetupMounts() error {
	slog.Debug("finalize: setting up mounts")

	if _, err := f.FS.Stat("/.base/boot"); err == nil {
		if err := f.FS.MkdirAll("/boot", 0o755); err != nil {
			return fmt.Errorf("mkdir /boot: %w", err)
		}
		if err := f.Mounter.Mount("/.base/boot", "/boot", "", "bind"); err != nil {
			return fmt.Errorf("bind mount /boot: %w", err)
		}
	}

	if _, err := f.FS.Stat("/.base/k3os/system"); err == nil {
		if err := f.FS.MkdirAll("/k3os/system", 0o755); err != nil {
			return fmt.Errorf("mkdir /k3os/system: %w", err)
		}
		if err := f.Mounter.Mount("/.base/k3os/system", "/k3os/system", "", "ro,bind"); err != nil {
			return fmt.Errorf("bind mount /k3os/system: %w", err)
		}
	}

	// Unmount /.base repeatedly (live systems double-mount this).
	for {
		mounted, err := f.Mounter.Mounted("/.base")
		if err != nil {
			return fmt.Errorf("check /.base mounted: %w", err)
		}
		if !mounted {
			break
		}
		if err := f.Cmd.Run("umount", "-l", "/.base"); err != nil {
			return fmt.Errorf("umount /.base: %w", err)
		}
	}

	return nil
}
