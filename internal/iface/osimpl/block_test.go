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
	}
}
