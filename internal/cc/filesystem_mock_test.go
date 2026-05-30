package cc

import (
	"bytes"
	"os"

	"github.com/petercb/k3os-bin/internal/iface"
	"github.com/stretchr/testify/mock"
)

// Compile-time interface checks.
var (
	_ iface.FileSystem = (*MockFileSystem)(nil)
	_ iface.File       = (*MockFile)(nil)
)

// MockFile is a testable iface.File implementation. Read and Write operate on
// an internal bytes.Buffer so tests can inspect written content without
// touching the real filesystem. Close and Name are delegated to mock.Mock so
// callers can set expectations on them.
type MockFile struct {
	mock.Mock
	buf bytes.Buffer
}

func (f *MockFile) Read(p []byte) (int, error) {
	return f.buf.Read(p)
}

func (f *MockFile) Write(p []byte) (int, error) {
	return f.buf.Write(p)
}

func (f *MockFile) Close() error {
	return f.Called().Error(0)
}

func (f *MockFile) Name() string {
	return f.Called().String(0)
}

// MockFileSystem is a testable iface.FileSystem implementation backed by
// testify/mock. Methods that return interface values (Open, Create,
// CreateTemp, Stat) guard against nil before type-asserting to avoid panics
// when the mock is configured to return an error with a nil value.
type MockFileSystem struct {
	mock.Mock
}

func (m *MockFileSystem) WriteFile(name string, data []byte, perm os.FileMode) error {
	return m.Called(name, data, perm).Error(0)
}

func (m *MockFileSystem) ReadFile(name string) ([]byte, error) {
	args := m.Called(name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockFileSystem) MkdirAll(path string, perm os.FileMode) error {
	return m.Called(path, perm).Error(0)
}

func (m *MockFileSystem) Stat(name string) (os.FileInfo, error) {
	args := m.Called(name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(os.FileInfo), args.Error(1)
}

func (m *MockFileSystem) Open(name string) (iface.File, error) {
	args := m.Called(name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(iface.File), args.Error(1)
}

func (m *MockFileSystem) Create(name string) (iface.File, error) {
	args := m.Called(name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(iface.File), args.Error(1)
}

func (m *MockFileSystem) CreateTemp(dir, pattern string) (iface.File, error) {
	args := m.Called(dir, pattern)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(iface.File), args.Error(1)
}

func (m *MockFileSystem) Chown(name string, uid, gid int) error {
	return m.Called(name, uid, gid).Error(0)
}

func (m *MockFileSystem) Chmod(name string, mode os.FileMode) error {
	return m.Called(name, mode).Error(0)
}

func (m *MockFileSystem) Rename(oldpath, newpath string) error {
	return m.Called(oldpath, newpath).Error(0)
}

func (m *MockFileSystem) Remove(name string) error {
	return m.Called(name).Error(0)
}

func (m *MockFileSystem) RemoveAll(path string) error {
	return m.Called(path).Error(0)
}

func (m *MockFileSystem) Symlink(oldname, newname string) error {
	return m.Called(oldname, newname).Error(0)
}

func (m *MockFileSystem) Readlink(name string) (string, error) {
	args := m.Called(name)
	return args.String(0), args.Error(1)
}

func (m *MockFileSystem) Lstat(name string) (os.FileInfo, error) {
	args := m.Called(name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(os.FileInfo), args.Error(1)
}

func (m *MockFileSystem) ReadDir(name string) ([]iface.DirEntry, error) {
	args := m.Called(name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]iface.DirEntry), args.Error(1)
}

func (m *MockFileSystem) Hostname() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}
