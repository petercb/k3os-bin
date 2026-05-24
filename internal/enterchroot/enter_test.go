//go:build linux

package enterchroot

import (
	"os"
	"strings"
	"testing"
	"testing/quick"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
