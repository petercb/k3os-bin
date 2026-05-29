//go:build linux

package enterchroot

import (
	"errors"
	"os"
	"strings"
	"testing"
	"testing/quick"

	"github.com/petercb/k3os-bin/internal/iface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockLoopDevice implements iface.LoopDevice for testing.
type mockLoopDevice struct {
	path            string
	detachErr       error
	detachCalled    bool
	autoclearErr    error
	autoclearCalled bool
}

func (m *mockLoopDevice) Path() string        { return m.path }
func (m *mockLoopDevice) Detach() error       { m.detachCalled = true; return m.detachErr }
func (m *mockLoopDevice) SetAutoclear() error { m.autoclearCalled = true; return m.autoclearErr }

// mockLoopAttacher implements iface.LoopAttacher for testing.
type mockLoopAttacher struct {
	dev *mockLoopDevice
	err error
}

func (m *mockLoopAttacher) Attach(_ string, _ uint64, _ bool) (iface.LoopDevice, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.dev, nil
}

func writeTempFile(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "proc_filesystems")
	require.NoError(t, err)
	_, err = f.WriteString(content)
	require.NoError(t, err)
	require.NoError(t, f.Close())
	return f.Name()
}

func TestInProcFS_WithSquashfs(t *testing.T) {
	tmp := writeTempFile(t, "nodev\tsquashfs\n")
	orig := procFilesystemsPath
	procFilesystemsPath = tmp
	t.Cleanup(func() { procFilesystemsPath = orig })

	assert.True(t, inProcFS())
}

func TestInProcFS_WithoutSquashfs(t *testing.T) {
	tmp := writeTempFile(t, "nodev\text4\n")
	orig := procFilesystemsPath
	procFilesystemsPath = tmp
	t.Cleanup(func() { procFilesystemsPath = orig })

	assert.False(t, inProcFS())
}

func TestCheckSquashfs_ReturnsError_WhenNotSupported(t *testing.T) {
	tmp := writeTempFile(t, "nodev\text4\n")
	orig := procFilesystemsPath
	procFilesystemsPath = tmp
	t.Cleanup(func() { procFilesystemsPath = orig })

	err := checkSquashfs()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "squashfs")
}

func TestCheckSquashfs_ReturnsNil_WhenSupported(t *testing.T) {
	tmp := writeTempFile(t, "\tsquashfs\n")
	orig := procFilesystemsPath
	procFilesystemsPath = tmp
	t.Cleanup(func() { procFilesystemsPath = orig })

	assert.NoError(t, checkSquashfs())
}

// TestInProcFS_Property verifies Property 3: inProcFS filesystem detection correctness.
// For any content string, inProcFS() returns true iff the content contains "squashfs".
//
// **Validates: Requirements 5.2, 5.3**
func TestInProcFS_Property(t *testing.T) {
	property := func(content string) bool {
		tmp := writeTempFile(t, content)
		orig := procFilesystemsPath
		procFilesystemsPath = tmp
		defer func() { procFilesystemsPath = orig }()

		got := inProcFS()
		want := strings.Contains(content, "squashfs")
		return got == want
	}

	if err := quick.Check(property, nil); err != nil {
		t.Errorf("property failed: %v", err)
	}
}

func TestMount_AttachFailure_ReturnsError(t *testing.T) {
	// Override ensureLoopFn to skip privileged operations in CI.
	origEnsureLoop := ensureLoopFn
	ensureLoopFn = func() error { return nil }
	t.Cleanup(func() { ensureLoopFn = origEnsureLoop })

	// Create a temp file to act as a non-directory root (triggers loop attach).
	tmpFile, err := os.CreateTemp(t.TempDir(), "root")
	require.NoError(t, err)
	require.NoError(t, tmpFile.Close())

	// Override loopAttacher to return an error.
	origAttacher := loopAttacher
	loopAttacher = &mockLoopAttacher{err: errors.New("attach failed")}
	t.Cleanup(func() { loopAttacher = origAttacher })

	// Override findRoot to return our temp file.
	t.Setenv("ENTER_ROOT", tmpFile.Name())

	err = Mount(t.TempDir(), []string{"test"}, os.Stdout, os.Stderr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "creating loopback device")
}

func TestMount_AttachSuccess_SetsEnvDevice(t *testing.T) {
	// Override ensureLoopFn to skip privileged operations in CI.
	origEnsureLoop := ensureLoopFn
	ensureLoopFn = func() error { return nil }
	t.Cleanup(func() { ensureLoopFn = origEnsureLoop })

	// Create a temp file to act as a non-directory root (triggers loop attach).
	tmpFile, err := os.CreateTemp(t.TempDir(), "root")
	require.NoError(t, err)
	require.NoError(t, tmpFile.Close())

	dev := &mockLoopDevice{path: "/dev/loop99"}
	origAttacher := loopAttacher
	loopAttacher = &mockLoopAttacher{dev: dev}
	t.Cleanup(func() { loopAttacher = origAttacher })

	t.Setenv("ENTER_ROOT", tmpFile.Name())

	// Mount will fail later (no reexec binary), but we can verify the
	// loop device path was set in the environment.
	_ = Mount(t.TempDir(), []string{"test"}, os.Stdout, os.Stderr)

	assert.Equal(t, "/dev/loop99", os.Getenv("ENTER_DEVICE"))
	assert.True(t, dev.detachCalled)
}
