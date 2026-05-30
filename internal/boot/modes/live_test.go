//go:build linux

package modes

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestLiveSetup_SetupBase_ISOLabel(t *testing.T) {
	t.Parallel()

	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}
	mnt := &MockMounter{}

	deps := &Deps{FS: fs, Cmd: cmd, Mounter: mnt, SleepFunc: func(time.Duration) {}}
	l := NewLiveSetup(deps)

	cmd.On("RunOutput", "blkid", "-L", "K3OS").Return("/dev/sr0", nil)
	mnt.On("Mount", "/dev/sr0", baseDir, "", "ro").Return(nil)

	err := l.SetupBase()
	require.NoError(t, err)

	cmd.AssertExpectations(t)
	mnt.AssertExpectations(t)
}

func TestLiveSetup_SetupBase_USBProbeSuccess(t *testing.T) {
	t.Parallel()

	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}
	mnt := &MockMounter{}

	deps := &Deps{FS: fs, Cmd: cmd, Mounter: mnt, SleepFunc: func(time.Duration) {}}
	l := NewLiveSetup(deps)

	// blkid fails (no ISO label)
	cmd.On("RunOutput", "blkid", "-L", "K3OS").Return("", errors.New("not found"))
	// lsblk returns disks
	cmd.On("RunOutput", "lsblk", "-o", "NAME,TYPE", "-n").Return("sda  disk\nsda1 part\nsdb  disk", nil)
	// First disk fails, second succeeds
	mnt.On("Mount", "/dev/sda", baseDir, "", "").Return(errors.New("no fs"))
	mnt.On("Mount", "/dev/sdb", baseDir, "", "").Return(nil)

	err := l.SetupBase()
	require.NoError(t, err)

	mnt.AssertExpectations(t)
}

func TestLiveSetup_SetupBase_USBRetry(t *testing.T) {
	t.Parallel()

	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}
	mnt := &MockMounter{}

	sleepCount := 0
	deps := &Deps{FS: fs, Cmd: cmd, Mounter: mnt, SleepFunc: func(time.Duration) { sleepCount++ }}
	l := NewLiveSetup(deps)

	cmd.On("RunOutput", "blkid", "-L", "K3OS").Return("", errors.New("not found"))
	// First 4 attempts: no disks found
	cmd.On("RunOutput", "lsblk", "-o", "NAME,TYPE", "-n").Return("", nil).Times(4)
	// 5th attempt: disk found
	cmd.On("RunOutput", "lsblk", "-o", "NAME,TYPE", "-n").Return("sda  disk", nil).Once()
	mnt.On("Mount", "/dev/sda", baseDir, "", "").Return(nil)

	err := l.SetupBase()
	require.NoError(t, err)
	assert.Equal(t, 4, sleepCount) // slept 4 times before finding disk on 5th try
}

func TestLiveSetup_SetupBase_AllRetriesFail(t *testing.T) {
	t.Parallel()

	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}
	mnt := &MockMounter{}

	deps := &Deps{FS: fs, Cmd: cmd, Mounter: mnt, SleepFunc: func(time.Duration) {}}
	l := NewLiveSetup(deps)

	cmd.On("RunOutput", "blkid", "-L", "K3OS").Return("", errors.New("not found"))
	cmd.On("RunOutput", "lsblk", "-o", "NAME,TYPE", "-n").Return("sda  disk", nil)
	mnt.On("Mount", "/dev/sda", baseDir, "", "").Return(errors.New("fail"))

	err := l.SetupBase()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to mount base filesystem")
}

func TestLiveSetup_SetupKernel_SquashfsExists(t *testing.T) {
	t.Parallel()

	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}
	mnt := &MockMounter{}

	deps := &Deps{FS: fs, Cmd: cmd, Mounter: mnt, KernelVersion: "5.15.0"}
	l := NewLiveSetup(deps)

	kernelPath := "/k3os/system/kernel/5.15.0/kernel.squashfs"
	fs.On("Stat", kernelPath).Return(fakeFileInfo{}, nil)
	fs.On("MkdirAll", "/run/k3os/kernel", os.FileMode(0o755)).Return(nil)
	mnt.On("Mount", kernelPath, "/run/k3os/kernel", "squashfs", "").Return(nil)
	mnt.On("Mount", "/run/k3os/kernel/lib/modules", "/lib/modules", "", "bind").Return(nil)
	mnt.On("Mount", "/run/k3os/kernel/lib/firmware", "/lib/firmware", "", "bind").Return(nil)
	cmd.On("Run", "umount", "/run/k3os/kernel").Return(nil)

	err := l.SetupKernel()
	require.NoError(t, err)

	fs.AssertExpectations(t)
	mnt.AssertExpectations(t)
}

