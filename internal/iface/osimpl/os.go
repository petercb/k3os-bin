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

// Lstat returns file information without following symlinks.
func (OSFileSystem) Lstat(name string) (os.FileInfo, error) { return os.Lstat(name) }

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

// RemoveAll removes a path and all children using os.RemoveAll.
func (OSFileSystem) RemoveAll(path string) error { return os.RemoveAll(path) }

// Symlink creates a symbolic link using os.Symlink.
func (OSFileSystem) Symlink(oldname, newname string) error { return os.Symlink(oldname, newname) }

// Readlink returns the destination of a symlink using os.Readlink.
func (OSFileSystem) Readlink(name string) (string, error) { return os.Readlink(name) }

// ReadDir reads a directory using os.ReadDir and wraps entries.
func (OSFileSystem) ReadDir(name string) ([]iface.DirEntry, error) {
	entries, err := os.ReadDir(name)
	if err != nil {
		return nil, err
	}
	result := make([]iface.DirEntry, len(entries))
	for i, e := range entries {
		result[i] = e
	}
	return result, nil
}

// Hostname returns the current hostname using os.Hostname.
func (OSFileSystem) Hostname() (string, error) { return os.Hostname() }
