//go:build linux

// Package loopdev provides a thin wrapper around Linux loop device ioctls,
// using golang.org/x/sys/unix and u-root for device discovery.
package loopdev

import (
	"fmt"
	"log/slog"
	"os"
	"sync/atomic"
	"unsafe"

	"github.com/petercb/k3os-bin/internal/iface"
	"github.com/u-root/u-root/pkg/mount/loop"
	"golang.org/x/sys/unix"
)

// ioctl command constants for loop device operations.
const (
	loopSetFd       = 0x4C00
	loopClrFd       = 0x4C01
	loopSetStatus64 = 0x4C04
	loopGetStatus64 = 0x4C05
	flagReadOnly    = 1 // LO_FLAGS_READ_ONLY (1 << 0), see linux/loop.h
	flagAutoclear   = 4 // LO_FLAGS_AUTOCLEAR (1 << 2), see linux/loop.h
)

// loopInfo64 mirrors the Linux struct loop_info64.
type loopInfo64 struct {
	Device         uint64
	INode          uint64
	RDevice        uint64
	Offset         uint64
	SizeLimit      uint64
	Number         uint32
	EncryptType    uint32
	EncryptKeySize uint32
	Flags          uint32
	FileName       [64]byte
	CryptName      [64]byte
	EncryptKey     [32]byte
	Init           [2]uint64
}

// deviceFinder abstracts loop device discovery for testability.
type deviceFinder interface {
	FindDevice() (string, error)
}

// urootDeviceFinder is the production implementation using u-root.
type urootDeviceFinder struct{}

func (urootDeviceFinder) FindDevice() (string, error) {
	return loop.FindDevice()
}

// syscaller abstracts low-level system call operations for testability.
type syscaller interface {
	Open(path string, flags int, perm uint32) (int, error)
	Close(fd int) error
	// IoctlSetInt performs an ioctl with an integer argument (e.g., LOOP_SET_FD, LOOP_CLR_FD).
	IoctlSetInt(fd int, req uint, val int) error
	// IoctlLoopGetStatus64 reads loop_info64 from the device.
	IoctlLoopGetStatus64(fd int, info *loopInfo64) error
	// IoctlLoopSetStatus64 writes loop_info64 to the device.
	IoctlLoopSetStatus64(fd int, info *loopInfo64) error
}

// realSyscaller is the production implementation using unix package.
type realSyscaller struct{}

func (realSyscaller) Open(path string, flags int, perm uint32) (int, error) {
	return unix.Open(path, flags, perm)
}

func (realSyscaller) Close(fd int) error {
	return unix.Close(fd)
}

func (realSyscaller) IoctlSetInt(fd int, req uint, val int) error {
	return unix.IoctlSetInt(fd, req, val)
}

func (realSyscaller) IoctlLoopGetStatus64(fd int, info *loopInfo64) error {
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), loopGetStatus64, uintptr(unsafe.Pointer(info)))
	if errno != 0 {
		return errno
	}
	return nil
}

func (realSyscaller) IoctlLoopSetStatus64(fd int, info *loopInfo64) error {
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), loopSetStatus64, uintptr(unsafe.Pointer(info)))
	if errno != 0 {
		return errno
	}
	return nil
}

// Attacher implements iface.LoopAttacher using Linux ioctls.
type Attacher struct {
	sc syscaller
	df deviceFinder
}

// NewAttacher returns an Attacher using real system calls and u-root device discovery.
func NewAttacher() *Attacher {
	return &Attacher{sc: realSyscaller{}, df: urootDeviceFinder{}}
}

// newAttacherWith returns an Attacher using the provided dependencies (for testing).
func newAttacherWith(df deviceFinder, sc syscaller) *Attacher {
	return &Attacher{sc: sc, df: df}
}

// Attach opens the backing file, finds a free loop device, and attaches
// the file to that device with the given offset and read-only flag.
func (a *Attacher) Attach(backingFile string, offset uint64, readOnly bool) (iface.LoopDevice, error) {
	// Open backing file.
	flags := os.O_RDONLY
	if !readOnly {
		flags = os.O_RDWR
	}
	backingFd, err := a.sc.Open(backingFile, flags, 0)
	if err != nil {
		return nil, fmt.Errorf("opening backing file %s: %w", backingFile, err)
	}
	defer func() { _ = a.sc.Close(backingFd) }()

	// Find a free loop device via u-root.
	loopPath, err := a.df.FindDevice()
	if err != nil {
		return nil, fmt.Errorf("finding free loop device: %w", err)
	}

	// Open the loop device.
	loopFd, err := a.sc.Open(loopPath, os.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("opening loop device %s: %w", loopPath, err)
	}

	// Associate the backing file with the loop device via LOOP_SET_FD.
	if err := a.sc.IoctlSetInt(loopFd, loopSetFd, backingFd); err != nil {
		_ = a.sc.Close(loopFd)
		return nil, fmt.Errorf("LOOP_SET_FD: %w", err)
	}

	// Set status (offset, flags) via LOOP_SET_STATUS64.
	info := loopInfo64{
		Offset: offset,
	}
	if readOnly {
		info.Flags |= flagReadOnly
	}
	if err := a.sc.IoctlLoopSetStatus64(loopFd, &info); err != nil {
		// Clean up on failure.
		if clrErr := a.sc.IoctlSetInt(loopFd, loopClrFd, 0); clrErr != nil {
			slog.Error("failed to clear loop device during cleanup", "device", loopPath, "error", clrErr)
		}
		_ = a.sc.Close(loopFd)
		return nil, fmt.Errorf("LOOP_SET_STATUS64: %w", err)
	}

	return &Device{
		path: loopPath,
		fd:   loopFd,
		sc:   a.sc,
	}, nil
}

// Device represents an attached loop device, implementing iface.LoopDevice.
type Device struct {
	path   string
	fd     int
	sc     syscaller
	closed atomic.Bool
}

// Path returns the path of the loop device (e.g., "/dev/loop0").
func (d *Device) Path() string {
	return d.path
}

// Detach removes the association between the loop device and its backing file.
func (d *Device) Detach() error {
	if !d.closed.CompareAndSwap(false, true) {
		return nil
	}
	if err := d.sc.IoctlSetInt(d.fd, loopClrFd, 0); err != nil {
		d.closed.Store(false) // revert on failure so retry is possible
		return fmt.Errorf("LOOP_CLR_FD on %s: %w", d.path, err)
	}
	if err := d.sc.Close(d.fd); err != nil {
		return fmt.Errorf("closing loop device fd %s: %w", d.path, err)
	}
	return nil
}

// SetAutoclear sets the LO_FLAGS_AUTOCLEAR flag on the loop device so the
// kernel automatically detaches when the last reference is dropped.
func (d *Device) SetAutoclear() error {
	if d.closed.Load() {
		return nil
	}
	var info loopInfo64
	if err := d.sc.IoctlLoopGetStatus64(d.fd, &info); err != nil {
		return fmt.Errorf("LOOP_GET_STATUS64 on %s: %w", d.path, err)
	}
	info.Flags |= flagAutoclear
	if err := d.sc.IoctlLoopSetStatus64(d.fd, &info); err != nil {
		return fmt.Errorf("LOOP_SET_STATUS64 on %s: %w", d.path, err)
	}
	return nil
}
