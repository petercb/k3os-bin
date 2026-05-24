package osimpl

import (
	"os"

	"github.com/petercb/k3os-bin/internal/iface"
)

// OSFileSystem implements iface.FileSystem using real OS calls.
type OSFileSystem struct{}

// WriteFile writes data to a file using os.WriteFile.
func (OSFileSystem) WriteFile(name string, data []byte, perm os.FileMode) error {
	return os.WriteFile(name, data, perm)
}

// ReadFile reads a file using os.ReadFile.
func (OSFileSystem) ReadFile(name string) ([]byte, error) { return os.ReadFile(name) }

// MkdirAll creates a directory tree using os.MkdirAll.
func (OSFileSystem) MkdirAll(path string, perm os.FileMode) error { return os.MkdirAll(path, perm) }

// Stat returns file information using os.Stat.
func (OSFileSystem) Stat(name string) (os.FileInfo, error) { return os.Stat(name) }

// Open opens a file using os.Open.
func (OSFileSystem) Open(name string) (iface.File, error) { return os.Open(name) }

// Create creates a file using os.Create.
func (OSFileSystem) Create(name string) (iface.File, error) { return os.Create(name) }

// CreateTemp creates a temporary file using os.CreateTemp.
func (OSFileSystem) CreateTemp(dir, pattern string) (iface.File, error) {
	return os.CreateTemp(dir, pattern)
}

// Chown changes file ownership using os.Chown.
func (OSFileSystem) Chown(name string, uid, gid int) error { return os.Chown(name, uid, gid) }

// Chmod changes file permissions using os.Chmod.
func (OSFileSystem) Chmod(name string, mode os.FileMode) error { return os.Chmod(name, mode) }

// Rename renames a path using os.Rename.
func (OSFileSystem) Rename(oldpath, newpath string) error { return os.Rename(oldpath, newpath) }

// Remove removes a path using os.Remove.
func (OSFileSystem) Remove(name string) error { return os.Remove(name) }

// Hostname returns the current hostname using os.Hostname.
func (OSFileSystem) Hostname() (string, error) { return os.Hostname() }
