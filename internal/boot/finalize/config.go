//go:build linux

package finalize

import (
	"fmt"
	"log/slog"
)

// SetupConfig runs "k3os config --boot" and conditionally adds services
// based on the existence of conf.d files.
func (f *Finalizer) SetupConfig() error {
	slog.Debug("finalize: setting up config")

	if err := f.ConfigRunner(); err != nil {
		return fmt.Errorf("k3os config --boot: %w", err)
	}

	// Add udev-settle to sysinit if its conf.d file exists.
	if _, err := f.FS.Stat("/etc/conf.d/udev-settle"); err == nil {
		if err := f.FS.Symlink("/etc/init.d/udev-settle", "/etc/runlevels/sysinit/udev-settle"); err != nil {
			return fmt.Errorf("symlink udev-settle: %w", err)
		}
	}

	// Add wpa_supplicant dependency to connman if cloud-config.config exists.
	if _, err := f.FS.Stat("/var/lib/connman/cloud-config.config"); err == nil {
		existing, _ := f.FS.ReadFile("/etc/conf.d/connman")
		content := string(existing) + "rc_want=\"wpa_supplicant\"\n"
		if err := f.FS.WriteFile("/etc/conf.d/connman", []byte(content), 0o644); err != nil {
			return fmt.Errorf("write connman conf: %w", err)
		}
	}

	// Add cloud-config to boot if its conf.d file exists.
	if _, err := f.FS.Stat("/etc/conf.d/cloud-config"); err == nil {
		if err := f.FS.Symlink("/etc/init.d/cloud-config", "/etc/runlevels/boot/cloud-config"); err != nil {
			return fmt.Errorf("symlink cloud-config: %w", err)
		}
	}

	// Add rngd to boot if its conf.d file exists.
	if _, err := f.FS.Stat("/etc/conf.d/rngd"); err == nil {
		if err := f.FS.Symlink("/etc/init.d/rngd", "/etc/runlevels/boot/rngd"); err != nil {
			return fmt.Errorf("symlink rngd: %w", err)
		}
	}

	return nil
}
