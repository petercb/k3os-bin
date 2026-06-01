//go:build linux

package osimpl

import (
	"fmt"

	"golang.org/x/sys/unix"
)

// loopClrFd is the LOOP_CLR_FD ioctl command constant.
const loopClrFd = 0x4C01

// LoopPathDetacher implements iface.LoopDetacher using the LOOP_CLR_FD ioctl.
type LoopPathDetacher struct{}

// DetachPath opens the given loop device and issues LOOP_CLR_FD to detach it.
func (LoopPathDetacher) DetachPath(device string) error {
	fd, err := unix.Open(device, unix.O_RDONLY, 0)
	if err != nil {
		return fmt.Errorf("open %s: %w", device, err)
	}
	defer unix.Close(fd) //nolint:errcheck

	if err := unix.IoctlSetInt(fd, loopClrFd, 0); err != nil {
		return fmt.Errorf("LOOP_CLR_FD on %s: %w", device, err)
	}
	return nil
}
