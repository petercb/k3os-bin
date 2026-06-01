//go:build linux

package osimpl

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSysfsBlockProber_FindByLabel_NotExist(t *testing.T) {
	t.Parallel()

	bp := SysfsBlockProber{}
	_, err := bp.FindByLabel("NONEXISTENT_LABEL_12345")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "readlink")
}

func TestSysfsBlockProber_FindByLabel_InvalidLabel(t *testing.T) {
	t.Parallel()

	bp := SysfsBlockProber{}

	_, err := bp.FindByLabel("../by-uuid/something")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid label")

	_, err = bp.FindByLabel("foo/bar")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid label")
}

func TestSysfsBlockProber_ListDisks_NoPanic(t *testing.T) {
	t.Parallel()

	bp := SysfsBlockProber{}
	disks, err := bp.ListDisks()
	if err != nil {
		// /sys/block may not exist in all environments (e.g., containers).
		t.Skipf("skipping: /sys/block not available: %v", err)
	}
	// Verify no panic and that virtual devices are filtered.
	for _, d := range disks {
		assert.False(t, len(d) >= 4 && d[:4] == "loop", "loop device should be filtered: %s", d)
		assert.False(t, len(d) >= 3 && d[:3] == "ram", "ram device should be filtered: %s", d)
		assert.False(t, len(d) >= 3 && d[:3] == "dm-", "dm device should be filtered: %s", d)
		assert.False(t, len(d) >= 4 && d[:4] == "zram", "zram device should be filtered: %s", d)
		assert.False(t, len(d) >= 3 && d[:3] == "nbd", "nbd device should be filtered: %s", d)
		assert.False(t, len(d) >= 2 && d[:2] == "sr", "sr device should be filtered: %s", d)
		assert.False(t, len(d) >= 2 && d[:2] == "md", "md device should be filtered: %s", d)
	}
}
