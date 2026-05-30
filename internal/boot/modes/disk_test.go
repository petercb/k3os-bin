//go:build linux

package modes

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/petercb/k3os-bin/internal/iface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestDiskHandler_SetupMounts_Success(t *testing.T) {
	t.Parallel()

	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}
	mnt := &MockMounter{}

	deps := &Deps{FS: fs, Cmd: cmd, Mounter: mnt}
	h := NewDiskHandler(deps)

	fs.On("MkdirAll", targetDir, os.FileMode(0o755)).Return(nil)
	mnt.On("Mount", "LABEL=K3OS_STATE", targetDir, "", "").Return(nil)
	// No growpart marker
	fs.On("ReadFile", targetDir+"/k3os/system/growpart").Return(nil, os.ErrNotExist)

	err := h.SetupMounts()
	require.NoError(t, err)

	fs.AssertExpectations(t)
	mnt.AssertExpectations(t)
}

func TestDiskHandler_SetupMounts_WithGrowpart(t *testing.T) {
	t.Parallel()

	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}
	mnt := &MockMounter{}

	deps := &Deps{FS: fs, Cmd: cmd, Mounter: mnt}
	h := NewDiskHandler(deps)

	fs.On("MkdirAll", targetDir, os.FileMode(0o755)).Return(nil)
	mnt.On("Mount", "LABEL=K3OS_STATE", targetDir, "", "").Return(nil).Once()
	fs.On("ReadFile", targetDir+"/k3os/system/growpart").Return([]byte("/dev/sda 2"), nil)
	fs.On("Stat", "/dev/sda2").Return(fakeFileInfo{}, nil)
	cmd.On("Run", "umount", targetDir).Return(nil)
	cmd.On("Run", "parted", "/dev/sda", "resizepart", "2", "100%").Return(nil)
	cmd.On("Run", "partprobe", "/dev/sda").Return(nil)
	cmd.On("Run", "e2fsck", "-f", "/dev/sda2").Return(nil)
	cmd.On("Run", "resize2fs", "/dev/sda2").Return(nil)
	mnt.On("Mount", "LABEL=K3OS_STATE", targetDir, "", "").Return(nil).Once()
	fs.On("Remove", targetDir+"/k3os/system/growpart").Return(nil)

	err := h.SetupMounts()
	require.NoError(t, err)

	fs.AssertExpectations(t)
	cmd.AssertExpectations(t)
	mnt.AssertExpectations(t)
}

func TestDiskHandler_SetupMounts_GrowpartBlkidFallback(t *testing.T) {
	t.Parallel()

	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}
	mnt := &MockMounter{}

	deps := &Deps{FS: fs, Cmd: cmd, Mounter: mnt}
	h := NewDiskHandler(deps)

	fs.On("MkdirAll", targetDir, os.FileMode(0o755)).Return(nil)
	mnt.On("Mount", "LABEL=K3OS_STATE", targetDir, "", "").Return(nil).Once()
	fs.On("ReadFile", targetDir+"/k3os/system/growpart").Return([]byte("/dev/vda 1"), nil)
	// Device path from growpart doesn't exist, need blkid fallback
	fs.On("Stat", "/dev/vda1").Return(nil, os.ErrNotExist)
	cmd.On("RunOutput", "blkid", "-L", "K3OS_STATE").Return("/dev/sda2", nil)
	// Now check the resolved device
	fs.On("Stat", "/dev/sda2").Return(fakeFileInfo{}, nil)
	cmd.On("Run", "umount", targetDir).Return(nil)
	cmd.On("Run", "parted", "/dev/sda", "resizepart", "2", "100%").Return(nil)
	cmd.On("Run", "partprobe", "/dev/sda").Return(nil)
	cmd.On("Run", "e2fsck", "-f", "/dev/sda2").Return(nil)
	cmd.On("Run", "resize2fs", "/dev/sda2").Return(nil)
	mnt.On("Mount", "LABEL=K3OS_STATE", targetDir, "", "").Return(nil).Once()
	fs.On("Remove", targetDir+"/k3os/system/growpart").Return(nil)

	err := h.SetupMounts()
	require.NoError(t, err)

	cmd.AssertExpectations(t)
}

