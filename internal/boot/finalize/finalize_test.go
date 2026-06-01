//go:build linux

package finalize

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// SetupSudoers
// ---------------------------------------------------------------------------

func TestSetupSudoers_Success(t *testing.T) {
	t.Parallel()
	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}
	mnt := &MockMounter{}
	f := &Finalizer{FS: fs, Cmd: cmd, Mounter: mnt, Mode: "local"}

	fs.On("MkdirAll", "/etc/sudoers.d", os.FileMode(0o755)).Return(nil)
	fs.On("WriteFile", "/etc/sudoers.d/sudo", mock.AnythingOfType("[]uint8"), os.FileMode(0o440)).Return(nil)

	err := f.SetupSudoers()
	require.NoError(t, err)
	fs.AssertExpectations(t)
}

func TestSetupSudoers_MkdirFails(t *testing.T) {
	t.Parallel()
	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}
	mnt := &MockMounter{}
	f := &Finalizer{FS: fs, Cmd: cmd, Mounter: mnt}

	fs.On("MkdirAll", "/etc/sudoers.d", os.FileMode(0o755)).Return(errors.New("permission denied"))

	err := f.SetupSudoers()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "permission denied")
}

func TestSetupSudoers_WriteFails(t *testing.T) {
	t.Parallel()
	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}
	mnt := &MockMounter{}
	f := &Finalizer{FS: fs, Cmd: cmd, Mounter: mnt}

	fs.On("MkdirAll", "/etc/sudoers.d", os.FileMode(0o755)).Return(nil)
	fs.On("WriteFile", "/etc/sudoers.d/sudo", mock.AnythingOfType("[]uint8"), os.FileMode(0o440)).Return(errors.New("write failed"))

	err := f.SetupSudoers()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "write failed")
}

// ---------------------------------------------------------------------------
// SetupTTYs
// ---------------------------------------------------------------------------

func TestSetupTTYs_StandardTTYs(t *testing.T) {
	t.Parallel()
	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}
	mnt := &MockMounter{}
	f := &Finalizer{FS: fs, Cmd: cmd, Mounter: mnt, CmdlineReader: func() (string, error) {
		return "root=/dev/sda1", nil
	}}

	fi := fakeFileInfo{name: "tty1", isDir: false}
	for i := 1; i <= 6; i++ {
		fs.On("Stat", fmt.Sprintf("/dev/tty%d", i)).Return(fi, nil)
	}

	fs.On("ReadFile", "/etc/inittab").Return([]byte(""), nil)
	fs.On("WriteFile", "/etc/inittab", mock.AnythingOfType("[]uint8"), os.FileMode(0o644)).Return(nil)
	fs.On("ReadFile", "/etc/securetty").Return([]byte(""), nil)
	fs.On("WriteFile", "/etc/securetty", mock.AnythingOfType("[]uint8"), os.FileMode(0o644)).Return(nil)

	err := f.SetupTTYs()
	require.NoError(t, err)
	fs.AssertExpectations(t)
}

func TestSetupTTYs_SerialConsole(t *testing.T) {
	t.Parallel()
	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}
	mnt := &MockMounter{}
	f := &Finalizer{FS: fs, Cmd: cmd, Mounter: mnt, CmdlineReader: func() (string, error) {
		return "console=ttyS0,115200n8", nil
	}}

	// No standard TTYs.
	for i := 1; i <= 6; i++ {
		fs.On("Stat", fmt.Sprintf("/dev/tty%d", i)).Return(nil, os.ErrNotExist)
	}
	// Serial console device exists.
	fi := fakeFileInfo{name: "ttyS0", isDir: false}
	fs.On("Stat", "/dev/ttyS0").Return(fi, nil)

	fs.On("ReadFile", "/etc/inittab").Return([]byte(""), nil)
	fs.On("WriteFile", "/etc/inittab", mock.AnythingOfType("[]uint8"), os.FileMode(0o644)).Return(nil)
	fs.On("ReadFile", "/etc/securetty").Return([]byte(""), nil)
	fs.On("WriteFile", "/etc/securetty", mock.AnythingOfType("[]uint8"), os.FileMode(0o644)).Return(nil)

	err := f.SetupTTYs()
	require.NoError(t, err)
	fs.AssertExpectations(t)
}

