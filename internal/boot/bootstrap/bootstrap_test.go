//go:build linux

package bootstrap

import (
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// SetupEtc
// ---------------------------------------------------------------------------

func TestSetupEtc_Success(t *testing.T) {
	t.Parallel()

	fs := &MockFileSystem{}
	mnt := &MockMounter{}
	cmd := &MockCommandRunner{}

	b := &Bootstrapper{FS: fs, Mounter: mnt, Cmd: cmd}

	fs.On("MkdirAll", "/etc", os.FileMode(0o755)).Return(nil)
	fs.On("MkdirAll", "/proc", os.FileMode(0o755)).Return(nil)
	mnt.On("Mount", "none", "/etc", "tmpfs", "").Return(nil)
	mnt.On("Mount", "none", "/proc", "proc", "").Return(nil)
	cmd.On("Run", "cp", "-rfp", "/usr/etc/.", "/etc/").Return(nil)

	err := b.SetupEtc()
	require.NoError(t, err)

	fs.AssertExpectations(t)
	mnt.AssertExpectations(t)
	cmd.AssertExpectations(t)
}

func TestSetupEtc_MkdirEtcFails(t *testing.T) {
	t.Parallel()

	fs := &MockFileSystem{}
	mnt := &MockMounter{}
	cmd := &MockCommandRunner{}

	b := &Bootstrapper{FS: fs, Mounter: mnt, Cmd: cmd}

	fs.On("MkdirAll", "/etc", os.FileMode(0o755)).Return(errors.New("mkdir etc failed"))

	err := b.SetupEtc()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mkdir etc failed")
}

func TestSetupEtc_MountEtcFails(t *testing.T) {
	t.Parallel()

	fs := &MockFileSystem{}
	mnt := &MockMounter{}
	cmd := &MockCommandRunner{}

	b := &Bootstrapper{FS: fs, Mounter: mnt, Cmd: cmd}

	fs.On("MkdirAll", "/etc", os.FileMode(0o755)).Return(nil)
	fs.On("MkdirAll", "/proc", os.FileMode(0o755)).Return(nil)
	mnt.On("Mount", "none", "/etc", "tmpfs", "").Return(errors.New("mount failed"))

	err := b.SetupEtc()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mount failed")
}

// ---------------------------------------------------------------------------
// SetupModules
// ---------------------------------------------------------------------------

func TestSetupModules_BothExist(t *testing.T) {
	t.Parallel()

	fs := &MockFileSystem{}
	mnt := &MockMounter{}
	cmd := &MockCommandRunner{}

	b := &Bootstrapper{FS: fs, Mounter: mnt, Cmd: cmd, KernelVersion: "5.15.0"}

	fs.On("Stat", ".base/lib/modules/5.15.0").Return(fakeFileInfo{}, nil)
	mnt.On("Mount", ".base/lib/modules", "/lib/modules", "", "bind").Return(nil)

	fs.On("Stat", ".base/lib/firmware").Return(fakeFileInfo{}, nil)
	mnt.On("Mount", ".base/lib/firmware", "/lib/firmware", "", "bind").Return(nil)

	err := b.SetupModules()
	require.NoError(t, err)

	fs.AssertExpectations(t)
	mnt.AssertExpectations(t)
}

func TestSetupModules_NeitherExist(t *testing.T) {
	t.Parallel()

	fs := &MockFileSystem{}
	mnt := &MockMounter{}
	cmd := &MockCommandRunner{}

	b := &Bootstrapper{FS: fs, Mounter: mnt, Cmd: cmd, KernelVersion: "5.15.0"}

	fs.On("Stat", ".base/lib/modules/5.15.0").Return(nil, os.ErrNotExist)
	fs.On("Stat", ".base/lib/firmware").Return(nil, os.ErrNotExist)

	err := b.SetupModules()
	require.NoError(t, err)

	mnt.AssertNotCalled(t, "Mount", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestSetupModules_ModulesBindFails(t *testing.T) {
	t.Parallel()

	fs := &MockFileSystem{}
	mnt := &MockMounter{}
	cmd := &MockCommandRunner{}

	b := &Bootstrapper{FS: fs, Mounter: mnt, Cmd: cmd, KernelVersion: "5.15.0"}

	fs.On("Stat", ".base/lib/modules/5.15.0").Return(fakeFileInfo{}, nil)
	mnt.On("Mount", ".base/lib/modules", "/lib/modules", "", "bind").Return(errors.New("bind failed"))

	err := b.SetupModules()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bind failed")
}

// ---------------------------------------------------------------------------
// SetupUsers
// ---------------------------------------------------------------------------

func TestSetupUsers_Success(t *testing.T) {
	t.Parallel()

	fs := &MockFileSystem{}
	mnt := &MockMounter{}
	cmd := &MockCommandRunner{}

	b := &Bootstrapper{FS: fs, Mounter: mnt, Cmd: cmd}

	cmd.On("Run", "sed", "-i", "s!/bin/ash!/bin/bash!", "/etc/passwd").Return(nil)
	cmd.On("Run", "addgroup", "-S", "sudo").Return(nil)
	cmd.On("Run", "sed", "-i", `s/^(sudo:.*)/\1rancher/g`, "/etc/group").Return(nil)
	cmd.On("Run", "addgroup", "-g", "1000", "rancher").Return(nil)
	cmd.On("Run", "adduser", "-s", "/bin/bash", "-u", "1000", "-D", "-G", "rancher", "rancher").Return(nil)
	cmd.On("RunWithStdin", "rancher:*\n", "chpasswd", "-e").Return(nil)

	err := b.SetupUsers()
	require.NoError(t, err)

	cmd.AssertExpectations(t)
}

func TestSetupUsers_AddGroupFails(t *testing.T) {
	t.Parallel()

	fs := &MockFileSystem{}
	mnt := &MockMounter{}
	cmd := &MockCommandRunner{}

	b := &Bootstrapper{FS: fs, Mounter: mnt, Cmd: cmd}

	cmd.On("Run", "sed", "-i", "s!/bin/ash!/bin/bash!", "/etc/passwd").Return(nil)
	cmd.On("Run", "addgroup", "-S", "sudo").Return(errors.New("addgroup failed"))

	err := b.SetupUsers()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "addgroup failed")
}

// ---------------------------------------------------------------------------
// SetupDirs
// ---------------------------------------------------------------------------

func TestSetupDirs_Success(t *testing.T) {
	t.Parallel()

	fs := &MockFileSystem{}
	mnt := &MockMounter{}
	cmd := &MockCommandRunner{}

	b := &Bootstrapper{FS: fs, Mounter: mnt, Cmd: cmd}

	fs.On("MkdirAll", "/run/k3os", os.FileMode(0o755)).Return(nil)

	err := b.SetupDirs()
	require.NoError(t, err)

	fs.AssertExpectations(t)
}

func TestSetupDirs_Fails(t *testing.T) {
	t.Parallel()

	fs := &MockFileSystem{}
	mnt := &MockMounter{}
	cmd := &MockCommandRunner{}

	b := &Bootstrapper{FS: fs, Mounter: mnt, Cmd: cmd}

	fs.On("MkdirAll", "/run/k3os", os.FileMode(0o755)).Return(errors.New("mkdir failed"))

	err := b.SetupDirs()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mkdir failed")
}

// ---------------------------------------------------------------------------
// SetupKernel
// ---------------------------------------------------------------------------

func TestSetupKernel_SquashfsExists(t *testing.T) {
	t.Parallel()

	fs := &MockFileSystem{}
	mnt := &MockMounter{}
	cmd := &MockCommandRunner{}

	b := &Bootstrapper{FS: fs, Mounter: mnt, Cmd: cmd, KernelVersion: "5.15.0"}

	kernelPath := "/k3os/system/kernel/5.15.0/kernel.squashfs"
	fs.On("Stat", kernelPath).Return(fakeFileInfo{}, nil)
	fs.On("MkdirAll", "/run/k3os/kernel", os.FileMode(0o755)).Return(nil)
	mnt.On("Mount", kernelPath, "/run/k3os/kernel", "squashfs", "").Return(nil)
	mnt.On("Mount", "/run/k3os/kernel/lib/modules", "/lib/modules", "", "bind").Return(nil)
	mnt.On("Mount", "/run/k3os/kernel/lib/firmware", "/lib/firmware", "", "bind").Return(nil)
	cmd.On("Run", "umount", "/run/k3os/kernel").Return(nil)

	err := b.SetupKernel()
	require.NoError(t, err)

	fs.AssertExpectations(t)
	mnt.AssertExpectations(t)
	cmd.AssertExpectations(t)
}

func TestSetupKernel_SquashfsNotExists(t *testing.T) {
	t.Parallel()

	fs := &MockFileSystem{}
	mnt := &MockMounter{}
	cmd := &MockCommandRunner{}

	b := &Bootstrapper{FS: fs, Mounter: mnt, Cmd: cmd, KernelVersion: "5.15.0"}

	kernelPath := "/k3os/system/kernel/5.15.0/kernel.squashfs"
	fs.On("Stat", kernelPath).Return(nil, os.ErrNotExist)

	err := b.SetupKernel()
	require.NoError(t, err)

	mnt.AssertNotCalled(t, "Mount", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestSetupKernel_MountSquashfsFails(t *testing.T) {
	t.Parallel()

	fs := &MockFileSystem{}
	mnt := &MockMounter{}
	cmd := &MockCommandRunner{}

	b := &Bootstrapper{FS: fs, Mounter: mnt, Cmd: cmd, KernelVersion: "5.15.0"}

	kernelPath := "/k3os/system/kernel/5.15.0/kernel.squashfs"
	fs.On("Stat", kernelPath).Return(fakeFileInfo{}, nil)
	fs.On("MkdirAll", "/run/k3os/kernel", os.FileMode(0o755)).Return(nil)
	mnt.On("Mount", kernelPath, "/run/k3os/kernel", "squashfs", "").Return(errors.New("mount squashfs failed"))

	err := b.SetupKernel()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mount squashfs failed")
}

// ---------------------------------------------------------------------------
// SetupConfig
// ---------------------------------------------------------------------------

func TestSetupConfig_LocalMode(t *testing.T) {
	t.Parallel()

	fs := &MockFileSystem{}
	mnt := &MockMounter{}
	cmd := &MockCommandRunner{}

	b := &Bootstrapper{FS: fs, Mounter: mnt, Cmd: cmd}

	err := b.SetupConfig("local")
	require.NoError(t, err)

	cmd.AssertNotCalled(t, "Run", mock.Anything)
}

func TestSetupConfig_NonLocalMode(t *testing.T) {
	t.Parallel()

	fs := &MockFileSystem{}
	mnt := &MockMounter{}
	cmd := &MockCommandRunner{}

	b := &Bootstrapper{FS: fs, Mounter: mnt, Cmd: cmd}

	expectedBin := "/k3os/system/k3os/current/k3os"
	cmd.On("Run", expectedBin, "config", "--initrd").Return(nil)

	err := b.SetupConfig("live")
	require.NoError(t, err)

	cmd.AssertExpectations(t)
}

func TestSetupConfig_RunFails(t *testing.T) {
	t.Parallel()

	fs := &MockFileSystem{}
	mnt := &MockMounter{}
	cmd := &MockCommandRunner{}

	b := &Bootstrapper{FS: fs, Mounter: mnt, Cmd: cmd}

	expectedBin := "/k3os/system/k3os/current/k3os"
	cmd.On("Run", expectedBin, "config", "--initrd").Return(errors.New("config failed"))

	err := b.SetupConfig("disk")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "config failed")
}

// ---------------------------------------------------------------------------
// Run
// ---------------------------------------------------------------------------

func TestRun_AllStepsSucceed(t *testing.T) {
	t.Parallel()

	fs := &MockFileSystem{}
	mnt := &MockMounter{}
	cmd := &MockCommandRunner{}

	b := &Bootstrapper{FS: fs, Mounter: mnt, Cmd: cmd, KernelVersion: "5.15.0", Mode: "live"}

	// SetupEtc
	fs.On("MkdirAll", "/etc", os.FileMode(0o755)).Return(nil)
	fs.On("MkdirAll", "/proc", os.FileMode(0o755)).Return(nil)
	mnt.On("Mount", "none", "/etc", "tmpfs", "").Return(nil)
	mnt.On("Mount", "none", "/proc", "proc", "").Return(nil)
	cmd.On("Run", "cp", "-rfp", "/usr/etc/.", "/etc/").Return(nil)

	// SetupModules
	fs.On("Stat", ".base/lib/modules/5.15.0").Return(nil, os.ErrNotExist)
	fs.On("Stat", ".base/lib/firmware").Return(nil, os.ErrNotExist)

	// SetupUsers
	cmd.On("Run", "sed", "-i", "s!/bin/ash!/bin/bash!", "/etc/passwd").Return(nil)
	cmd.On("Run", "addgroup", "-S", "sudo").Return(nil)
	cmd.On("Run", "sed", "-i", `s/^(sudo:.*)/\1rancher/g`, "/etc/group").Return(nil)
	cmd.On("Run", "addgroup", "-g", "1000", "rancher").Return(nil)
	cmd.On("Run", "adduser", "-s", "/bin/bash", "-u", "1000", "-D", "-G", "rancher", "rancher").Return(nil)
	cmd.On("RunWithStdin", "rancher:*\n", "chpasswd", "-e").Return(nil)

	// SetupDirs
	fs.On("MkdirAll", "/run/k3os", os.FileMode(0o755)).Return(nil)

	// SetupKernel
	kernelPath := "/k3os/system/kernel/5.15.0/kernel.squashfs"
	fs.On("Stat", kernelPath).Return(nil, os.ErrNotExist)

	// SetupConfig (non-local mode)
	expectedBin := "/k3os/system/k3os/current/k3os"
	cmd.On("Run", expectedBin, "config", "--initrd").Return(nil)

	err := b.Run()
	require.NoError(t, err)

	fs.AssertExpectations(t)
	mnt.AssertExpectations(t)
	cmd.AssertExpectations(t)
}

func TestRun_StopsOnFirstError(t *testing.T) {
	t.Parallel()

	fs := &MockFileSystem{}
	mnt := &MockMounter{}
	cmd := &MockCommandRunner{}

	b := &Bootstrapper{FS: fs, Mounter: mnt, Cmd: cmd, KernelVersion: "5.15.0", Mode: "live"}

	// SetupEtc fails immediately
	fs.On("MkdirAll", "/etc", os.FileMode(0o755)).Return(errors.New("etc failed"))

	err := b.Run()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "etc failed")

	// No other methods should have been called
	mnt.AssertNotCalled(t, "Mount", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	cmd.AssertNotCalled(t, "Run", mock.Anything)
}
