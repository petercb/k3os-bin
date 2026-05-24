//go:build linux
// +build linux

package osimpl

import (
	"os"
	"path"
	"strings"
)

// LinuxSysctlApplier implements iface.SysctlApplier by writing to /proc/sys/.
type LinuxSysctlApplier struct{}

// Set writes a sysctl value under /proc/sys.
func (LinuxSysctlApplier) Set(key string, value string) error {
	elements := []string{"/proc", "sys"}
	elements = append(elements, strings.Split(key, ".")...)
	p := path.Join(elements...)
	return os.WriteFile(p, []byte(value), 0o644)
}
