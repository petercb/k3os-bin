//go:build linux

// Package bootstrap implements the bootstrap phase of the k3OS init sequence.
// It ports the shell script overlay/libexec/k3os/bootstrap to Go.
package bootstrap

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/petercb/k3os-bin/internal/iface"
	"github.com/petercb/k3os-bin/internal/system"
)

// Bootstrapper holds dependencies needed to execute the bootstrap phase.
type Bootstrapper struct {
	FS            iface.FileSystem
	Mounter       iface.Mounter
	Cmd           iface.CommandRunner
	CopyDir       func(src, dst string) error
	RCRunner      func() error
	ConfigRunner  func() error
	KernelVersion string
	Mode          string
}

// SetupEtc creates /etc and /proc, mounts tmpfs on /etc and proc on /proc,
// then copies /usr/etc/* into /etc.
//
// Note: All log lines in the bootstrap phase use Info level because /proc is
// not yet mounted, so k3os.debug cannot be detected and debug-level filtering
// is not yet available.
//
// ForceMount is used here because /proc is not yet available at this point
// in the boot sequence, and the regular Mount() checks /proc/self/mountinfo
// to determine if a target is already mounted.
func (b *Bootstrapper) SetupEtc() error {
	slog.Info("bootstrap: setting up /etc")

	if err := b.FS.MkdirAll("/etc", 0o755); err != nil {
		return fmt.Errorf("mkdir /etc: %w", err)
	}
	if err := b.FS.MkdirAll("/proc", 0o755); err != nil {
		return fmt.Errorf("mkdir /proc: %w", err)
	}
	if err := b.Mounter.ForceMount("none", "/etc", "tmpfs", ""); err != nil {
		return fmt.Errorf("mount tmpfs on /etc: %w", err)
	}
	if err := b.Mounter.ForceMount("none", "/proc", "proc", ""); err != nil {
		return fmt.Errorf("mount proc on /proc: %w", err)
	}
	if err := b.CopyDir("/usr/etc", "/etc"); err != nil {
		return fmt.Errorf("copy /usr/etc to /etc: %w", err)
	}
	return nil
}

// SetupModules bind-mounts kernel modules and firmware from .base if they exist.
func (b *Bootstrapper) SetupModules() error {
	slog.Info("bootstrap: setting up modules")

	modulesPath := fmt.Sprintf(".base/lib/modules/%s", b.KernelVersion)
	if _, err := b.FS.Stat(modulesPath); err == nil {
		if err := b.Mounter.Mount(".base/lib/modules", "/lib/modules", "", "bind"); err != nil {
			return fmt.Errorf("bind mount modules: %w", err)
		}
	}

	if _, err := b.FS.Stat(".base/lib/firmware"); err == nil {
		if err := b.Mounter.Mount(".base/lib/firmware", "/lib/firmware", "", "bind"); err != nil {
			return fmt.Errorf("bind mount firmware: %w", err)
		}
	}

	return nil
}

// SetupUsers creates the rancher user and sudo group using pure Go file
// manipulation. This avoids shelling out to sed/addgroup/adduser and similar
// tools which require /dev/null (not available until SetupRC mounts devtmpfs).
func (b *Bootstrapper) SetupUsers() error {
	slog.Info("bootstrap: setting up users")

	// Replace /bin/ash with /bin/bash in /etc/passwd
	if err := b.replaceInFile("/etc/passwd", "/bin/ash", "/bin/bash"); err != nil {
		return fmt.Errorf("replace shell in passwd: %w", err)
	}

	// Create /home directory for user home dirs
	if err := b.FS.MkdirAll("/home", 0o755); err != nil {
		return fmt.Errorf("mkdir /home: %w", err)
	}

	// Add sudo system group to /etc/group
	if err := b.appendLine("/etc/group", "sudo:x:101:rancher"); err != nil {
		return fmt.Errorf("add sudo group: %w", err)
	}

	// Add rancher group (GID 1000) to /etc/group
	if err := b.appendLine("/etc/group", "rancher:x:1000:"); err != nil {
		return fmt.Errorf("add rancher group: %w", err)
	}

	// Add rancher user (UID 1000) to /etc/passwd
	if err := b.appendLine("/etc/passwd", "rancher:x:1000:1000::/home/rancher:/bin/bash"); err != nil {
		return fmt.Errorf("add rancher user: %w", err)
	}

	// Add rancher shadow entry with locked password (*)
	if err := b.appendLine("/etc/shadow", "rancher:*:::::::"); err != nil {
		return fmt.Errorf("add rancher shadow: %w", err)
	}

	// Create rancher home directory
	if err := b.FS.MkdirAll("/home/rancher", 0o755); err != nil {
		return fmt.Errorf("mkdir rancher home: %w", err)
	}

	return nil
}

