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

func TestShellHandler_Execute_Success(t *testing.T) {
	t.Parallel()

	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}
	mnt := &MockMounter{}
	proc := &MockProcessExecutor{}

	deps := &Deps{
		FS:            fs,
		Cmd:           cmd,
		Mounter:       mnt,
		Proc:          proc,
		KernelVersion: "5.15.0",
		SleepFunc:     func(time.Duration) {},
	}
	h := NewShellHandler(deps)

	// Live setup: SetupBase - ISO found
	cmd.On("RunOutput", "blkid", "-L", "K3OS").Return("/dev/sr0", nil)
	mnt.On("Mount", "/dev/sr0", baseDir, "", "ro").Return(nil)

	// SetupKernel - not exists
	fs.On("Stat", "/k3os/system/kernel/5.15.0/kernel.squashfs").Return(nil, os.ErrNotExist)

	// SetupPasswd
	cmd.On("Run", "passwd", "-d", "rancher").Return(nil)

	// SetupMotd
	fs.On("ReadFile", motdPath).Return(nil, os.ErrNotExist)
	fs.On("WriteFile", motdPath, []byte(motdContent), os.FileMode(0o644)).Return(nil)

	// Exec bash
	proc.On("Exec", "/bin/bash", []string{"/bin/bash"}, mock.AnythingOfType("[]string")).Return(nil)

	err := h.Execute()
	require.Error(t, err)

	var execErr *ErrExecCalled
	require.ErrorAs(t, err, &execErr)
	assert.Equal(t, "/bin/bash", execErr.Path)
}

func TestShellHandler_Execute_LiveSetupFails(t *testing.T) {
	t.Parallel()

	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}
	mnt := &MockMounter{}
	proc := &MockProcessExecutor{}

	deps := &Deps{
		FS:            fs,
		Cmd:           cmd,
		Mounter:       mnt,
		Proc:          proc,
		KernelVersion: "5.15.0",
		SleepFunc:     func(time.Duration) {},
	}
	h := NewShellHandler(deps)

	// Live setup fails: blkid fails and USB probe fails
	cmd.On("RunOutput", "blkid", "-L", "K3OS").Return("", errors.New("not found"))
	cmd.On("RunOutput", "lsblk", "-o", "NAME,TYPE", "-n").Return("", nil)

	err := h.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "live setup")

	proc.AssertNotCalled(t, "Exec", mock.Anything, mock.Anything, mock.Anything)
}

func TestShellHandler_Execute_ExecFails(t *testing.T) {
	t.Parallel()

	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}
	mnt := &MockMounter{}
	proc := &MockProcessExecutor{}

	deps := &Deps{
		FS:            fs,
		Cmd:           cmd,
		Mounter:       mnt,
		Proc:          proc,
		KernelVersion: "5.15.0",
		SleepFunc:     func(time.Duration) {},
	}
	h := NewShellHandler(deps)

	// Live setup succeeds
	cmd.On("RunOutput", "blkid", "-L", "K3OS").Return("/dev/sr0", nil)
	mnt.On("Mount", "/dev/sr0", baseDir, "", "ro").Return(nil)
	fs.On("Stat", "/k3os/system/kernel/5.15.0/kernel.squashfs").Return(nil, os.ErrNotExist)
	cmd.On("Run", "passwd", "-d", "rancher").Return(nil)
	fs.On("ReadFile", motdPath).Return(nil, os.ErrNotExist)
	fs.On("WriteFile", motdPath, []byte(motdContent), os.FileMode(0o644)).Return(nil)

	// Exec bash fails
	proc.On("Exec", "/bin/bash", []string{"/bin/bash"}, mock.AnythingOfType("[]string")).Return(errors.New("exec failed"))

	err := h.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exec failed")
}