func TestLiveSetup_SetupKernel_NotExists(t *testing.T) {
	t.Parallel()

	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}
	mnt := &MockMounter{}

	deps := &Deps{FS: fs, Cmd: cmd, Mounter: mnt, KernelVersion: "5.15.0"}
	l := NewLiveSetup(deps)

	kernelPath := "/k3os/system/kernel/5.15.0/kernel.squashfs"
	fs.On("Stat", kernelPath).Return(nil, os.ErrNotExist)

	err := l.SetupKernel()
	require.NoError(t, err)

	mnt.AssertNotCalled(t, "Mount", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestLiveSetup_SetupPasswd_Success(t *testing.T) {
	t.Parallel()

	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}
	mnt := &MockMounter{}

	deps := &Deps{FS: fs, Cmd: cmd, Mounter: mnt}
	l := NewLiveSetup(deps)

	cmd.On("Run", "passwd", "-d", "rancher").Return(nil)

	err := l.SetupPasswd()
	require.NoError(t, err)

	cmd.AssertExpectations(t)
}

func TestLiveSetup_SetupPasswd_Fails(t *testing.T) {
	t.Parallel()

	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}
	mnt := &MockMounter{}

	deps := &Deps{FS: fs, Cmd: cmd, Mounter: mnt}
	l := NewLiveSetup(deps)

	cmd.On("Run", "passwd", "-d", "rancher").Return(errors.New("passwd failed"))

	err := l.SetupPasswd()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "passwd failed")
}

func TestLiveSetup_SetupMotd(t *testing.T) {
	t.Parallel()

	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}
	mnt := &MockMounter{}

	deps := &Deps{FS: fs, Cmd: cmd, Mounter: mnt}
	l := NewLiveSetup(deps)

	fs.On("ReadFile", motdPath).Return([]byte("Welcome\n"), nil)
	expectedContent := "Welcome\n" + motdContent
	fs.On("WriteFile", motdPath, []byte(expectedContent), os.FileMode(0o644)).Return(nil)

	err := l.SetupMotd()
	require.NoError(t, err)

	fs.AssertExpectations(t)
}

func TestLiveSetup_SetupMotd_NoExistingFile(t *testing.T) {
	t.Parallel()

	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}
	mnt := &MockMounter{}

	deps := &Deps{FS: fs, Cmd: cmd, Mounter: mnt}
	l := NewLiveSetup(deps)

	fs.On("ReadFile", motdPath).Return(nil, os.ErrNotExist)
	fs.On("WriteFile", motdPath, []byte(motdContent), os.FileMode(0o644)).Return(nil)

	err := l.SetupMotd()
	require.NoError(t, err)
}

func TestLiveHandler_Execute(t *testing.T) {
	t.Parallel()

	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}
	mnt := &MockMounter{}

	deps := &Deps{FS: fs, Cmd: cmd, Mounter: mnt, KernelVersion: "5.15.0", SleepFunc: func(time.Duration) {}}
	h := NewLiveHandler(deps)

	// SetupBase - ISO found
	cmd.On("RunOutput", "blkid", "-L", "K3OS").Return("/dev/sr0", nil)
	mnt.On("Mount", "/dev/sr0", baseDir, "", "ro").Return(nil)

	// SetupKernel - not exists
	fs.On("Stat", "/k3os/system/kernel/5.15.0/kernel.squashfs").Return(nil, os.ErrNotExist)

	// SetupPasswd
	cmd.On("Run", "passwd", "-d", "rancher").Return(nil)

	// SetupMotd
	fs.On("ReadFile", motdPath).Return(nil, os.ErrNotExist)
	fs.On("WriteFile", motdPath, []byte(motdContent), os.FileMode(0o644)).Return(nil)

	err := h.Execute()
	require.NoError(t, err)
}

func TestInstallHandler_Execute(t *testing.T) {
	t.Parallel()

	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}
	mnt := &MockMounter{}

	deps := &Deps{FS: fs, Cmd: cmd, Mounter: mnt, KernelVersion: "5.15.0", SleepFunc: func(time.Duration) {}}
	h := NewInstallHandler(deps)

	// Same as live
	cmd.On("RunOutput", "blkid", "-L", "K3OS").Return("/dev/sr0", nil)
	mnt.On("Mount", "/dev/sr0", baseDir, "", "ro").Return(nil)
	fs.On("Stat", "/k3os/system/kernel/5.15.0/kernel.squashfs").Return(nil, os.ErrNotExist)
	cmd.On("Run", "passwd", "-d", "rancher").Return(nil)
	fs.On("ReadFile", motdPath).Return(nil, os.ErrNotExist)
	fs.On("WriteFile", motdPath, []byte(motdContent), os.FileMode(0o644)).Return(nil)

	err := h.Execute()
	require.NoError(t, err)
}

func TestParseDisks(t *testing.T) {
	t.Parallel()

	input := "sda    disk\nsda1   part\nsdb    disk\nnvme0n1 disk"
	disks := parseDisks(input)
	assert.Equal(t, []string{"sda", "sdb", "nvme0n1"}, disks)
}

func TestParseDisks_Empty(t *testing.T) {
	t.Parallel()

	disks := parseDisks("")
	assert.Empty(t, disks)
}
