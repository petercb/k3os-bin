//go:build linux
// +build linux

package osimpl

import "syscall"

// LinuxHostnameSetter implements iface.HostnameSetter using the syscall.
type LinuxHostnameSetter struct{}

// SetHostname sets the system hostname through the Linux syscall.
func (LinuxHostnameSetter) SetHostname(name string) error {
	return syscall.Sethostname([]byte(name))
}