// replaceInFile reads a file, replaces all occurrences of old with replacement,
// and writes it back. Uses the FS interface for testability.
func (b *Bootstrapper) replaceInFile(path, old, replacement string) error {
	data, err := b.FS.ReadFile(path)
	if err != nil {
		return err
	}
	replaced := strings.ReplaceAll(string(data), old, replacement)
	return b.FS.WriteFile(path, []byte(replaced), 0o644)
}

// appendLine reads a file and appends a line to it (with trailing newline).
func (b *Bootstrapper) appendLine(path, line string) error {
	data, err := b.FS.ReadFile(path)
	if err != nil {
		// File might not exist yet (e.g., /etc/shadow)
		data = nil
	}
	content := string(data)
	if len(content) > 0 && content[len(content)-1] != '\n' {
		content += "\n"
	}
	content += line + "\n"
	return b.FS.WriteFile(path, []byte(content), 0o644)
}

// SetupRC runs the k3os rc logic for hardware initialization (modalias
// module loading, devtmpfs, mounts). The RCRunner field is wired to
// rc.Run in production so the logic is called directly in-process.
func (b *Bootstrapper) SetupRC() error {
	slog.Info("bootstrap: running k3os rc")
	if err := b.RCRunner(); err != nil {
		return fmt.Errorf("k3os rc: %w", err)
	}
	return nil
}

// SetupDirs creates the /run/k3os directory.
func (b *Bootstrapper) SetupDirs() error {
	slog.Info("bootstrap: setting up dirs")
	if err := b.FS.MkdirAll("/run/k3os", 0o755); err != nil {
		return fmt.Errorf("mkdir /run/k3os: %w", err)
	}
	return nil
}

// SetupKernel mounts the kernel squashfs and bind-mounts modules/firmware
// from it. If the squashfs does not exist, it returns nil.
func (b *Bootstrapper) SetupKernel() error {
	slog.Info("bootstrap: setting up kernel")

	kernelPath := system.RootPath("kernel", b.KernelVersion, "kernel.squashfs")
	if _, err := b.FS.Stat(kernelPath); err != nil {
		return nil
	}

	if err := b.FS.MkdirAll("/run/k3os/kernel", 0o755); err != nil {
		return fmt.Errorf("mkdir /run/k3os/kernel: %w", err)
	}
	if err := b.Mounter.Mount(kernelPath, "/run/k3os/kernel", "squashfs", ""); err != nil {
		return fmt.Errorf("mount squashfs: %w", err)
	}
	if err := b.Mounter.Mount("/run/k3os/kernel/lib/modules", "/lib/modules", "", "bind"); err != nil {
		return fmt.Errorf("bind mount kernel modules: %w", err)
	}
	if err := b.Mounter.Mount("/run/k3os/kernel/lib/firmware", "/lib/firmware", "", "bind"); err != nil {
		return fmt.Errorf("bind mount kernel firmware: %w", err)
	}
	if err := b.Cmd.Run("umount", "/run/k3os/kernel"); err != nil {
		return fmt.Errorf("umount /run/k3os/kernel: %w", err)
	}
	return nil
}

// SetupConfig runs "k3os config --initrd" unless the mode is "local".
func (b *Bootstrapper) SetupConfig(mode string) error {
	slog.Info("bootstrap: setting up config", "mode", mode)

	if mode == "local" {
		return nil
	}

	if err := b.ConfigRunner(); err != nil {
		return fmt.Errorf("k3os config --initrd: %w", err)
	}
	return nil
}

// Run executes the full bootstrap sequence in order, stopping on first error.
func (b *Bootstrapper) Run() error {
	steps := []struct {
		name string
		fn   func() error
	}{
		{"SetupEtc", b.SetupEtc},
		{"SetupModules", b.SetupModules},
		{"SetupUsers", b.SetupUsers},
		{"SetupRC", b.SetupRC},
		{"SetupDirs", b.SetupDirs},
		{"SetupKernel", b.SetupKernel},
		{"SetupConfig", func() error { return b.SetupConfig(b.Mode) }},
	}

	for _, step := range steps {
		slog.Info("bootstrap: running step", "step", step.name)
		if err := step.fn(); err != nil {
			return fmt.Errorf("bootstrap %s: %w", step.name, err)
		}
	}

	return nil
}
