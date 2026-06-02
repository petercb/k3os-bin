//go:build linux

// Package namespace provides declarative creation of Linux namespace elements
// such as directories, mounts, device nodes, and symbolic links.
package namespace

import (
	"fmt"
	"os"

	"github.com/petercb/k3os-bin/internal/mount"
	"golang.org/x/sys/unix"
)

// Creator is a single namespace element that can be created.
type Creator interface {
	Create() error
	fmt.Stringer
}

// Dir creates a directory with the given mode.
type Dir struct {
	Name string
	Mode os.FileMode
}

// Create calls os.MkdirAll to create the directory tree.
func (d Dir) Create() error {
	return os.MkdirAll(d.Name, d.Mode)
}

func (d Dir) String() string {
	return fmt.Sprintf("dir{%s %04o}", d.Name, d.Mode)
}

// Mount mounts a filesystem. If Silent is true, errors are swallowed.
type Mount struct {
	Source string
	Target string
	FSType string
	Flags  uintptr
	Data   string
	Silent bool
}

// Create ensures the target directory exists then performs the mount syscall.
func (m Mount) Create() error {
	if err := os.MkdirAll(m.Target, 0o755); err != nil {
		if m.Silent {
			return nil
		}
		return fmt.Errorf("mount mkdir %s: %w", m.Target, err)
	}

	if err := unix.Mount(m.Source, m.Target, m.FSType, m.Flags, m.Data); err != nil {
		if m.Silent {
			return nil
		}
		return fmt.Errorf("mount %s on %s: %w", m.Source, m.Target, err)
	}

	return nil
}

func (m Mount) String() string {
	return fmt.Sprintf("mount{%s on %s type %s}", m.Source, m.Target, m.FSType)
}

// AsMountPoint returns a MountPoint representing this mount for pool tracking.
func (m Mount) AsMountPoint() *mount.Point {
	return &mount.Point{
		Source: m.Source,
		Target: m.Target,
		FSType: m.FSType,
		Flags:  m.Flags,
		Data:   m.Data,
	}
}

// Dev creates a device node.
type Dev struct {
	Name  string
	Mode  uint32
	Major uint32
	Minor uint32
}

// Create calls unix.Mknod to create the device node. The S_IFCHR file-type
// bits are ORed in automatically so callers only specify permission bits.
// If the node already exists, the error is ignored.
func (d Dev) Create() error {
	dev := unix.Mkdev(d.Major, d.Minor)
	if err := unix.Mknod(d.Name, unix.S_IFCHR|d.Mode, int(dev)); err != nil {
		if os.IsExist(err) {
			return nil
		}
		return fmt.Errorf("mknod %s: %w", d.Name, err)
	}
	return nil
}

func (d Dev) String() string {
	return fmt.Sprintf("dev{%s %d:%d}", d.Name, d.Major, d.Minor)
}

// Symlink creates a symbolic link.
type Symlink struct {
	Target  string
	NewPath string
}

// Create calls unix.Symlink to create the symbolic link. If the link already
// exists, the error is ignored for idempotency.
func (s Symlink) Create() error {
	if err := unix.Symlink(s.Target, s.NewPath); err != nil {
		if os.IsExist(err) {
			return nil
		}
		return fmt.Errorf("symlink %s -> %s: %w", s.NewPath, s.Target, err)
	}
	return nil
}

func (s Symlink) String() string {
	return fmt.Sprintf("symlink{%s -> %s}", s.NewPath, s.Target)
}
