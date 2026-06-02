//go:build linux

package mount

import "golang.org/x/sys/unix"

// Unmount removes the filesystem mounted at target.
func Unmount(target string, flags int) error {
	return unix.Unmount(target, flags)
}