func TestDiskHandler_SetupKernelSquashfs_Copies(t *testing.T) {
	t.Parallel()

	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}
	mnt := &MockMounter{}

	deps := &Deps{FS: fs, Cmd: cmd, Mounter: mnt, KernelVersion: "5.15.0"}
	h := NewDiskHandler(deps)

	src := "/.base/k3os/system/kernel/5.15.0/kernel.squashfs"
	dst := targetDir + "/k3os/system/kernel/5.15.0/kernel.squashfs"
	dstDir := targetDir + "/k3os/system/kernel/5.15.0"

	fs.On("Stat", src).Return(fakeFileInfo{}, nil)
	fs.On("Stat", dst).Return(nil, os.ErrNotExist)
	fs.On("MkdirAll", dstDir, os.FileMode(0o755)).Return(nil)
	cmd.On("Run", "cp", "-r", src, dst).Return(nil)

	err := h.SetupKernelSquashfs()
	require.NoError(t, err)

	fs.AssertExpectations(t)
	cmd.AssertExpectations(t)
}

func TestDiskHandler_SetupKernelSquashfs_AlreadyExists(t *testing.T) {
	t.Parallel()

	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}
	mnt := &MockMounter{}

	deps := &Deps{FS: fs, Cmd: cmd, Mounter: mnt, KernelVersion: "5.15.0"}
	h := NewDiskHandler(deps)

	src := "/.base/k3os/system/kernel/5.15.0/kernel.squashfs"
	dst := targetDir + "/k3os/system/kernel/5.15.0/kernel.squashfs"

	fs.On("Stat", src).Return(fakeFileInfo{}, nil)
	fs.On("Stat", dst).Return(fakeFileInfo{}, nil)

	err := h.SetupKernelSquashfs()
	require.NoError(t, err)

	cmd.AssertNotCalled(t, "Run", mock.Anything)
}

func TestDiskHandler_SetupK3OS_CopiesAndSymlinks(t *testing.T) {
	t.Parallel()

	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}
	mnt := &MockMounter{}

	deps := &Deps{FS: fs, Cmd: cmd, Mounter: mnt, VersionID: "v0.21.5"}
	h := NewDiskHandler(deps)

	// current k3os doesn't exist
	fs.On("Stat", targetDir+"/k3os/system/k3os/current/k3os").Return(nil, os.ErrNotExist)
	// source exists
	fs.On("Stat", "/.base/k3os/system/k3os/current/k3os").Return(fakeFileInfo{}, nil)
	// destination doesn't exist
	dstFile := targetDir + "/k3os/system/k3os/v0.21.5/k3os"
	fs.On("Stat", dstFile).Return(nil, os.ErrNotExist)
	fs.On("MkdirAll", targetDir+"/k3os/system/k3os/v0.21.5", os.FileMode(0o755)).Return(nil)
	cmd.On("Run", "cp", "-f", "/.base/k3os/system/k3os/current/k3os", dstFile+".tmp").Return(nil)
	fs.On("Rename", dstFile+".tmp", dstFile).Return(nil)
	fs.On("Symlink", "v0.21.5", targetDir+"/k3os/system/k3os/current").Return(nil)

	err := h.SetupK3OS()
	require.NoError(t, err)

	fs.AssertExpectations(t)
	cmd.AssertExpectations(t)
}

func TestDiskHandler_SetupK3OS_AlreadyExists(t *testing.T) {
	t.Parallel()

	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}
	mnt := &MockMounter{}

	deps := &Deps{FS: fs, Cmd: cmd, Mounter: mnt, VersionID: "v0.21.5"}
	h := NewDiskHandler(deps)

	fs.On("Stat", targetDir+"/k3os/system/k3os/current/k3os").Return(fakeFileInfo{}, nil)

	err := h.SetupK3OS()
	require.NoError(t, err)

	cmd.AssertNotCalled(t, "Run", mock.Anything)
}

func TestDiskHandler_SetupInit_CreatesSymlink(t *testing.T) {
	t.Parallel()

	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}
	mnt := &MockMounter{}

	deps := &Deps{FS: fs, Cmd: cmd, Mounter: mnt}
	h := NewDiskHandler(deps)

	initPath := targetDir + "/sbin/init"
	fs.On("Stat", initPath).Return(nil, os.ErrNotExist)
	fs.On("MkdirAll", targetDir+"/sbin", os.FileMode(0o755)).Return(nil)
	fs.On("Symlink", "../k3os/system/k3os/current/k3os", initPath).Return(nil)

	err := h.SetupInit()
	require.NoError(t, err)

	fs.AssertExpectations(t)
}

