//go:build linux
// +build linux

package osimpl

import (
	"os"
	"path"
	"strings"
)

// LinuxSysctlApplier implements iface.SysctlApplier by writing to /proc/sys/.
// Root overrides the base directory (default: /proc/sys) and is intended for
// testing only.
type LinuxSysctlApplier struct {
	Root string
}

// Set writes a sysctl value under /proc/sys (or Root if set).
func (a LinuxSysctlApplier) Set(key string, value string) error {
	root := a.Root
	if root == "" {
		root = "/proc/sys"
	}
	elements := []string{root}
	elements = append(elements, strings.Split(key, ".")...)
	p := path.Join(elements...)
	return os.WriteFile(p, []byte(value), 0o644)
}
