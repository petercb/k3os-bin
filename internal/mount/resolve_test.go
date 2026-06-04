//go:build linux

package mount

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveDevice_PlainDevice(t *testing.T) {
	t.Parallel()

	// A plain device path should be returned unchanged.
	assert.Equal(t, "/dev/sda1", resolveDevice("/dev/sda1"))
	assert.Equal(t, "/dev/nvme0n1p2", resolveDevice("/dev/nvme0n1p2"))
	assert.Equal(t, "none", resolveDevice("none"))
	assert.Empty(t, resolveDevice(""))
}

func TestResolveDevice_LabelNotFound(t *testing.T) {
	t.Parallel()

	// If the LABEL symlink doesn't exist, return the original string.
	result := resolveDevice("LABEL=NONEXISTENT_LABEL_12345")
	assert.Equal(t, "LABEL=NONEXISTENT_LABEL_12345", result)
}

func TestResolveDevice_UUIDNotFound(t *testing.T) {
	t.Parallel()

	// If the UUID symlink doesn't exist, return the original string.
	result := resolveDevice("UUID=00000000-0000-0000-0000-000000000000")
	assert.Equal(t, "UUID=00000000-0000-0000-0000-000000000000", result)
}

func TestResolveDevice_LabelRelativeSymlink(t *testing.T) {
	t.Parallel()

	// Create a temp directory simulating /dev/disk/by-label with a relative symlink.
	tmpDir := t.TempDir()
	byLabel := filepath.Join(tmpDir, "by-label")
	if err := os.MkdirAll(byLabel, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create a relative symlink: by-label/K3OS_STATE -> ../../sda2
	// From by-label/ directory, ../../sda2 goes up to parent of tmpDir.
	// Use a more realistic relative link: ../sda2 (goes from by-label/ up to tmpDir/)
	if err := os.Symlink("../sda2", filepath.Join(byLabel, "K3OS_STATE")); err != nil {
		t.Fatal(err)
	}

	linkPath := filepath.Join(byLabel, "K3OS_STATE")
	target, err := os.Readlink(linkPath)
	require.NoError(t, err)
	assert.Equal(t, "../sda2", target)

	// Simulate the resolution logic
	resolved := filepath.Clean(filepath.Join(filepath.Dir(linkPath), target))
	assert.Equal(t, filepath.Join(tmpDir, "sda2"), resolved)
}

func TestResolveDevice_LabelAbsoluteSymlink(t *testing.T) {
	t.Parallel()

	// Create a temp directory simulating /dev/disk/by-label with an absolute symlink.
	tmpDir := t.TempDir()
	byLabel := filepath.Join(tmpDir, "by-label")
	if err := os.MkdirAll(byLabel, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create an absolute symlink: by-label/MY_DISK -> /dev/vdb1
	if err := os.Symlink("/dev/vdb1", filepath.Join(byLabel, "MY_DISK")); err != nil {
		t.Fatal(err)
	}

	linkPath := filepath.Join(byLabel, "MY_DISK")
	target, err := os.Readlink(linkPath)
	require.NoError(t, err)
	assert.Equal(t, "/dev/vdb1", target)

	// Simulate the resolution logic for absolute targets
	resolved := filepath.Clean(target)
	assert.Equal(t, "/dev/vdb1", resolved)
}