func TestSetupTTYs_SerialConsoleDeduplicated(t *testing.T) {
	t.Parallel()
	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}
	mnt := &MockMounter{}
	// tty1 appears both as a standard TTY and in console= cmdline parameter.
	f := &Finalizer{FS: fs, Cmd: cmd, Mounter: mnt, CmdlineReader: func() (string, error) {
		return "console=tty1,115200n8", nil
	}}

	fi := fakeFileInfo{name: "tty1", isDir: false}
	// tty1 exists as standard TTY.
	fs.On("Stat", "/dev/tty1").Return(fi, nil)
	for i := 2; i <= 6; i++ {
		fs.On("Stat", fmt.Sprintf("/dev/tty%d", i)).Return(nil, os.ErrNotExist)
	}

	fs.On("ReadFile", "/etc/inittab").Return([]byte(""), nil)
	fs.On("WriteFile", "/etc/inittab", mock.MatchedBy(func(data []byte) bool {
		content := string(data)
		// tty1 should appear exactly once.
		count := 0
		for _, line := range strings.Split(content, "\n") {
			if strings.HasPrefix(line, "tty1::") {
				count++
			}
		}
		return count == 1
	}), os.FileMode(0o644)).Return(nil)
	fs.On("ReadFile", "/etc/securetty").Return([]byte(""), nil)
	fs.On("WriteFile", "/etc/securetty", mock.MatchedBy(func(data []byte) bool {
		content := string(data)
		count := 0
		for _, line := range strings.Split(content, "\n") {
			if line == "tty1" {
				count++
			}
		}
		return count == 1
	}), os.FileMode(0o644)).Return(nil)

	err := f.SetupTTYs()
	require.NoError(t, err)
	fs.AssertExpectations(t)
}

func TestSetupTTYs_NoDevices(t *testing.T) {
	t.Parallel()
	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}
	mnt := &MockMounter{}
	f := &Finalizer{FS: fs, Cmd: cmd, Mounter: mnt, CmdlineReader: func() (string, error) {
		return "root=/dev/sda1", nil
	}}

	// No devices exist.
	for i := 1; i <= 6; i++ {
		fs.On("Stat", fmt.Sprintf("/dev/tty%d", i)).Return(nil, os.ErrNotExist)
	}

	err := f.SetupTTYs()
	require.NoError(t, err)
}

func TestSetupTTYs_CmdlineReadError(t *testing.T) {
	t.Parallel()
	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}
	mnt := &MockMounter{}
	f := &Finalizer{FS: fs, Cmd: cmd, Mounter: mnt, CmdlineReader: func() (string, error) {
		return "", errors.New("read failed")
	}}

	fi := fakeFileInfo{name: "tty1", isDir: false}
	for i := 1; i <= 6; i++ {
		fs.On("Stat", fmt.Sprintf("/dev/tty%d", i)).Return(fi, nil)
	}

	err := f.SetupTTYs()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read cmdline")
}

// ---------------------------------------------------------------------------
// SetupServices
// ---------------------------------------------------------------------------

func TestSetupServices_Success(t *testing.T) {
	t.Parallel()
	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}
	mnt := &MockMounter{}
	f := &Finalizer{FS: fs, Cmd: cmd, Mounter: mnt, VirtDetector: func() ([]string, error) {
		return nil, nil
	}}

	fs.On("Symlink", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(nil)

	err := f.SetupServices()
	require.NoError(t, err)
	fs.AssertExpectations(t)
}

func TestSetupServices_KVM(t *testing.T) {
	t.Parallel()
	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}
	mnt := &MockMounter{}
	f := &Finalizer{FS: fs, Cmd: cmd, Mounter: mnt, VirtDetector: func() ([]string, error) {
		return []string{"kvm"}, nil
	}}

	fs.On("Symlink", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(nil)

	err := f.SetupServices()
	require.NoError(t, err)

	// Verify qemu-guest-agent symlink was created.
	fs.AssertCalled(t, "Symlink", "/etc/init.d/qemu-guest-agent", "/etc/runlevels/boot/qemu-guest-agent")
}

func TestSetupServices_HyperV(t *testing.T) {
	t.Parallel()
	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}
	mnt := &MockMounter{}
	f := &Finalizer{FS: fs, Cmd: cmd, Mounter: mnt, VirtDetector: func() ([]string, error) {
		return []string{"hyperv"}, nil
	}}

	fs.On("Symlink", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(nil)

	err := f.SetupServices()
	require.NoError(t, err)

	fs.AssertCalled(t, "Symlink", "/etc/init.d/hv_kvp_daemon", "/etc/runlevels/boot/hv_kvp_daemon")
	fs.AssertCalled(t, "Symlink", "/etc/init.d/hv_fcopy_daemon", "/etc/runlevels/boot/hv_fcopy_daemon")
	fs.AssertCalled(t, "Symlink", "/etc/init.d/hv_vss_daemon", "/etc/runlevels/boot/hv_vss_daemon")
}

