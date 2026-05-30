//go:build linux

// Package bootstrap implements the bootstrap phase of the k3OS init sequence.
// It ports the shell script overlay/libexec/k3os/bootstrap to Go.
package bootstrap

import (
	"fmt"
	"log/slog"

	"github.com/petercb/k3os-bin/internal/iface"
	"github.com/petercb/k3os-bin/internal/system"
)

// Bootstrapper holds dependencies needed to execute the bootstrap phase.
type Bootstrapper struct {
	FS            iface.FileSystem
	Mounter       iface.Mounter
	Cmd           iface.CommandRunner
	RCRunner      func() error
	KernelVersion string
	Mode          string
}

// SetupEtc creates /etc and /proc, mounts tmpfs on /etc and proc on /proc,
// then copies /usr/etc/* into /etc.
func (b *Bootstrapper) SetupEtc() error {
	slog.Debug("bootstrap: setting up /etc")

	if err := b.FS.MkdirAll("/etc", 0o755); err != nil {
		return fmt.Errorf("mkdir /etc: %w", err)
	}
	if err := b.FS.MkdirAll("/proc", 0o755); err != nil {
		return fmt.Errorf("mkdir /proc: %w", err)
	}
	if err := b.Mounter.Mount("none", "/etc", "tmpfs", ""); err != nil {
		return fmt.Errorf("mount tmpfs on /etc: %w", err)
	}
	if err := b.Mounter.Mount("none", "/proc", "proc", ""); err != nil {
		return fmt.Errorf("mount proc on /proc: %w", err)
	}
	if err := b.Cmd.Run("cp", "-rfp", "/usr/etc/.", "/etc/"); err != nil {
		return fmt.Errorf("copy /usr/etc to /etc: %w", err)
	}
	return nil
}

// SetupModules bind-mounts kernel modules and firmware from .base if they exist.
func (b *Bootstrapper) SetupModules() error {
	slog.Debug("bootstrap: setting up modules")

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

// SetupUsers creates the rancher user and sudo group, matching the original
// shell script behavior.
func (b *Bootstrapper) SetupUsers() error {
	slog.Debug("bootstrap: setting up users")

	if err := b.Cmd.Run("sed", "-i", "s!/bin/ash!/bin/bash!", "/etc/passwd"); err != nil {
		return fmt.Errorf("sed passwd: %w", err)
	}
	if err := b.Cmd.Run("addgroup", "-S", "sudo"); err != nil {
		return fmt.Errorf("addgroup sudo: %w", err)
	}
	if err := b.Cmd.Run("sed", "-i", `s/^(sudo:.*)/\1rancher/g`, "/etc/group"); err != nil {
		return fmt.Errorf("sed group: %w", err)
	}
	if err := b.Cmd.Run("addgroup", "-g", "1000", "rancher"); err != nil {
		return fmt.Errorf("addgroup rancher: %w", err)
	}
	if err := b.Cmd.Run("adduser", "-s", "/bin/bash", "-u", "1000", "-D", "-G", "rancher", "rancher"); err != nil {
		return fmt.Errorf("adduser rancher: %w", err)
	}
	if err := b.Cmd.RunWithStdin("rancher:*\n", "chpasswd", "-e"); err != nil {
		return fmt.Errorf("chpasswd: %w", err)
	}
	return nil
}

// SetupRC runs the k3os rc logic for hardware initialization (modalias
// module loading, devtmpfs, mounts). The RCRunner field is wired to
// rc.Run in production so the logic is called directly in-process.
func (b *Bootstrapper) SetupRC() error {
	slog.Debug("bootstrap: running k3os rc")
	if err := b.RCRunner(); err != nil {
		return fmt.Errorf("k3os rc: %w", err)
	}
	return nil
}

// SetupDirs creates the /run/k3os directory.
func (b *Bootstrapper) SetupDirs() error {
	slog.Debug("bootstrap: setting up dirs")
	if err := b.FS.MkdirAll("/run/k3os", 0o755); err != nil {
		return fmt.Errorf("mkdir /run/k3os: %w", err)
	}
	return nil
}

// SetupKernel mounts the kernel squashfs and bind-mounts modules/firmware
// from it. If the squashfs does not exist, it returns nil.
func (b *Bootstrapper) SetupKernel() error {
	slog.Debug("bootstrap: setting up kernel")

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
	slog.Debug("bootstrap: setting up config", "mode", mode)

	if mode == "local" {
		return nil
	}

	k3osBin := system.RootPath("k3os", "current", "k3os")
	if err := b.Cmd.Run(k3osBin, "config", "--initrd"); err != nil {
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
		slog.Debug("bootstrap: running step", "step", step.name)
		if err := step.fn(); err != nil {
			return fmt.Errorf("bootstrap %s: %w", step.name, err)
		}
	}

	return nil
}
