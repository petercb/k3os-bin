//go:build linux

package modes

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	targetDir       = "/run/k3os/target"
	stateLabel      = "K3OS_STATE"
	stateMaxRetries = 10
	stateRetrySleep = 1 * time.Second
)

// ErrExecCalled is a sentinel error returned when Execute() successfully
// calls pivot_root and exec. In production this never returns because exec
// replaces the process, but in tests it signals success.
type ErrExecCalled struct {
	Path string
	Args []string
	Env  []string
}

func (e *ErrExecCalled) Error() string {
	return fmt.Sprintf("exec called: %s %v", e.Path, e.Args)
}

// DiskHandler implements ModeHandler for the "disk" boot mode.
type DiskHandler struct {
	deps *Deps
}

// NewDiskHandler creates a new DiskHandler with the given dependencies.
func NewDiskHandler(deps *Deps) *DiskHandler {
	return &DiskHandler{deps: deps}
}

// Execute runs the disk mode boot sequence: mount, grow, setup, takeover, pivot.
func (h *DiskHandler) Execute() error {
	if err := h.SetupMounts(); err != nil {
		return fmt.Errorf("setup mounts: %w", err)
	}
	if err := h.SetupK3OS(); err != nil {
		return fmt.Errorf("setup k3os: %w", err)
	}
	if err := h.SetupKernelSquashfs(); err != nil {
		return fmt.Errorf("setup kernel squashfs: %w", err)
	}
	if err := h.SetupInit(); err != nil {
		return fmt.Errorf("setup init: %w", err)
	}
	if err := h.SetupK3s(); err != nil {
		return fmt.Errorf("setup k3s: %w", err)
	}
	if err := h.Takeover(); err != nil {
		return fmt.Errorf("takeover: %w", err)
	}
	if err := h.CleanupEphemeral(); err != nil {
		return fmt.Errorf("cleanup ephemeral: %w", err)
	}
	if err := h.PivotAndExec(); err != nil {
		return err
	}
	return nil
}

// SetupMounts mounts K3OS_STATE at the target directory and handles the
// growpart marker if present.
func (h *DiskHandler) SetupMounts() error {
	slog.Debug("disk: setting up mounts")

	if err := h.deps.FS.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("mkdir target: %w", err)
	}

	device, err := h.resolveStateDevice()
	if err != nil {
		return fmt.Errorf("mount K3OS_STATE: %w", err)
	}
	if mntErr := h.deps.Mounter.Mount(device, targetDir, "", ""); mntErr != nil {
		return fmt.Errorf("mount K3OS_STATE: %w", mntErr)
	}

	growpartPath := filepath.Join(targetDir, "k3os/system/growpart")
	data, err := h.deps.FS.ReadFile(growpartPath)
	if err != nil {
		// No growpart marker, nothing to do
		return nil
	}

	fields := strings.Fields(string(data))
	if len(fields) < 2 {
		return nil
	}
	dev, num := fields[0], fields[1]

	// If the device+num path does not exist, probe via BlockProber
	devNum := dev + num
	if _, err := h.deps.FS.Stat(devNum); err != nil {
		part, blkErr := h.deps.BlockProber.FindByLabel("K3OS_STATE")
		if blkErr != nil {
			return nil
		}
		// Extract device (strip trailing digits) and partition number
		dev = stripPartitionNumber(part)
		num = extractPartitionNumber(part)
		devNum = part
	}

	if _, err := h.deps.FS.Stat(devNum); err != nil {
		return nil
	}

	// Unmount, grow, remount
	if err := h.deps.Cmd.Run("umount", targetDir); err != nil {
		return fmt.Errorf("umount for grow: %w", err)
	}
	// grow: resize partition via pure Go GPT manipulation, then e2fsck + resize2fs
	partNumInt, convErr := strconv.Atoi(num)
	if convErr != nil {
		return fmt.Errorf("parse partition number %q: %w", num, convErr)
	}

	if h.deps.PartitionGrower == nil {
		slog.Debug("disk: PartitionGrower not configured, skipping partition grow")
	} else if err := h.deps.PartitionGrower.GrowPartition(dev, partNumInt); err != nil {
		// If partition grow fails, skip e2fsck and resize2fs since the partition
		// boundary may be inconsistent (e.g., GPT partially written). Running
		// filesystem tools against a stale partition layout is unsafe.
		slog.Warn("disk: partition grow failed, skipping filesystem resize", "error", err)
	} else {
		if err := h.deps.Cmd.Run("e2fsck", "-f", devNum); err != nil {
			slog.Warn("disk: e2fsck failed", "error", err)
		}
		if err := h.deps.Cmd.Run("resize2fs", devNum); err != nil {
			slog.Warn("disk: resize2fs failed", "error", err)
		}
	}

	if err := h.deps.Mounter.Mount(device, targetDir, "", ""); err != nil {
		return fmt.Errorf("remount K3OS_STATE: %w", err)
	}
	if err := h.deps.FS.Remove(growpartPath); err != nil {
		slog.Warn("disk: failed to remove growpart marker", "error", err)
	}
	return nil
}