func TestSetupServices_VMware(t *testing.T) {
	t.Parallel()
	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}
	mnt := &MockMounter{}
	f := &Finalizer{FS: fs, Cmd: cmd, Mounter: mnt, VirtDetector: func() ([]string, error) {
		return []string{"vmware"}, nil
	}}

	fs.On("Symlink", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(nil)

	err := f.SetupServices()
	require.NoError(t, err)

	fs.AssertCalled(t, "Symlink", "/etc/init.d/open-vm-tools", "/etc/runlevels/boot/open-vm-tools")
}

func TestSetupServices_VirtDetectorFails(t *testing.T) {
	t.Parallel()
	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}
	mnt := &MockMounter{}
	f := &Finalizer{FS: fs, Cmd: cmd, Mounter: mnt, VirtDetector: func() ([]string, error) {
		return nil, errors.New("virt-what not found")
	}}

	fs.On("Symlink", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(nil)

	// virt-what failure is non-fatal.
	err := f.SetupServices()
	require.NoError(t, err)
	fs.AssertExpectations(t)
}

func TestSetupServices_SymlinkFails(t *testing.T) {
	t.Parallel()
	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}
	mnt := &MockMounter{}
	f := &Finalizer{FS: fs, Cmd: cmd, Mounter: mnt, VirtDetector: func() ([]string, error) {
		return nil, nil
	}}

	fs.On("Symlink", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(errors.New("symlink failed")).Once()

	err := f.SetupServices()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "symlink failed")
}

// ---------------------------------------------------------------------------
// SetupHostname
// ---------------------------------------------------------------------------

func TestSetupHostname_AlreadyExists(t *testing.T) {
	t.Parallel()
	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}
	mnt := &MockMounter{}
	f := &Finalizer{FS: fs, Cmd: cmd, Mounter: mnt}

	fi := fakeFileInfo{name: "hostname", isDir: false}
	fs.On("Stat", "/etc/hostname").Return(fi, nil)

	err := f.SetupHostname()
	require.NoError(t, err)
	fs.AssertExpectations(t)
}

func TestSetupHostname_PersistedHostname(t *testing.T) {
	t.Parallel()
	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}
	mnt := &MockMounter{}
	f := &Finalizer{FS: fs, Cmd: cmd, Mounter: mnt}

	fs.On("Stat", "/etc/hostname").Return(nil, os.ErrNotExist)
	fs.On("ReadFile", "/var/lib/rancher/k3os/hostname").Return([]byte("myhost\n"), nil)
	fs.On("WriteFile", "/etc/hostname", []byte("myhost\n"), os.FileMode(0o644)).Return(nil)

	err := f.SetupHostname()
	require.NoError(t, err)
	fs.AssertExpectations(t)
}

func TestSetupHostname_RandomGeneration(t *testing.T) {
	t.Parallel()
	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}
	mnt := &MockMounter{}
	f := &Finalizer{FS: fs, Cmd: cmd, Mounter: mnt, RandFunc: func() (uint32, error) {
		return 12345, nil
	}}

	fs.On("Stat", "/etc/hostname").Return(nil, os.ErrNotExist)
	fs.On("ReadFile", "/var/lib/rancher/k3os/hostname").Return(nil, os.ErrNotExist)
	fs.On("MkdirAll", "/var/lib/rancher/k3os", os.FileMode(0o755)).Return(nil)
	fs.On("WriteFile", "/var/lib/rancher/k3os/hostname", []byte("k3os-12345\n"), os.FileMode(0o644)).Return(nil)
	fs.On("WriteFile", "/etc/hostname", []byte("k3os-12345\n"), os.FileMode(0o644)).Return(nil)

	err := f.SetupHostname()
	require.NoError(t, err)
	fs.AssertExpectations(t)
}

func TestSetupHostname_RandFuncFails(t *testing.T) {
	t.Parallel()
	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}
	mnt := &MockMounter{}
	f := &Finalizer{FS: fs, Cmd: cmd, Mounter: mnt, RandFunc: func() (uint32, error) {
		return 0, errors.New("entropy exhausted")
	}}

	fs.On("Stat", "/etc/hostname").Return(nil, os.ErrNotExist)
	fs.On("ReadFile", "/var/lib/rancher/k3os/hostname").Return(nil, os.ErrNotExist)

	err := f.SetupHostname()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "entropy exhausted")
}

// ---------------------------------------------------------------------------
// SetupHosts
// ---------------------------------------------------------------------------

func TestSetupHosts_AlreadyExists(t *testing.T) {
	t.Parallel()
	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}
	mnt := &MockMounter{}
	f := &Finalizer{FS: fs, Cmd: cmd, Mounter: mnt}

	fi := fakeFileInfo{name: "hosts", isDir: false}
	fs.On("Stat", "/etc/hosts").Return(fi, nil)

	err := f.SetupHosts()
	require.NoError(t, err)
	fs.AssertExpectations(t)
}

