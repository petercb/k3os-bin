//go:build linux

package finalize

import (
	"fmt"
	"log/slog"
)

// Service runlevel definitions matching the shell script.
var (
	sysinitServices  = []string{"hwdrivers", "dmesg", "devfs", "loadkmap", "udev", "udev-root", "udev-coldplug"}
	bootServices     = []string{"acpid", "hwclock", "syslog", "bootmisc", "hostname", "sysctl", "modules", "connman", "dbus", "haveged", "issue"}
	defaultServices  = []string{"sshd", "local", "ccapply", "crond", "iscsid", "ntpd"}
	shutdownServices = []string{"savecache", "killprocs", "mount-ro"}
)

// SetupServices creates OpenRC runlevel symlinks and adds VM-specific services
// based on virtualization detection.
func (f *Finalizer) SetupServices() error {
	slog.Debug("finalize: setting up services")

	// Create standard runlevel symlinks.
	runlevels := []struct {
		level    string
		services []string
	}{
		{"sysinit", sysinitServices},
		{"boot", bootServices},
		{"default", defaultServices},
		{"shutdown", shutdownServices},
	}

	for _, rl := range runlevels {
		for _, svc := range rl.services {
			src := fmt.Sprintf("/etc/init.d/%s", svc)
			dst := fmt.Sprintf("/etc/runlevels/%s/%s", rl.level, svc)
			if err := f.FS.Symlink(src, dst); err != nil {
				return fmt.Errorf("symlink %s to %s: %w", src, dst, err)
			}
		}
	}

	// Detect virtualization and add VM-specific services.
	virtTypes, err := f.VirtDetector()
	if err != nil {
		// virt-what failure is non-fatal (matches "|| true" in shell).
		slog.Debug("finalize: virt-what failed", "error", err)
		virtTypes = nil
	}

	for _, vt := range virtTypes {
		switch vt {
		case "kvm", "qemu":
			if err := f.FS.Symlink("/etc/init.d/qemu-guest-agent", "/etc/runlevels/boot/qemu-guest-agent"); err != nil {
				return fmt.Errorf("symlink qemu-guest-agent: %w", err)
			}
		case "microsoft", "hyperv":
			for _, svc := range []string{"hv_kvp_daemon", "hv_fcopy_daemon", "hv_vss_daemon"} {
				if err := f.FS.Symlink(fmt.Sprintf("/etc/init.d/%s", svc), fmt.Sprintf("/etc/runlevels/boot/%s", svc)); err != nil {
					return fmt.Errorf("symlink %s: %w", svc, err)
				}
			}
		case "vmw", "vmware":
			if err := f.FS.Symlink("/etc/init.d/open-vm-tools", "/etc/runlevels/boot/open-vm-tools"); err != nil {
				return fmt.Errorf("symlink open-vm-tools: %w", err)
			}
		}
	}

	return nil
}