func TestDiskHandler_SetupInit_AlreadyExists(t *testing.T) {
	t.Parallel()

	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}
	mnt := &MockMounter{}

	deps := &Deps{FS: fs, Cmd: cmd, Mounter: mnt}
	h := NewDiskHandler(deps)

	initPath := targetDir + "/sbin/init"
	fs.On("Stat", initPath).Return(fakeFileInfo{}, nil)

	err := h.SetupInit()
	require.NoError(t, err)

	fs.AssertNotCalled(t, "Symlink", mock.Anything, mock.Anything)
}

func TestDiskHandler_SetupK3s_LinksLatestDir(t *testing.T) {
	t.Parallel()

	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}
	mnt := &MockMounter{}

	deps := &Deps{FS: fs, Cmd: cmd, Mounter: mnt}
	h := NewDiskHandler(deps)

	k3sDir := targetDir + "/k3os/system/k3s"
	k3sBin := k3sDir + "/current/k3s"

	fs.On("Stat", k3sBin).Return(nil, os.ErrNotExist)
	fs.On("ReadDir", k3sDir).Return([]iface.DirEntry{
		fakeDirEntry{name: "v1.25.0", isDir: true, mode: os.ModeDir},
		fakeDirEntry{name: "current", isDir: false, mode: os.ModeSymlink},
	}, nil)
	fs.On("Remove", k3sDir+"/current").Return(nil)
	fs.On("Symlink", "v1.25.0", k3sDir+"/current").Return(nil)

	err := h.SetupK3s()
	require.NoError(t, err)

	fs.AssertExpectations(t)
}

func TestDiskHandler_SetupK3s_AlreadyExists(t *testing.T) {
	t.Parallel()

	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}
	mnt := &MockMounter{}

	deps := &Deps{FS: fs, Cmd: cmd, Mounter: mnt}
	h := NewDiskHandler(deps)

	k3sBin := targetDir + "/k3os/system/k3s/current/k3s"
	fs.On("Stat", k3sBin).Return(fakeFileInfo{}, nil)

	err := h.SetupK3s()
	require.NoError(t, err)

	fs.AssertNotCalled(t, "ReadDir", mock.Anything)
}

func TestDiskHandler_SetupK3s_NoDirEntries(t *testing.T) {
	t.Parallel()

	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}
	mnt := &MockMounter{}

	deps := &Deps{FS: fs, Cmd: cmd, Mounter: mnt}
	h := NewDiskHandler(deps)

	k3sBin := targetDir + "/k3os/system/k3s/current/k3s"
	k3sDir := targetDir + "/k3os/system/k3s"

	fs.On("Stat", k3sBin).Return(nil, os.ErrNotExist)
	fs.On("ReadDir", k3sDir).Return(nil, os.ErrNotExist)

	err := h.SetupK3s()
	require.NoError(t, err)
}

func TestDiskHandler_Takeover_NoMarker(t *testing.T) {
	t.Parallel()

	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}
	mnt := &MockMounter{}

	deps := &Deps{FS: fs, Cmd: cmd, Mounter: mnt}
	h := NewDiskHandler(deps)

	fs.On("Stat", targetDir+"/k3os/system/takeover").Return(nil, os.ErrNotExist)

	err := h.Takeover()
	require.NoError(t, err)

	cmd.AssertNotCalled(t, "Run", "reboot", "-f")
}

func TestDiskHandler_Takeover_Reboot(t *testing.T) {
	t.Parallel()

	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}
	mnt := &MockMounter{}

	deps := &Deps{FS: fs, Cmd: cmd, Mounter: mnt}
	h := NewDiskHandler(deps)

	fs.On("Stat", targetDir+"/k3os/system/takeover").Return(fakeFileInfo{}, nil)
	fs.On("WriteFile", targetDir+"/k3os/system/factory-reset", []byte(nil), os.FileMode(0o644)).Return(nil)
	fs.On("Lstat", targetDir+"/sbin").Return(fakeFileInfo{mode: os.ModeDir}, nil)
	fs.On("ReadDir", targetDir).Return([]iface.DirEntry{
		fakeDirEntry{name: "boot", isDir: true},
		fakeDirEntry{name: "k3os", isDir: true},
		fakeDirEntry{name: "sbin", isDir: true},
		fakeDirEntry{name: "tmp", isDir: true},
	}, nil)
	fs.On("RemoveAll", targetDir+"/tmp").Return(nil)
	fs.On("ReadDir", targetDir+"/sbin").Return([]iface.DirEntry{
		fakeDirEntry{name: "init"},
		fakeDirEntry{name: "other"},
	}, nil)
	fs.On("RemoveAll", targetDir+"/sbin/other").Return(nil)
	fs.On("ReadDir", targetDir+"/boot").Return([]iface.DirEntry{
		fakeDirEntry{name: "grub"},
		fakeDirEntry{name: "vmlinuz"},
	}, nil)
	fs.On("RemoveAll", targetDir+"/boot/vmlinuz").Return(nil)
	fs.On("Remove", targetDir+"/k3os/system/takeover").Return(nil)
	fs.On("RemoveAll", targetDir+"/k3os/data").Return(nil)
	cmd.On("Run", "sync").Return(nil)
	// No poweroff marker
	fs.On("Stat", targetDir+"/k3os/system/poweroff").Return(nil, os.ErrNotExist)
	cmd.On("Run", "reboot", "-f").Return(nil)

	err := h.Takeover()
	require.NoError(t, err)

	cmd.AssertExpectations(t)
}

