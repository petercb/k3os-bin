//go:build linux

// Package virt provides pure Go virtualization detection using DMI/SMBIOS sysfs data.
package virt

import (
	"os"
	"strings"
)

// DMIDetector detects virtualization by reading DMI/SMBIOS data from sysfs.
type DMIDetector struct {
	// BasePath is the directory containing DMI identity files.
	// Defaults to "/sys/class/dmi/id/".
	BasePath string

	// ReadFile is the function used to read files.
	// Defaults to os.ReadFile.
	ReadFile func(string) ([]byte, error)
}

// NewDMIDetector returns a DMIDetector with default settings.
func NewDMIDetector() *DMIDetector {
	return &DMIDetector{
		BasePath: "/sys/class/dmi/id/",
		ReadFile: os.ReadFile,
	}
}

// Detect reads DMI sysfs files and returns detected virtualization types.
// If the files cannot be read (e.g., not running on Linux or no permissions),
// it returns nil, nil (non-fatal).
func (d *DMIDetector) Detect() ([]string, error) {
	sysVendor := d.readDMIFile("sys_vendor")
	productName := d.readDMIFile("product_name")
	boardVendor := d.readDMIFile("board_vendor")

	// Check for QEMU/KVM — return single canonical ID to avoid duplicate symlinks
	// in the consumer (services.go matches both "kvm" and "qemu" to the same case).
	if containsCI(sysVendor, "QEMU") {
		return []string{"kvm"}, nil
	}

	// Check for Microsoft Hyper-V
	if containsCI(sysVendor, "Microsoft") && containsCI(productName, "Virtual Machine") {
		return []string{"hyperv"}, nil
	}

	// Check for VMware
	if containsCI(sysVendor, "VMware") {
		return []string{"vmware"}, nil
	}

	// Check for VirtualBox (innotek GmbH or Oracle with VirtualBox product).
	// Detected for informational purposes; no service enablement case exists
	// in services.go because k3OS does not ship VirtualBox guest additions.
	if containsCI(sysVendor, "innotek") || containsCI(boardVendor, "innotek") ||
		((containsCI(sysVendor, "Oracle") || containsCI(boardVendor, "Oracle")) &&
			containsCI(productName, "VirtualBox")) {
		return []string{"virtualbox"}, nil
	}

	return nil, nil
}

// readDMIFile reads a single DMI file and returns its trimmed content.
// Returns an empty string on any error.
func (d *DMIDetector) readDMIFile(name string) string {
	data, err := d.ReadFile(d.BasePath + name)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// containsCI reports whether s contains substr, case-insensitive.
func containsCI(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}
