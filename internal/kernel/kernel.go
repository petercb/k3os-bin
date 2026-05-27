//go:build linux
// +build linux

package kernel

import "golang.org/x/sys/unix"

// GetKernelVersion returns the kernel release string (e.g. "5.15.0-generic")
// by reading the uname system call.
func GetKernelVersion() (string, error) {
	var utsname unix.Utsname
	if err := unix.Uname(&utsname); err != nil {
		return "", err
	}
	return unix.ByteSliceToString(utsname.Release[:]), nil
}