func TestDiskHandler_Takeover_Poweroff(t *testing.T) {
	t.Parallel()

	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}
	mnt := &MockMounter{}

	deps := &Deps{FS: fs, Cmd: cmd, Mounter: mnt}
	h := NewDiskHandler(deps)

	fs.On("Stat", targetDir+"/k3os/system/takeover").Return(fakeFileInfo{}, nil)
	fs.On("WriteFile", targetDir+"/k3os/system/factory-reset", []byte(nil), os.FileMode(0o644)).Return(nil)
	fs.On("Lstat", targetDir+"/sbin").Return(fakeFileInfo{mode: os.ModeDir}, nil)
	fs.On("ReadDir", targetDir).Return([]iface.DirEntry{
		fakeDirEntry{name: "boot", isDir: true},
		fakeDirEntry{name: "k3os", isDir: true},
		fakeDirEntry{name: "sbin", isDir: true},
	}, nil)
	fs.On("ReadDir", targetDir+"/sbin").Return([]iface.DirEntry{
		fakeDirEntry{name: "init"},
	}, nil)
	fs.On("ReadDir", targetDir+"/boot").Return([]iface.DirEntry{
		fakeDirEntry{name: "grub"},
	}, nil)
	fs.On("Remove", targetDir+"/k3os/system/takeover").Return(nil)
	fs.On("RemoveAll", targetDir+"/k3os/data").Return(nil)
	cmd.On("Run", "sync").Return(nil).Times(2)
	// Poweroff marker exists
	fs.On("Stat", targetDir+"/k3os/system/poweroff").Return(fakeFileInfo{}, nil)
	fs.On("Remove", targetDir+"/k3os/system/poweroff").Return(nil)
	cmd.On("Run", "poweroff", "-f").Return(nil)

	err := h.Takeover()
	require.NoError(t, err)

	cmd.AssertCalled(t, "Run", "poweroff", "-f")
}

func TestDiskHandler_CleanupEphemeral_FactoryReset(t *testing.T) {
	t.Parallel()

	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}
	mnt := &MockMounter{}

	deps := &Deps{FS: fs, Cmd: cmd, Mounter: mnt}
	h := NewDiskHandler(deps)

	fs.On("Stat", targetDir+"/k3os/system/factory-reset").Return(fakeFileInfo{}, nil)
	fs.On("Stat", targetDir+"/k3os/system/ephemeral").Return(nil, os.ErrNotExist)
	fs.On("RemoveAll", targetDir+"/k3os/data").Return(nil)
	fs.On("Remove", targetDir+"/k3os/system/factory-reset").Return(nil)

	err := h.CleanupEphemeral()
	require.NoError(t, err)

	fs.AssertExpectations(t)
}

func TestDiskHandler_CleanupEphemeral_NeitherMarker(t *testing.T) {
	t.Parallel()

	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}
	mnt := &MockMounter{}

	deps := &Deps{FS: fs, Cmd: cmd, Mounter: mnt}
	h := NewDiskHandler(deps)

	fs.On("Stat", targetDir+"/k3os/system/factory-reset").Return(nil, os.ErrNotExist)
	fs.On("Stat", targetDir+"/k3os/system/ephemeral").Return(nil, os.ErrNotExist)

	err := h.CleanupEphemeral()
	require.NoError(t, err)

	fs.AssertNotCalled(t, "RemoveAll", mock.Anything)
}

