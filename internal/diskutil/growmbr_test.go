//go:build linux

package diskutil

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGrowMBR_InvalidPartNum(t *testing.T) {
	t.Parallel()

	err := growMBR("/dev/nonexistent", 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid partition number")
}

func TestGrowMBR_NonexistentDevice(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("sfdisk"); err != nil {
		t.Skip("sfdisk not found in PATH")
	}

	err := growMBR("/dev/nonexistent-xyz-device", 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "sfdisk")
}

// TestGrowMBR_Integration creates an MBR disk image with a small partition,
// attaches it as a loop device, grows it, and verifies the partition expanded.
func TestGrowMBR_Integration(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("test requires root privileges (loop device setup)")
	}
	if _, err := exec.LookPath("sfdisk"); err != nil {
		t.Skip("sfdisk not found in PATH")
	}
	if _, err := exec.LookPath("losetup"); err != nil {
		t.Skip("losetup not found in PATH")
	}

	const (
		diskSize   = 100 * 1024 * 1024 // 100 MiB
		sectorSize = 512
	)

	tmpDir := t.TempDir()
	imgPath := tmpDir + "/mbr-disk.img"

	// Create a sparse disk image.
	f, err := os.Create(imgPath)
	require.NoError(t, err)
	require.NoError(t, f.Truncate(diskSize))
	require.NoError(t, f.Close())

	// Create MBR with a small partition (20 MiB) using sfdisk.
	// Partition starts at sector 2048, size = 20 MiB = 40960 sectors.
	sfdiskInput := "2048 40960 83\n"
	cmd := exec.Command("sfdisk", "--label", "dos", imgPath)
	cmd.Stdin = strings.NewReader(sfdiskInput)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "sfdisk failed: %s", string(out))

	// Attach as loop device.
	loOut, err := exec.Command("losetup", "--find", "--show", imgPath).CombinedOutput()
	if err != nil {
		t.Skipf("cannot attach loop device: %s", string(loOut))
	}
	loopDev := strings.TrimSpace(string(loOut))
	require.NotEmpty(t, loopDev)

	t.Cleanup(func() {
		_ = exec.Command("losetup", "-d", loopDev).Run()
	})

	// Read partition size before grow.
	beforeOut, err := exec.Command("sfdisk", "--dump", loopDev).CombinedOutput()
	require.NoError(t, err, "sfdisk --dump failed: %s", string(beforeOut))

	// Verify partition 1 exists and is small.
	assert.Contains(t, string(beforeOut), "size=")

	// Grow partition 1.
	err = growMBR(loopDev, 1)
	require.NoError(t, err)

	// Read partition size after grow.
	afterOut, err := exec.Command("sfdisk", "--dump", loopDev).CombinedOutput()
	require.NoError(t, err, "sfdisk --dump after grow failed: %s", string(afterOut))

	// Parse the size fields to verify growth.
	beforeSize := extractSfdiskSize(t, string(beforeOut), 1)
	afterSize := extractSfdiskSize(t, string(afterOut), 1)

	assert.Greater(t, afterSize, beforeSize,
		"partition should have grown: before=%d, after=%d sectors", beforeSize, afterSize)

	// Verify it grew significantly (at least 50 MiB worth).
	grownBytes := (afterSize - beforeSize) * sectorSize
	assert.Greater(t, grownBytes, int64(50*1024*1024),
		"partition should have grown by at least 50 MiB")
}

// extractSfdiskSize parses sfdisk --dump output and returns the size in sectors
// for the given partition number (1-indexed).
func extractSfdiskSize(t *testing.T, dump string, partNum int) int64 {
	t.Helper()
	// sfdisk --dump output lines look like:
	// /dev/loop0p1 : start=     2048, size=    40960, type=83
	for _, line := range strings.Split(dump, "\n") {
		if !strings.Contains(line, "size=") {
			continue
		}
		if !strings.Contains(line, "start=") {
			continue
		}
		// This is a partition line. Count which partition number it is.
		partNum--
		if partNum > 0 {
			continue
		}
		// Extract size= value.
		for _, field := range strings.Split(line, ",") {
			field = strings.TrimSpace(field)
			if strings.HasPrefix(field, "size=") {
				sizeStr := strings.TrimSpace(strings.TrimPrefix(field, "size="))
				var size int64
				_, err := strings.NewReader(sizeStr).Read(nil)
				_ = err
				// Parse the integer.
				for _, c := range sizeStr {
					if c >= '0' && c <= '9' {
						size = size*10 + int64(c-'0')
					} else {
						break
					}
				}
				return size
			}
		}
	}
	t.Fatalf("could not find partition %d size in sfdisk dump:\n%s", partNum, dump)
	return 0
}
