//go:build !linux

package virt

import "os"

// DMIDetector detects virtualization by reading DMI/SMBIOS data from sysfs.
// On non-Linux platforms, Detect always returns nil.
type DMIDetector struct {
	// BasePath is the directory containing DMI identity files.
	BasePath string

	// ReadFile is the function used to read files.
	ReadFile func(string) ([]byte, error)
}

// NewDMIDetector returns a DMIDetector with default settings.
func NewDMIDetector() *DMIDetector {
	return &DMIDetector{
		BasePath: "/sys/class/dmi/id/",
		ReadFile: os.ReadFile,
	}
}

// Detect is a stub for non-Linux platforms. It always returns nil, nil.
func (d *DMIDetector) Detect() ([]string, error) {
	return nil, nil
}
