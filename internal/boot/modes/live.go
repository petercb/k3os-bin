//go:build linux

package modes

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/petercb/k3os-bin/internal/system"
)

const (
	baseDir     = "/.base"
	maxRetries  = 5
	retrySleep  = 1 * time.Second
	motdPath    = "/etc/motd"
	motdContent = "\nYou can configure this system or install to disk using \"sudo k3os install\"\n"
)

// LiveSetup contains the shared live setup logic used by live, install, and
// shell modes.
type LiveSetup struct {
	deps *Deps
}

// NewLiveSetup creates a new LiveSetup with the given dependencies.
func NewLiveSetup(deps *Deps) *LiveSetup {
	return &LiveSetup{deps: deps}
}

// Run executes the full live setup sequence.
func (l *LiveSetup) Run() error {
	if err := l.SetupBase(); err != nil {
		return fmt.Errorf("setup base: %w", err)
	}
	if err := l.SetupKernel(); err != nil {
		return fmt.Errorf("setup kernel: %w", err)
	}
	if err := l.SetupPasswd(); err != nil {
		return fmt.Errorf("setup passwd: %w", err)
	}
	if err := l.SetupMotd(); err != nil {
		return fmt.Errorf("setup motd: %w", err)
	}
	return nil
}

// SetupBase mounts the K3OS ISO or probes USB disks at /.base.
// It first tries finding the K3OS label; if found, mounts it read-only.
// Otherwise, probes all block disks up to maxRetries times with 1s sleep.
func (l *LiveSetup) SetupBase() error {
	slog.Debug("live: setting up base")

	// Try ISO label first
	device, err := l.deps.BlockProber.FindByLabel("K3OS")
	if err == nil && device != "" {
		if err := l.deps.Mounter.Mount(device, baseDir, "", "ro"); err != nil {
			return fmt.Errorf("mount K3OS ISO: %w", err)
		}
		return nil
	}

	// Probe USB disks with retry loop
	sleepFn := l.deps.SleepFunc
	if sleepFn == nil {
		sleepFn = time.Sleep
	}

	for j := range maxRetries {
		diskNames, diskErr := l.deps.BlockProber.ListDisks()
		if diskErr == nil {
			for _, name := range diskNames {
				dev := "/dev/" + name
				if err := l.deps.Mounter.Mount(dev, baseDir, "", ""); err == nil {
					return nil
				}
			}
		}
		slog.Info("live: waiting for USB", "attempt", j+1)
		sleepFn(retrySleep)
	}

	return fmt.Errorf("failed to mount base filesystem after %d attempts", maxRetries)
}

// SetupKernel mounts the kernel squashfs and bind-mounts modules/firmware.
func (l *LiveSetup) SetupKernel() error {
	slog.Debug("live: setting up kernel")

	kernelPath := system.RootPath("kernel", l.deps.KernelVersion, "kernel.squashfs")
	if _, err := l.deps.FS.Stat(kernelPath); err != nil {
		return nil
	}

	if err := l.deps.FS.MkdirAll("/run/k3os/kernel", 0o755); err != nil {
		return fmt.Errorf("mkdir kernel: %w", err)
	}
	if err := l.deps.Mounter.Mount(kernelPath, "/run/k3os/kernel", "squashfs", ""); err != nil {
		return fmt.Errorf("mount squashfs: %w", err)
	}
	if err := l.deps.Mounter.Mount("/run/k3os/kernel/lib/modules", "/lib/modules", "", "bind"); err != nil {
		return fmt.Errorf("bind mount modules: %w", err)
	}
	if err := l.deps.Mounter.Mount("/run/k3os/kernel/lib/firmware", "/lib/firmware", "", "bind"); err != nil {
		return fmt.Errorf("bind mount firmware: %w", err)
	}
	if err := l.deps.Cmd.Run("umount", "/run/k3os/kernel"); err != nil {
		return fmt.Errorf("umount kernel: %w", err)
	}
	return nil
}

// SetupPasswd removes the password for the rancher user.
func (l *LiveSetup) SetupPasswd() error {
	slog.Debug("live: removing rancher password")
	if err := l.deps.Cmd.Run("passwd", "-d", "rancher"); err != nil {
		return fmt.Errorf("passwd -d rancher: %w", err)
	}
	return nil
}

// SetupMotd appends install instructions to /etc/motd.
func (l *LiveSetup) SetupMotd() error {
	slog.Debug("live: setting up motd")

	existing, _ := l.deps.FS.ReadFile(motdPath)
	content := string(existing) + motdContent
	if err := l.deps.FS.WriteFile(motdPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write motd: %w", err)
	}
	return nil
}

// parseDisks extracts disk names from lsblk-style output.
// Each line is "NAME TYPE"; we want lines where TYPE is "disk".
func parseDisks(output string) []string {
	var disks []string
	for _, line := range splitLines(output) {
		fields := splitFields(line)
		if len(fields) >= 2 && fields[1] == "disk" {
			disks = append(disks, fields[0])
		}
	}
	return disks
}

// splitLines splits a string into lines, filtering empty ones.
func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := range len(s) {
		if s[i] == '\n' {
			line := s[start:i]
			if line != "" {
				lines = append(lines, line)
			}
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

// splitFields splits a line into whitespace-separated fields.
func splitFields(s string) []string {
	var fields []string
	start := -1
	for i := range len(s) {
		if s[i] == ' ' || s[i] == '\t' {
			if start >= 0 {
				fields = append(fields, s[start:i])
				start = -1
			}
		} else {
			if start < 0 {
				start = i
			}
		}
	}
	if start >= 0 {
		fields = append(fields, s[start:])
	}
	return fields
}

// LiveHandler implements ModeHandler for the "live" boot mode.
type LiveHandler struct {
	deps *Deps
}

// NewLiveHandler creates a new LiveHandler.
func NewLiveHandler(deps *Deps) *LiveHandler {
	return &LiveHandler{deps: deps}
}

// Execute runs the live mode setup.
func (h *LiveHandler) Execute() error {
	return NewLiveSetup(h.deps).Run()
}

// InstallHandler implements ModeHandler for the "install" boot mode.
// It is identical to live mode.
type InstallHandler struct {
	deps *Deps
}

// NewInstallHandler creates a new InstallHandler.
func NewInstallHandler(deps *Deps) *InstallHandler {
	return &InstallHandler{deps: deps}
}

// Execute runs the install mode setup (same as live).
func (h *InstallHandler) Execute() error {
	return NewLiveSetup(h.deps).Run()
}
