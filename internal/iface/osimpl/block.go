//go:build linux

package osimpl

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SysfsBlockProber implements iface.BlockProber using Linux sysfs/devfs.
type SysfsBlockProber struct{}

// FindByLabel returns the device path for a filesystem label by reading the
// symlink at /dev/disk/by-label/<label>.
func (SysfsBlockProber) FindByLabel(label string) (string, error) {
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
			strings.HasPrefix(name, "dm-") {
			continue
		}
		disks = append(disks, name)
	}
	return disks, nil
}