func TestSetupHosts_Generated(t *testing.T) {
	t.Parallel()
	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}
	mnt := &MockMounter{}
	f := &Finalizer{FS: fs, Cmd: cmd, Mounter: mnt}

	fs.On("Stat", "/etc/hosts").Return(nil, os.ErrNotExist)
	fs.On("ReadFile", "/etc/hostname").Return([]byte("myhost\n"), nil)
	fs.On("WriteFile", "/etc/hosts", mock.AnythingOfType("[]uint8"), os.FileMode(0o644)).Return(nil)

	err := f.SetupHosts()
	require.NoError(t, err)
	fs.AssertExpectations(t)
}

func TestSetupHosts_ReadHostnameFails(t *testing.T) {
	t.Parallel()
	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}
	mnt := &MockMounter{}
	f := &Finalizer{FS: fs, Cmd: cmd, Mounter: mnt}

	fs.On("Stat", "/etc/hosts").Return(nil, os.ErrNotExist)
	fs.On("ReadFile", "/etc/hostname").Return(nil, errors.New("no hostname"))

	err := f.SetupHosts()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read /etc/hostname")
}

// ---------------------------------------------------------------------------
// SetupRoot
// ---------------------------------------------------------------------------

func TestSetupRoot_AlreadyExists(t *testing.T) {
	t.Parallel()
	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}
	mnt := &MockMounter{}
	f := &Finalizer{FS: fs, Cmd: cmd, Mounter: mnt}

	fi := fakeFileInfo{name: "root", isDir: true}
	fs.On("Stat", "/root").Return(fi, nil)

	err := f.SetupRoot()
	require.NoError(t, err)
	fs.AssertExpectations(t)
}

func TestSetupRoot_CreatesDir(t *testing.T) {
	t.Parallel()
	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}
	mnt := &MockMounter{}
	f := &Finalizer{FS: fs, Cmd: cmd, Mounter: mnt}

	fs.On("Stat", "/root").Return(nil, os.ErrNotExist)
	fs.On("MkdirAll", "/root", os.FileMode(0o700)).Return(nil)
	fs.On("Chmod", "/root", os.FileMode(0o700)).Return(nil)

	err := f.SetupRoot()
	require.NoError(t, err)
	fs.AssertExpectations(t)
}

func TestSetupRoot_MkdirFails(t *testing.T) {
	t.Parallel()
	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}
	mnt := &MockMounter{}
	f := &Finalizer{FS: fs, Cmd: cmd, Mounter: mnt}

	fs.On("Stat", "/root").Return(nil, os.ErrNotExist)
	fs.On("MkdirAll", "/root", os.FileMode(0o700)).Return(errors.New("mkdir failed"))

	err := f.SetupRoot()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mkdir failed")
}

// ---------------------------------------------------------------------------
// SetupMounts
// ---------------------------------------------------------------------------

func TestSetupMounts_BindMounts(t *testing.T) {
	t.Parallel()
	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}
	mnt := &MockMounter{}
	f := &Finalizer{FS: fs, Cmd: cmd, Mounter: mnt}

	fi := fakeFileInfo{name: "boot", isDir: true}
	fs.On("Stat", "/.base/boot").Return(fi, nil)
	fs.On("Stat", "/.base/k3os/system").Return(fi, nil)
	fs.On("MkdirAll", "/boot", os.FileMode(0o755)).Return(nil)
	fs.On("MkdirAll", "/k3os/system", os.FileMode(0o755)).Return(nil)
	mnt.On("Mount", "/.base/boot", "/boot", "", "bind").Return(nil)
	mnt.On("Mount", "/.base/k3os/system", "/k3os/system", "", "ro,bind").Return(nil)
	mnt.On("Mounted", "/.base").Return(false, nil)

	err := f.SetupMounts()
	require.NoError(t, err)
	fs.AssertExpectations(t)
	mnt.AssertExpectations(t)
}

func TestSetupMounts_UnmountLoop(t *testing.T) {
	t.Parallel()
	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}
	mnt := &MockMounter{}
	f := &Finalizer{FS: fs, Cmd: cmd, Mounter: mnt}

	// No boot or system dirs.
	fs.On("Stat", "/.base/boot").Return(nil, os.ErrNotExist)
	fs.On("Stat", "/.base/k3os/system").Return(nil, os.ErrNotExist)

	// Mounted twice, then unmounted.
	mnt.On("Mounted", "/.base").Return(true, nil).Once()
	mnt.On("Mounted", "/.base").Return(true, nil).Once()
	mnt.On("Mounted", "/.base").Return(false, nil).Once()
	cmd.On("Run", "umount", "-l", "/.base").Return(nil).Twice()

	err := f.SetupMounts()
	require.NoError(t, err)
	cmd.AssertExpectations(t)
	mnt.AssertExpectations(t)
}

