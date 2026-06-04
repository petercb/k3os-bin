//go:build linux

// Package devpopulate provides a pure Go replacement for "mdev -s" device
// population. It performs two tasks during early boot:
//
//  1. Device node creation: walks /sys/class/block, reads the "dev" file for
//     each entry (which contains major:minor numbers), and ensures the
//     corresponding block device node exists under /dev via unix.Mknod. On
//     modern kernels with devtmpfs mounted, these nodes already exist; Mknod
//     will simply return EEXIST which is silently ignored.
//
//  2. Symlink population: probes each block device for filesystem labels and
//     UUIDs (using go-blockdevice/v2's blkid), and creates /dev/disk/by-label/
//     and /dev/disk/by-uuid/ symlinks equivalent to what mdev or udev creates.
//
// This eliminates the need to shell out to mdev (and by extension, busybox)
// for cold-plug device population during early boot.
package devpopulate

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/siderolabs/go-blockdevice/v2/blkid"
	"golang.org/x/sys/unix"
)

// Prober abstracts block device filesystem probing for testability.
type Prober interface {
	// Probe returns the filesystem label and UUID for a device path.
	// Returns empty strings (not errors) if the device has no label/UUID.
	Probe(devPath string) (label, uuid string, err error)
}

// BlkidProber implements Prober using go-blockdevice/v2's blkid package.
type BlkidProber struct{}

// Probe opens the given device path and probes for filesystem metadata.
func (BlkidProber) Probe(devPath string) (label, uuid string, err error) {
	info, err := blkid.ProbePath(devPath, blkid.WithSkipLocking(true))
	if err != nil {
		return "", "", fmt.Errorf("probe %s: %w", devPath, err)
	}

	if info.Label != nil {
		label = *info.Label
	}
	if info.UUID != nil {
		uuid = info.UUID.String()
	}

	// Also check nested partitions for labels/UUIDs.
	if label == "" && uuid == "" {
		for _, part := range info.Parts {
			if part.Label != nil && *part.Label != "" {
				label = *part.Label
			}
			if part.UUID != nil {
				uuid = part.UUID.String()
			}
			if label != "" || uuid != "" {
				break
			}
		}
	}

	return label, uuid, nil
}

// Options configures the PopulateDev behavior.
type Options struct {
	// DevDir is the root device directory (default: /dev).
	DevDir string
	// SysBlockDir is the sysfs block device directory (default: /sys/class/block).
	SysBlockDir string
	// Prober is the filesystem prober to use.
	Prober Prober
	// CreateNodes controls whether to create device nodes via Mknod.
	// When devtmpfs is mounted, this is unnecessary but harmless (EEXIST is ignored).
	CreateNodes bool
}

// DefaultOptions returns Options with production defaults.
func DefaultOptions() Options {
	return Options{
		DevDir:      "/dev",
		SysBlockDir: "/sys/class/block",
		Prober:      BlkidProber{},
		CreateNodes: true,
	}
}

// PopulateDev scans sysfs for block devices and:
//  1. Ensures device nodes exist under /dev (via Mknod if CreateNodes is true).
//  2. Creates /dev/disk/by-label/ and /dev/disk/by-uuid/ symlinks for devices
//     that have a filesystem label or UUID.
//
// This is a pure Go replacement for "mdev -s". Individual failures are logged
// and skipped (best-effort population).
func PopulateDev(opts Options) error {
	entries, err := os.ReadDir(opts.SysBlockDir)
	if err != nil {
		return fmt.Errorf("read sysfs block dir %s: %w", opts.SysBlockDir, err)
	}

	// Collect all device names to probe (whole disks + their partitions).
	var devNames []string
	for _, entry := range entries {
		devNames = append(devNames, entry.Name())

		// Scan for partitions: /sys/class/block/<disk>/<diskN>
		partDir := filepath.Join(opts.SysBlockDir, entry.Name())
		partEntries, err := os.ReadDir(partDir)
		if err != nil {
			continue
		}
		for _, pe := range partEntries {
			if pe.IsDir() && isPartition(entry.Name(), pe.Name()) {
				devNames = append(devNames, pe.Name())
			}
		}
	}

	for _, name := range devNames {
		devPath := filepath.Join(opts.DevDir, name)

		// Step 1: Ensure device node exists (read major:minor from sysfs).
		if opts.CreateNodes {
			ensureDeviceNode(opts.SysBlockDir, opts.DevDir, name)
		}

		// Skip if the device node still doesn't exist after attempted creation.
		if _, err := os.Stat(devPath); err != nil {
			continue
		}

		// Step 2: Probe for label/UUID and create symlinks.
		label, uuid, err := opts.Prober.Probe(devPath)
		if err != nil {
			slog.Debug("devpopulate: probe failed", "device", devPath, "error", err)
			continue
		}

		if label != "" {
			if err := createSymlink(opts.DevDir, "by-label", label, name); err != nil {
				slog.Debug("devpopulate: symlink failed", "type", "label", "device", name, "error", err)
			}
		}

		if uuid != "" {
			if err := createSymlink(opts.DevDir, "by-uuid", uuid, name); err != nil {
				slog.Debug("devpopulate: symlink failed", "type", "uuid", "device", name, "error", err)
			}
		}
	}

	return nil
}