// resolveStateDevice resolves the K3OS_STATE label to an actual device path
// using the BlockProber, retrying up to stateMaxRetries times with a sleep
// between attempts. During early boot the /dev/disk/by-label symlinks may
// not yet be populated (udev race), so we wait for the device to appear.
func (h *DiskHandler) resolveStateDevice() (string, error) {
	sleepFn := h.deps.SleepFunc
	if sleepFn == nil {
		sleepFn = time.Sleep
	}

	for i := range stateMaxRetries {
		device, err := h.deps.BlockProber.FindByLabel(stateLabel)
		if err == nil && device != "" {
			return device, nil
		}
		slog.Debug("disk: waiting for K3OS_STATE device", "attempt", i+1, "error", err)
		sleepFn(stateRetrySleep)
	}
	return "", fmt.Errorf("device with label %q not found after %d attempts", stateLabel, stateMaxRetries)
}

// SetupKernelSquashfs copies kernel.squashfs from .base to target if not present.
func (h *DiskHandler) SetupKernelSquashfs() error {
	slog.Debug("disk: setting up kernel squashfs")

	src := fmt.Sprintf("/.base/k3os/system/kernel/%s/kernel.squashfs", h.deps.KernelVersion)
	dst := filepath.Join(targetDir, "k3os/system/kernel", h.deps.KernelVersion, "kernel.squashfs")

	if _, err := h.deps.FS.Stat(src); err != nil {
		return nil
	}
	if _, err := h.deps.FS.Stat(dst); err == nil {
		return nil
	}

	dstDir := filepath.Dir(dst)
	if err := h.deps.FS.MkdirAll(dstDir, 0o755); err != nil {
		return fmt.Errorf("mkdir kernel dir: %w", err)
	}
	if err := h.deps.Cmd.Run("cp", "-r", src, dst); err != nil {
		return fmt.Errorf("copy kernel squashfs: %w", err)
	}
	return nil
}

// SetupK3OS copies the k3os binary to the target and creates the current symlink.
func (h *DiskHandler) SetupK3OS() error {
	slog.Debug("disk: setting up k3os")

	currentK3OS := filepath.Join(targetDir, "k3os/system/k3os/current/k3os")
	if _, err := h.deps.FS.Stat(currentK3OS); err == nil {
		return nil
	}

	src := "/.base/k3os/system/k3os/current/k3os"
	if _, err := h.deps.FS.Stat(src); err != nil {
		return nil
	}

	dstFile := filepath.Join(targetDir, "k3os/system/k3os", h.deps.VersionID, "k3os")
	if _, err := h.deps.FS.Stat(dstFile); err != nil {
		dstDir := filepath.Dir(dstFile)
		if err := h.deps.FS.MkdirAll(dstDir, 0o755); err != nil {
			return fmt.Errorf("mkdir k3os dir: %w", err)
		}
		tmpFile := dstFile + ".tmp"
		if err := h.deps.Cmd.Run("cp", "-f", src, tmpFile); err != nil {
			return fmt.Errorf("copy k3os binary: %w", err)
		}
		if err := h.deps.FS.Rename(tmpFile, dstFile); err != nil {
			return fmt.Errorf("rename k3os binary: %w", err)
		}
	}

	currentLink := filepath.Join(targetDir, "k3os/system/k3os/current")
	if err := h.deps.FS.Symlink(h.deps.VersionID, currentLink); err != nil {
		// If it already exists, remove and recreate
		_ = h.deps.FS.Remove(currentLink)
		if err := h.deps.FS.Symlink(h.deps.VersionID, currentLink); err != nil {
			return fmt.Errorf("symlink k3os current: %w", err)
		}
	}
	return nil
}