func TestSetupMounts_MountBootFails(t *testing.T) {
	t.Parallel()
	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}
	mnt := &MockMounter{}
	f := &Finalizer{FS: fs, Cmd: cmd, Mounter: mnt}

	fi := fakeFileInfo{name: "boot", isDir: true}
	fs.On("Stat", "/.base/boot").Return(fi, nil)
	fs.On("MkdirAll", "/boot", os.FileMode(0o755)).Return(nil)
	mnt.On("Mount", "/.base/boot", "/boot", "", "bind").Return(errors.New("mount failed"))

	err := f.SetupMounts()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mount failed")
}

// ---------------------------------------------------------------------------
// SetupManifests
// ---------------------------------------------------------------------------

func TestSetupManifests_Success(t *testing.T) {
	t.Parallel()
	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}
	mnt := &MockMounter{}

	copierCalled := false
	f := &Finalizer{FS: fs, Cmd: cmd, Mounter: mnt, ManifestCopier: func(src, dst string) error {
		copierCalled = true
		assert.Equal(t, "/usr/share/rancher/k3s/server/manifests", src)
		assert.Equal(t, "/var/lib/rancher/k3s/server/manifests", dst)
		return nil
	}}

	fs.On("MkdirAll", "/var/lib/rancher/k3s/server/manifests", os.FileMode(0o755)).Return(nil)

	err := f.SetupManifests()
	require.NoError(t, err)
	assert.True(t, copierCalled)
	fs.AssertExpectations(t)
}

func TestSetupManifests_MkdirFails(t *testing.T) {
	t.Parallel()
	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}
	mnt := &MockMounter{}
	f := &Finalizer{FS: fs, Cmd: cmd, Mounter: mnt, ManifestCopier: func(_, _ string) error { return nil }}

	fs.On("MkdirAll", "/var/lib/rancher/k3s/server/manifests", os.FileMode(0o755)).Return(errors.New("mkdir failed"))

	err := f.SetupManifests()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mkdir failed")
}

func TestSetupManifests_CopyFails(t *testing.T) {
	t.Parallel()
	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}
	mnt := &MockMounter{}
	f := &Finalizer{FS: fs, Cmd: cmd, Mounter: mnt, ManifestCopier: func(_, _ string) error {
		return errors.New("copy failed")
	}}

	fs.On("MkdirAll", "/var/lib/rancher/k3s/server/manifests", os.FileMode(0o755)).Return(nil)

	err := f.SetupManifests()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "copy failed")
}

// ---------------------------------------------------------------------------
// SetupStateDirs
// ---------------------------------------------------------------------------

func TestSetupStateDirs_Success(t *testing.T) {
	t.Parallel()
	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}
	mnt := &MockMounter{}
	f := &Finalizer{FS: fs, Cmd: cmd, Mounter: mnt}

	fs.On("MkdirAll", "/var/lib/nfs", os.FileMode(0o755)).Return(nil)
	fs.On("MkdirAll", "/var/lib/rancher/k3s/agent/libexec/kubernetes", os.FileMode(0o755)).Return(nil)

	err := f.SetupStateDirs()
	require.NoError(t, err)
	fs.AssertExpectations(t)
}

func TestSetupStateDirs_MkdirFails(t *testing.T) {
	t.Parallel()
	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}
	mnt := &MockMounter{}
	f := &Finalizer{FS: fs, Cmd: cmd, Mounter: mnt}

	fs.On("MkdirAll", "/var/lib/nfs", os.FileMode(0o755)).Return(errors.New("no space"))

	err := f.SetupStateDirs()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no space")
}

// ---------------------------------------------------------------------------
// GrowLive
// ---------------------------------------------------------------------------

func TestGrowLive_NonLocalMode(t *testing.T) {
	t.Parallel()
	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}
	mnt := &MockMounter{}
	f := &Finalizer{FS: fs, Cmd: cmd, Mounter: mnt, Mode: "live"}

	err := f.GrowLive()
	require.NoError(t, err)
}

func TestGrowLive_NoGrowpartFile(t *testing.T) {
	t.Parallel()
	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}
	mnt := &MockMounter{}
	f := &Finalizer{FS: fs, Cmd: cmd, Mounter: mnt, Mode: "local"}

	fs.On("ReadFile", "/k3os/system/growpart").Return(nil, os.ErrNotExist)

	err := f.GrowLive()
	require.NoError(t, err)
}