// ensureDeviceNode reads the major:minor from /sys/class/block/<name>/dev
// and creates the block device node at /dev/<name> if it doesn't exist.
// This is the same logic as "mdev -s" iterating /sys and calling mknod.
func ensureDeviceNode(sysBlockDir, devDir, name string) {
	devPath := filepath.Join(devDir, name)

	// If it already exists (devtmpfs), skip.
	if _, err := os.Stat(devPath); err == nil {
		return
	}

	// Read major:minor from sysfs. For partitions, the dev file is at
	// /sys/class/block/<name>/dev (sysfs creates entries for partitions too).
	devFile := filepath.Join(sysBlockDir, name, "dev")
	data, err := os.ReadFile(devFile)
	if err != nil {
		// Partition might not have its own top-level entry in /sys/class/block.
		// This is fine; devtmpfs typically handles these.
		return
	}

	major, minor, err := parseMajorMinor(strings.TrimSpace(string(data)))
	if err != nil {
		slog.Debug("devpopulate: parse dev failed", "device", name, "error", err)
		return
	}

	dev := unix.Mkdev(major, minor)
	// Create as a block device (S_IFBLK), mode 0660.
	err = unix.Mknod(devPath, unix.S_IFBLK|0o660, int(dev))
	if err != nil && !os.IsExist(err) {
		slog.Debug("devpopulate: mknod failed", "device", devPath, "error", err)
	}
}

// parseMajorMinor parses a "major:minor" string from sysfs.
func parseMajorMinor(s string) (uint32, uint32, error) {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid dev format: %q", s)
	}
	major, err := strconv.ParseUint(parts[0], 10, 32)
	if err != nil {
		return 0, 0, fmt.Errorf("parse major %q: %w", parts[0], err)
	}
	minor, err := strconv.ParseUint(parts[1], 10, 32)
	if err != nil {
		return 0, 0, fmt.Errorf("parse minor %q: %w", parts[1], err)
	}
	return uint32(major), uint32(minor), nil
}

// createSymlink creates a relative symlink:
// <devDir>/disk/<subdir>/<name> -> ../../<devName>
func createSymlink(devDir, subdir, name, devName string) error {
	dir := filepath.Join(devDir, "disk", subdir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}

	linkPath := filepath.Join(dir, name)
	target := filepath.Join("..", "..", devName)

	// Remove existing symlink if present (idempotent).
	_ = os.Remove(linkPath)

	if err := os.Symlink(target, linkPath); err != nil {
		return fmt.Errorf("symlink %s -> %s: %w", linkPath, target, err)
	}
	return nil
}

// isPartition checks if childName is a partition of parentName.
// Partition naming conventions: parentName followed by a digit or 'p' + digit.
// Examples: vda -> vda1, nvme0n1 -> nvme0n1p1, sda -> sda1.
func isPartition(parentName, childName string) bool {
	if len(childName) <= len(parentName) {
		return false
	}
	prefix := childName[:len(parentName)]
	if prefix != parentName {
		return false
	}
	suffix := childName[len(parentName):]
	if len(suffix) == 0 {
		return false
	}
	// Must start with a digit or 'p' followed by a digit.
	if suffix[0] >= '0' && suffix[0] <= '9' {
		return true
	}
	if suffix[0] == 'p' && len(suffix) > 1 && suffix[1] >= '0' && suffix[1] <= '9' {
		return true
	}
	return false
}
