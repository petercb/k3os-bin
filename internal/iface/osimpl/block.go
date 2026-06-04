//go:build linux

package osimpl

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/siderolabs/go-blockdevice/v2/blkid"
)

// SysfsBlockProber implements iface.BlockProber using Linux sysfs/devfs.
type SysfsBlockProber struct{}

// FindByLabel returns the device path for a filesystem label by reading the
// symlink at /dev/disk/by-label/<label>.
func (SysfsBlockProber) FindByLabel(label string) (string, error) {
	if strings.Contains(label, "/") || strings.Contains(label, "..") {
		return "", fmt.Errorf("invalid label %q: must not contain '/' or '..'", label)
	}

	linkPath := filepath.Join("/dev/disk/by-label", label)
	target, err := os.Readlink(linkPath)
	if err != nil {
		return "", fmt.Errorf("readlink %s: %w", linkPath, err)
	}

	if filepath.IsAbs(target) {
		return filepath.Clean(target), nil
	}

	return filepath.Clean(filepath.Join("/dev/disk/by-label", target)), nil
}

// ListDisks returns device names of all block devices of type "disk" by
// reading /sys/block/ and filtering out virtual devices.
func (SysfsBlockProber) ListDisks() ([]string, error) {
	entries, err := os.ReadDir("/sys/block")
	if err != nil {
		return nil, fmt.Errorf("readdir /sys/block: %w", err)
	}

	var disks []string
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, "loop") ||
			strings.HasPrefix(name, "ram") ||
			strings.HasPrefix(name, "dm-") ||
			strings.HasPrefix(name, "zram") ||
			strings.HasPrefix(name, "nbd") ||
			strings.HasPrefix(name, "sr") ||
			strings.HasPrefix(name, "md") {
			continue
		}
		disks = append(disks, name)
	}
	return disks, nil
}

// ProbeFS returns the filesystem type name for a block device path using
// go-blockdevice/v2's blkid probe. Returns empty string if the filesystem
// type cannot be determined.
func (SysfsBlockProber) ProbeFS(device string) string {
	info, err := blkid.ProbePath(device, blkid.WithSkipLocking(true))
	if err != nil {
		return ""
	}
	return info.Name
}