func TestGrowLive_Success(t *testing.T) {
	t.Parallel()
	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}
	mnt := &MockMounter{}
	f := &Finalizer{FS: fs, Cmd: cmd, Mounter: mnt, Mode: "local", SleepFunc: func(time.Duration) {}}

	fs.On("ReadFile", "/k3os/system/growpart").Return([]byte("/dev/sda 2\n"), nil)
	fi := fakeFileInfo{name: "sda2", isDir: false}
	fs.On("Stat", "/dev/sda2").Return(fi, nil)
	cmd.On("Run", "parted", "/dev/sda", "resizepart", "2", "yes", "100%").Return(nil)
	cmd.On("Run", "partprobe", "/dev/sda").Return(nil)
	cmd.On("Run", "resize2fs", "/dev/sda2").Return(nil)
	fs.On("Remove", "/k3os/system/growpart").Return(nil)

	err := f.GrowLive()
	require.NoError(t, err)
	fs.AssertExpectations(t)
	cmd.AssertExpectations(t)
}

func TestGrowLive_BlkidFallback(t *testing.T) {
	t.Parallel()
	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}
	mnt := &MockMounter{}
	bp := &MockBlockProber{}
	f := &Finalizer{FS: fs, Cmd: cmd, Mounter: mnt, BlockProber: bp, Mode: "local", SleepFunc: func(time.Duration) {}}

	fs.On("ReadFile", "/k3os/system/growpart").Return([]byte("/dev/sda 2\n"), nil)
	fs.On("Stat", "/dev/sda2").Return(nil, os.ErrNotExist)
	bp.On("FindByLabel", "K3OS_STATE").Return("/dev/nvme0n1p2", nil)
	cmd.On("Run", "parted", "/dev/nvme0n1", "resizepart", "2", "yes", "100%").Return(nil)
	cmd.On("Run", "partprobe", "/dev/nvme0n1").Return(nil)
	cmd.On("Run", "resize2fs", "/dev/nvme0n1p2").Return(nil)
	fs.On("Remove", "/k3os/system/growpart").Return(nil)

	err := f.GrowLive()
	require.NoError(t, err)
	fs.AssertExpectations(t)
	cmd.AssertExpectations(t)
	bp.AssertExpectations(t)
}

func TestGrowLive_BlkidFails(t *testing.T) {
	t.Parallel()
	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}
	mnt := &MockMounter{}
	bp := &MockBlockProber{}
	f := &Finalizer{FS: fs, Cmd: cmd, Mounter: mnt, BlockProber: bp, Mode: "local"}

	fs.On("ReadFile", "/k3os/system/growpart").Return([]byte("/dev/sda 2\n"), nil)
	fs.On("Stat", "/dev/sda2").Return(nil, os.ErrNotExist)
	bp.On("FindByLabel", "K3OS_STATE").Return("", errors.New("not found"))

	err := f.GrowLive()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "find K3OS_STATE by label")
}

// ---------------------------------------------------------------------------
// Cleanup
// ---------------------------------------------------------------------------

func TestCleanup_EmptyMode(t *testing.T) {
	t.Parallel()
	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}
	mnt := &MockMounter{}
	f := &Finalizer{FS: fs, Cmd: cmd, Mounter: mnt, Mode: ""}

	fs.On("RemoveAll", "/run/k3os").Return(nil)

	err := f.Cleanup()
	require.NoError(t, err)
	fs.AssertExpectations(t)
	fs.AssertNotCalled(t, "MkdirAll", "/run/k3os", mock.Anything)
}

func TestCleanup_WithMode(t *testing.T) {
	t.Parallel()
	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}
	mnt := &MockMounter{}
	f := &Finalizer{FS: fs, Cmd: cmd, Mounter: mnt, Mode: "local"}

	fs.On("RemoveAll", "/run/k3os").Return(nil)
	fs.On("MkdirAll", "/run/k3os", os.FileMode(0o755)).Return(nil)
	fs.On("WriteFile", "/run/k3os/mode", []byte("local\n"), os.FileMode(0o644)).Return(nil)

	err := f.Cleanup()
	require.NoError(t, err)
	fs.AssertExpectations(t)
}

func TestCleanup_RemoveAllFails(t *testing.T) {
	t.Parallel()
	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}
	mnt := &MockMounter{}
	f := &Finalizer{FS: fs, Cmd: cmd, Mounter: mnt, Mode: "local"}

	fs.On("RemoveAll", "/run/k3os").Return(errors.New("remove failed"))

	err := f.Cleanup()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "remove failed")
}

// ---------------------------------------------------------------------------
// SetupConfig
// ---------------------------------------------------------------------------