// SetupInit creates the /sbin/init symlink in the target.
func (h *DiskHandler) SetupInit() error {
	slog.Debug("disk: setting up init")

	initPath := filepath.Join(targetDir, "sbin/init")
	if _, err := h.deps.FS.Stat(initPath); err == nil {
		return nil
	}

	sbinDir := filepath.Join(targetDir, "sbin")
	if err := h.deps.FS.MkdirAll(sbinDir, 0o755); err != nil {
		return fmt.Errorf("mkdir sbin: %w", err)
	}
	if err := h.deps.FS.Symlink("../k3os/system/k3os/current/k3os", initPath); err != nil {
		return fmt.Errorf("symlink init: %w", err)
	}
	return nil
}

// SetupK3s finds the latest k3s directory and creates a current symlink.
func (h *DiskHandler) SetupK3s() error {
	slog.Debug("disk: setting up k3s")

	k3sCurrentBin := filepath.Join(targetDir, "k3os/system/k3s/current/k3s")
	if _, err := h.deps.FS.Stat(k3sCurrentBin); err == nil {
		return nil
	}

	k3sDir := filepath.Join(targetDir, "k3os/system/k3s")
	entries, err := h.deps.FS.ReadDir(k3sDir)
	if err != nil {
		// No k3s directory yet, nothing to do
		return nil
	}

	var latest string
	for _, entry := range entries {
		info, infoErr := entry.Info()
		if infoErr != nil {
			continue
		}
		// Skip symlinks, find a real directory
		if info.Mode()&os.ModeSymlink != 0 {
			continue
		}
		if entry.IsDir() {
			latest = entry.Name()
			break
		}
	}

	if latest == "" {
		return nil
	}

	currentLink := filepath.Join(k3sDir, "current")
	_ = h.deps.FS.Remove(currentLink)
	if err := h.deps.FS.Symlink(latest, currentLink); err != nil {
		return fmt.Errorf("symlink k3s current: %w", err)
	}
	return nil
}

// Takeover handles the takeover marker: factory reset and reboot/poweroff.
func (h *DiskHandler) Takeover() error {
	slog.Debug("disk: checking takeover")

	takeoverPath := filepath.Join(targetDir, "k3os/system/takeover")
	if _, err := h.deps.FS.Stat(takeoverPath); err != nil {
		return nil
	}

	// Mark factory reset
	factoryResetPath := filepath.Join(targetDir, "k3os/system/factory-reset")
	if err := h.deps.FS.WriteFile(factoryResetPath, nil, 0o644); err != nil {
		return fmt.Errorf("write factory-reset marker: %w", err)
	}

	// Check if sbin is a symlink and remove it
	sbinPath := filepath.Join(targetDir, "sbin")
	if info, err := h.deps.FS.Lstat(sbinPath); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			_ = h.deps.FS.Remove(sbinPath)
		}
	}

	// Remove everything except boot, k3os, sbin
	entries, err := h.deps.FS.ReadDir(targetDir)
	if err != nil {
		return fmt.Errorf("read target dir: %w", err)
	}
	for _, entry := range entries {
		switch entry.Name() {
		case "boot", "k3os", "sbin":
			continue
		default:
			_ = h.deps.FS.RemoveAll(filepath.Join(targetDir, entry.Name()))
		}
	}

	// Clean sbin, keep only init, k3s, k3os
	sbinEntries, err := h.deps.FS.ReadDir(sbinPath)
	if err == nil {
		for _, entry := range sbinEntries {
			switch entry.Name() {
			case "init", "k3s", "k3os":
				continue
			default:
				_ = h.deps.FS.RemoveAll(filepath.Join(sbinPath, entry.Name()))
			}
		}
	}

	// Clean boot (remove everything except grub*)
	bootDir := filepath.Join(targetDir, "boot")
	bootEntries, err := h.deps.FS.ReadDir(bootDir)
	if err == nil {
		for _, entry := range bootEntries {
			if !strings.HasPrefix(entry.Name(), "g") {
				_ = h.deps.FS.RemoveAll(filepath.Join(bootDir, entry.Name()))
			}
		}
	}

	// Remove takeover marker and data
	_ = h.deps.FS.Remove(takeoverPath)
	_ = h.deps.FS.RemoveAll(filepath.Join(targetDir, "k3os/data"))
	_ = h.deps.Cmd.Run("sync")

	// Check poweroff marker
	poweroffPath := filepath.Join(targetDir, "k3os/system/poweroff")
	if _, err := h.deps.FS.Stat(poweroffPath); err == nil {
		_ = h.deps.FS.Remove(poweroffPath)
		_ = h.deps.Cmd.Run("sync")
		if err := h.deps.Cmd.Run("poweroff", "-f"); err != nil {
			return fmt.Errorf("poweroff -f failed: %w", err)
		}
		// If poweroff returned without replacing the process, prevent further execution.
		slog.Warn("disk: poweroff returned without replacing process")
		return fmt.Errorf("poweroff -f returned unexpectedly")
	}
	if err := h.deps.Cmd.Run("reboot", "-f"); err != nil {
		return fmt.Errorf("reboot -f failed: %w", err)
	}
	// If reboot returned without replacing the process, prevent further execution.
	slog.Warn("disk: reboot returned without replacing process")
	return fmt.Errorf("reboot -f returned unexpectedly")
}

