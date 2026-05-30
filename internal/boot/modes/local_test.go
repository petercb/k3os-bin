//go:build linux

package modes

import (
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLocalHandler_SetupSSH_PersistDirNotExists(t *testing.T) {
	t.Parallel()

	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}
	mnt := &MockMounter{}

	copyDirCalled := false
	deps := &Deps{FS: fs, Cmd: cmd, Mounter: mnt, CopyDir: func(src, dst string) error {
		copyDirCalled = true
		assert.Equal(t, "/etc/ssh", src)
		assert.Equal(t, "/var/lib/rancher/k3os/ssh", dst)
		return nil
	}}
	h := NewLocalHandler(deps)

	// Persist dir does not exist
	fs.On("Stat", "/var/lib/rancher/k3os/ssh").Return(nil, os.ErrNotExist)
	fs.On("MkdirAll", "/var/lib/rancher/k3os", os.FileMode(0o755)).Return(nil)
	fs.On("RemoveAll", "/etc/ssh").Return(nil)
	fs.On("Symlink", "/var/lib/rancher/k3os/ssh", "/etc/ssh").Return(nil)

	err := h.SetupSSH()
	require.NoError(t, err)
	assert.True(t, copyDirCalled)

	fs.AssertExpectations(t)
}

func TestLocalHandler_SetupSSH_PersistDirExists(t *testing.T) {
	t.Parallel()

	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}
	mnt := &MockMounter{}

	copyDirCalled := false
	deps := &Deps{FS: fs, Cmd: cmd, Mounter: mnt, CopyDir: func(_, _ string) error {
		copyDirCalled = true
		return nil
	}}
	h := NewLocalHandler(deps)

	// Persist dir already exists
	fs.On("Stat", "/var/lib/rancher/k3os/ssh").Return(fakeFileInfo{isDir: true}, nil)
	fs.On("RemoveAll", "/etc/ssh").Return(nil)
	fs.On("Symlink", "/var/lib/rancher/k3os/ssh", "/etc/ssh").Return(nil)

	err := h.SetupSSH()
	require.NoError(t, err)

	// Should not have called CopyDir
	assert.False(t, copyDirCalled)
}

func TestLocalHandler_SetupSSH_CopyFails(t *testing.T) {
	t.Parallel()

	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}
	mnt := &MockMounter{}

	deps := &Deps{FS: fs, Cmd: cmd, Mounter: mnt, CopyDir: func(_, _ string) error {
		return errors.New("copy failed")
	}}
	h := NewLocalHandler(deps)

	fs.On("Stat", "/var/lib/rancher/k3os/ssh").Return(nil, os.ErrNotExist)
	fs.On("MkdirAll", "/var/lib/rancher/k3os", os.FileMode(0o755)).Return(nil)

	err := h.SetupSSH()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "copy failed")
}

func TestLocalHandler_SetupRancherNode_Success(t *testing.T) {
	t.Parallel()

	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}
	mnt := &MockMounter{}

	deps := &Deps{FS: fs, Cmd: cmd, Mounter: mnt}
	h := NewLocalHandler(deps)

	fs.On("MkdirAll", "/etc/rancher", os.FileMode(0o755)).Return(nil)
	fs.On("MkdirAll", "/var/lib/rancher/k3os/node", os.FileMode(0o755)).Return(nil)
	fs.On("Symlink", "/var/lib/rancher/k3os/node", "/etc/rancher/node").Return(nil)

	err := h.SetupRancherNode()
	require.NoError(t, err)

	fs.AssertExpectations(t)
}

func TestLocalHandler_SetupRancherNode_MkdirFails(t *testing.T) {
	t.Parallel()

	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}
	mnt := &MockMounter{}

	deps := &Deps{FS: fs, Cmd: cmd, Mounter: mnt}
	h := NewLocalHandler(deps)

	fs.On("MkdirAll", "/etc/rancher", os.FileMode(0o755)).Return(errors.New("mkdir failed"))

	err := h.SetupRancherNode()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mkdir failed")
}

func TestLocalHandler_Execute_Success(t *testing.T) {
	t.Parallel()

	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}
	mnt := &MockMounter{}

	deps := &Deps{FS: fs, Cmd: cmd, Mounter: mnt, CopyDir: func(_, _ string) error { return nil }}
	h := NewLocalHandler(deps)

	// SetupSSH - persist dir exists already
	fs.On("Stat", "/var/lib/rancher/k3os/ssh").Return(fakeFileInfo{isDir: true}, nil)
	fs.On("RemoveAll", "/etc/ssh").Return(nil)
	fs.On("Symlink", "/var/lib/rancher/k3os/ssh", "/etc/ssh").Return(nil)

	// SetupRancherNode
	fs.On("MkdirAll", "/etc/rancher", os.FileMode(0o755)).Return(nil)
	fs.On("MkdirAll", "/var/lib/rancher/k3os/node", os.FileMode(0o755)).Return(nil)
	fs.On("Symlink", "/var/lib/rancher/k3os/node", "/etc/rancher/node").Return(nil)

	err := h.Execute()
	require.NoError(t, err)
}