func TestSetupConfig_Success(t *testing.T) {
	t.Parallel()
	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}
	mnt := &MockMounter{}
	f := &Finalizer{FS: fs, Cmd: cmd, Mounter: mnt, ConfigRunner: func() error { return nil }}

	fs.On("Stat", "/etc/conf.d/udev-settle").Return(nil, os.ErrNotExist)
	fs.On("Stat", "/var/lib/connman/cloud-config.config").Return(nil, os.ErrNotExist)
	fs.On("Stat", "/etc/conf.d/cloud-config").Return(nil, os.ErrNotExist)
	fs.On("Stat", "/etc/conf.d/rngd").Return(nil, os.ErrNotExist)

	err := f.SetupConfig()
	require.NoError(t, err)
	fs.AssertExpectations(t)
}

func TestSetupConfig_AllConditionalsPresent(t *testing.T) {
	t.Parallel()
	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}
	mnt := &MockMounter{}
	f := &Finalizer{FS: fs, Cmd: cmd, Mounter: mnt, ConfigRunner: func() error { return nil }}

	fi := fakeFileInfo{name: "conf", isDir: false}
	fs.On("Stat", "/etc/conf.d/udev-settle").Return(fi, nil)
	fs.On("Symlink", "/etc/init.d/udev-settle", "/etc/runlevels/sysinit/udev-settle").Return(nil)
	fs.On("Stat", "/var/lib/connman/cloud-config.config").Return(fi, nil)
	fs.On("ReadFile", "/etc/conf.d/connman").Return([]byte(""), nil)
	fs.On("WriteFile", "/etc/conf.d/connman", mock.AnythingOfType("[]uint8"), os.FileMode(0o644)).Return(nil)
	fs.On("Stat", "/etc/conf.d/cloud-config").Return(fi, nil)
	fs.On("Symlink", "/etc/init.d/cloud-config", "/etc/runlevels/boot/cloud-config").Return(nil)
	fs.On("Stat", "/etc/conf.d/rngd").Return(fi, nil)
	fs.On("Symlink", "/etc/init.d/rngd", "/etc/runlevels/boot/rngd").Return(nil)

	err := f.SetupConfig()
	require.NoError(t, err)
	fs.AssertExpectations(t)
}

func TestSetupConfig_K3osConfigFails(t *testing.T) {
	t.Parallel()
	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}
	mnt := &MockMounter{}
	f := &Finalizer{FS: fs, Cmd: cmd, Mounter: mnt, ConfigRunner: func() error {
		return errors.New("config failed")
	}}

	err := f.SetupConfig()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "config failed")
}

// ---------------------------------------------------------------------------
// Finalizer.Run
// ---------------------------------------------------------------------------

func TestFinalizer_Run_Success(t *testing.T) {
	t.Parallel()
	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}
	mnt := &MockMounter{}
	f := &Finalizer{
		FS:      fs,
		Cmd:     cmd,
		Mounter: mnt,
		Mode:    "local",
		CmdlineReader: func() (string, error) {
			return "", nil
		},
		RandFunc: func() (uint32, error) {
			return 99999, nil
		},
		VirtDetector: func() ([]string, error) {
			return nil, nil
		},
		SleepFunc:      func(time.Duration) {},
		ConfigRunner:   func() error { return nil },
		ManifestCopier: func(_, _ string) error { return nil },
	}

	// SetupMounts - no base dirs, not mounted.
	fs.On("Stat", "/.base/boot").Return(nil, os.ErrNotExist)
	fs.On("Stat", "/.base/k3os/system").Return(nil, os.ErrNotExist)
	mnt.On("Mounted", "/.base").Return(false, nil)

	// GrowLive - no growpart.
	fs.On("ReadFile", "/k3os/system/growpart").Return(nil, os.ErrNotExist)

	// SetupHostname - generate random.
	fs.On("Stat", "/etc/hostname").Return(nil, os.ErrNotExist)
	fs.On("ReadFile", "/var/lib/rancher/k3os/hostname").Return(nil, os.ErrNotExist)
	fs.On("MkdirAll", "/var/lib/rancher/k3os", os.FileMode(0o755)).Return(nil)
	fs.On("WriteFile", "/var/lib/rancher/k3os/hostname", []byte("k3os-99999\n"), os.FileMode(0o644)).Return(nil)
	fs.On("WriteFile", "/etc/hostname", []byte("k3os-99999\n"), os.FileMode(0o644)).Return(nil)

	// SetupHosts - generate.
	fs.On("Stat", "/etc/hosts").Return(nil, os.ErrNotExist)
	fs.On("ReadFile", "/etc/hostname").Return([]byte("k3os-99999\n"), nil)
	fs.On("WriteFile", "/etc/hosts", mock.AnythingOfType("[]uint8"), os.FileMode(0o644)).Return(nil)

	// SetupRoot - already exists.
	fi := fakeFileInfo{name: "root", isDir: true}
	fs.On("Stat", "/root").Return(fi, nil)

	// SetupTTYs - no devices.
	for i := 1; i <= 6; i++ {
		fs.On("Stat", fmt.Sprintf("/dev/tty%d", i)).Return(nil, os.ErrNotExist)
	}

	// SetupSudoers.
	fs.On("MkdirAll", "/etc/sudoers.d", os.FileMode(0o755)).Return(nil)
	fs.On("WriteFile", "/etc/sudoers.d/sudo", mock.AnythingOfType("[]uint8"), os.FileMode(0o440)).Return(nil)

	// SetupServices.
	fs.On("Symlink", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(nil)

	// SetupConfig - uses ConfigRunner (wired above as no-op).
	fs.On("Stat", "/etc/conf.d/udev-settle").Return(nil, os.ErrNotExist)
	fs.On("Stat", "/var/lib/connman/cloud-config.config").Return(nil, os.ErrNotExist)
	fs.On("Stat", "/etc/conf.d/cloud-config").Return(nil, os.ErrNotExist)
	fs.On("Stat", "/etc/conf.d/rngd").Return(nil, os.ErrNotExist)

	// SetupManifests.
	fs.On("MkdirAll", "/var/lib/rancher/k3s/server/manifests", os.FileMode(0o755)).Return(nil)

	// SetupStateDirs.
	fs.On("MkdirAll", "/var/lib/nfs", os.FileMode(0o755)).Return(nil)
	fs.On("MkdirAll", "/var/lib/rancher/k3s/agent/libexec/kubernetes", os.FileMode(0o755)).Return(nil)

	// Cleanup.
	fs.On("RemoveAll", "/run/k3os").Return(nil)
	fs.On("MkdirAll", "/run/k3os", os.FileMode(0o755)).Return(nil)
	fs.On("WriteFile", "/run/k3os/mode", []byte("local\n"), os.FileMode(0o644)).Return(nil)

	err := f.Run()
	require.NoError(t, err)
}