// CleanupEphemeral removes k3os/data and factory-reset marker if either
// factory-reset or ephemeral markers exist.
func (h *DiskHandler) CleanupEphemeral() error {
	slog.Debug("disk: cleanup ephemeral")

	factoryResetPath := filepath.Join(targetDir, "k3os/system/factory-reset")
	ephemeralPath := filepath.Join(targetDir, "k3os/system/ephemeral")

	_, frErr := h.deps.FS.Stat(factoryResetPath)
	_, ephErr := h.deps.FS.Stat(ephemeralPath)

	if frErr != nil && ephErr != nil {
		return nil
	}

	_ = h.deps.FS.RemoveAll(filepath.Join(targetDir, "k3os/data"))
	_ = h.deps.FS.Remove(factoryResetPath)
	return nil
}

// PivotAndExec detaches loop0, makes mount private, does pivot_root, and
// execs /sbin/init with K3OS_MODE=local.
func (h *DiskHandler) PivotAndExec() error {
	slog.Debug("disk: pivot and exec")

	// Detach loop device (best effort)
	if h.deps.LoopDetacher != nil {
		_ = h.deps.LoopDetacher.DetachPath("/dev/loop0")
	}

	// Make root mount private
	if err := h.deps.Mounter.ForceMount("", "/", "none", "rprivate"); err != nil {
		return fmt.Errorf("make root private: %w", err)
	}

	// Create .root for old root
	dotRoot := filepath.Join(targetDir, ".root")
	if err := h.deps.FS.MkdirAll(dotRoot, 0o755); err != nil {
		return fmt.Errorf("mkdir .root: %w", err)
	}

	// pivot_root
	if err := h.deps.Proc.PivotRoot(targetDir, dotRoot); err != nil {
		return fmt.Errorf("pivot_root: %w", err)
	}

	// exec /sbin/init with K3OS_MODE=local
	env := append(os.Environ(), "K3OS_MODE=local")
	err := h.deps.Proc.Exec("/sbin/init", []string{"/sbin/init"}, env)
	if err != nil {
		return fmt.Errorf("exec /sbin/init: %w", err)
	}
	// In tests, the mock returns nil, signal success via sentinel
	return &ErrExecCalled{Path: "/sbin/init", Args: []string{"/sbin/init"}, Env: env}
}

// stripPartitionNumber removes the trailing partition number from a device path.
// e.g., /dev/sda2 -> /dev/sda, /dev/nvme0n1p2 -> /dev/nvme0n1
func stripPartitionNumber(device string) string {
	// Handle nvme-style (e.g., /dev/nvme0n1p2 -> /dev/nvme0n1)
	for i := len(device) - 1; i >= 0; i-- {
		if device[i] < '0' || device[i] > '9' {
			if device[i] == 'p' && i > 0 && device[i-1] >= '0' && device[i-1] <= '9' {
				return device[:i]
			}
			return device[:i+1]
		}
	}
	return device
}

// extractPartitionNumber extracts the trailing partition number from a device path.
// e.g., /dev/sda2 -> 2, /dev/nvme0n1p2 -> 2
func extractPartitionNumber(device string) string {
	for i := len(device) - 1; i >= 0; i-- {
		if device[i] < '0' || device[i] > '9' {
			return device[i+1:]
		}
	}
	return ""
}