func TestDiskHandler_PivotAndExec(t *testing.T) {
	t.Parallel()

	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}
	mnt := &MockMounter{}
	proc := &MockProcessExecutor{}

	deps := &Deps{FS: fs, Cmd: cmd, Mounter: mnt, Proc: proc}
	h := NewDiskHandler(deps)

	cmd.On("Run", "losetup", "-d", "/dev/loop0").Return(nil)
	mnt.On("ForceMount", "", "/", "none", "rprivate").Return(nil)
	fs.On("MkdirAll", targetDir+"/.root", os.FileMode(0o755)).Return(nil)
	proc.On("PivotRoot", targetDir, targetDir+"/.root").Return(nil)
	proc.On("Exec", "/sbin/init", []string{"/sbin/init"}, mock.AnythingOfType("[]string")).Return(nil)

	err := h.PivotAndExec()
	require.Error(t, err)

	var execErr *ErrExecCalled
	require.ErrorAs(t, err, &execErr)
	assert.Equal(t, "/sbin/init", execErr.Path)

	proc.AssertExpectations(t)
}

func TestDiskHandler_PivotAndExec_PivotFails(t *testing.T) {
	t.Parallel()

	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}
	mnt := &MockMounter{}
	proc := &MockProcessExecutor{}

	deps := &Deps{FS: fs, Cmd: cmd, Mounter: mnt, Proc: proc}
	h := NewDiskHandler(deps)

	cmd.On("Run", "losetup", "-d", "/dev/loop0").Return(nil)
	mnt.On("ForceMount", "", "/", "none", "rprivate").Return(nil)
	fs.On("MkdirAll", targetDir+"/.root", os.FileMode(0o755)).Return(nil)
	proc.On("PivotRoot", targetDir, targetDir+"/.root").Return(errors.New("pivot failed"))

	err := h.PivotAndExec()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pivot failed")
}

func TestStripPartitionNumber(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected string
	}{
		{"/dev/sda2", "/dev/sda"},
		{"/dev/nvme0n1p2", "/dev/nvme0n1"},
		{"/dev/vda1", "/dev/vda"},
	}
	for _, tc := range tests {
		assert.Equal(t, tc.expected, stripPartitionNumber(tc.input))
	}
}

func TestExtractPartitionNumber(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected string
	}{
		{"/dev/sda2", "2"},
		{"/dev/nvme0n1p2", "2"},
		{"/dev/vda1", "1"},
	}
	for _, tc := range tests {
		assert.Equal(t, tc.expected, extractPartitionNumber(tc.input))
	}
}

func TestDiskHandler_Execute_TakeoverSkipped(t *testing.T) {
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
		VersionID:     "v0.21.5",
		SleepFunc:     func(time.Duration) {},
	}
	h := NewDiskHandler(deps)

	// SetupMounts
	fs.On("MkdirAll", targetDir, os.FileMode(0o755)).Return(nil)
	mnt.On("Mount", "LABEL=K3OS_STATE", targetDir, "", "").Return(nil)
	fs.On("ReadFile", targetDir+"/k3os/system/growpart").Return(nil, os.ErrNotExist)

	// SetupK3OS - already exists
	fs.On("Stat", targetDir+"/k3os/system/k3os/current/k3os").Return(fakeFileInfo{}, nil)

	// SetupKernelSquashfs - source not present
	fs.On("Stat", "/.base/k3os/system/kernel/5.15.0/kernel.squashfs").Return(nil, os.ErrNotExist)

	// SetupInit - already exists
	fs.On("Stat", targetDir+"/sbin/init").Return(fakeFileInfo{}, nil)

	// SetupK3s - already has current
	fs.On("Stat", targetDir+"/k3os/system/k3s/current/k3s").Return(fakeFileInfo{}, nil)

	// Takeover - no marker
	fs.On("Stat", targetDir+"/k3os/system/takeover").Return(nil, os.ErrNotExist)

	// CleanupEphemeral - no markers
	fs.On("Stat", targetDir+"/k3os/system/factory-reset").Return(nil, os.ErrNotExist)
	fs.On("Stat", targetDir+"/k3os/system/ephemeral").Return(nil, os.ErrNotExist)

	// PivotAndExec
	cmd.On("Run", "losetup", "-d", "/dev/loop0").Return(nil)
	mnt.On("ForceMount", "", "/", "none", "rprivate").Return(nil)
	fs.On("MkdirAll", targetDir+"/.root", os.FileMode(0o755)).Return(nil)
	proc.On("PivotRoot", targetDir, targetDir+"/.root").Return(nil)
	proc.On("Exec", "/sbin/init", []string{"/sbin/init"}, mock.AnythingOfType("[]string")).Return(nil)

	err := h.Execute()
	require.Error(t, err)

	var execErr *ErrExecCalled
	require.ErrorAs(t, err, &execErr)
}