func TestFinalizer_Run_StopsOnError(t *testing.T) {
	t.Parallel()
	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}
	mnt := &MockMounter{}
	f := &Finalizer{
		FS:      fs,
		Cmd:     cmd,
		Mounter: mnt,
		Mode:    "local",
		CmdlineReader: func() (string, error) {
			return "", nil
		},
		RandFunc: func() (uint32, error) {
			return 0, nil
		},
		VirtDetector: func() ([]string, error) {
			return nil, nil
		},
		SleepFunc: func(time.Duration) {},
	}

	// SetupMounts fails immediately.
	fs.On("Stat", "/.base/boot").Return(nil, os.ErrNotExist)
	fs.On("Stat", "/.base/k3os/system").Return(nil, os.ErrNotExist)
	mnt.On("Mounted", "/.base").Return(false, errors.New("mount check failed"))

	err := f.Run()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mount check failed")
}

// ---------------------------------------------------------------------------
// Helper functions (direct tests)
// ---------------------------------------------------------------------------

func TestParseConsoleEntries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		cmdline  string
		expected []consoleEntry
	}{
		{
			name:     "no console entries",
			cmdline:  "root=/dev/sda1 ro quiet",
			expected: nil,
		},
		{
			name:    "single serial console",
			cmdline: "console=ttyS0,115200n8",
			expected: []consoleEntry{
				{tty: "ttyS0", baudrate: "115200"},
			},
		},
		{
			name:    "multiple consoles",
			cmdline: "console=tty0 console=ttyS0,9600",
			expected: []consoleEntry{
				{tty: "tty0", baudrate: "9600"},
				{tty: "ttyS0", baudrate: "9600"},
			},
		},
		{
			name:    "console with no baudrate",
			cmdline: "console=ttyS1",
			expected: []consoleEntry{
				{tty: "ttyS1", baudrate: "9600"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := parseConsoleEntries(tc.cmdline)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestExtractBaudrate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected string
	}{
		{"115200n8", "115200"},
		{"9600", "9600"},
		{"", "9600"},
		{"n8", "9600"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.expected, extractBaudrate(tc.input))
		})
	}
}

func TestSplitPartition(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		dev   string
		num   string
	}{
		{"/dev/sda2", "/dev/sda", "2"},
		{"/dev/nvme0n1p2", "/dev/nvme0n1", "2"},
		{"/dev/vda1", "/dev/vda", "1"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			dev, num := splitPartition(tc.input)
			assert.Equal(t, tc.dev, dev)
			assert.Equal(t, tc.num, num)
		})
	}
}

func TestTrimNewline(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected string
	}{
		{"hello\n", "hello"},
		{"hello\r\n", "hello"},
		{"hello", "hello"},
		{"\n", ""},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.expected, trimNewline(tc.input))
		})
	}
}
