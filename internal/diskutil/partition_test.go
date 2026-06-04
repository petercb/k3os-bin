//go:build linux

package diskutil

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/petercb/k3os-bin/internal/iface"
	"github.com/siderolabs/go-blockdevice/v2/block"
	"github.com/siderolabs/go-blockdevice/v2/partitioning/gpt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGPTPartitionGrower_InvalidPartNum(t *testing.T) {
	t.Parallel()

	grower := &GPTPartitionGrower{}

	err := grower.GrowPartition("/dev/nonexistent", 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid partition number 0")

	err = grower.GrowPartition("/dev/nonexistent", -1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid partition number -1")
}

func TestGPTPartitionGrower_DeviceOpenFailure(t *testing.T) {
	t.Parallel()

	grower := &GPTPartitionGrower{}

	err := grower.GrowPartition("/dev/nonexistent-device-xyz", 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported partition table type")
}

func TestGPTPartitionGrower_ImplementsInterface(t *testing.T) {
	t.Parallel()

	// Verify GPTPartitionGrower satisfies iface.PartitionGrower.
	var _ iface.PartitionGrower = (*GPTPartitionGrower)(nil)
}

// TestGPTPartitionGrower_Integration creates a sparse file, attaches it as a
// loop device, writes a GPT with a partition that does not fill the disk, then
// calls GrowPartition and verifies the partition expanded to fill available space.
func TestGPTPartitionGrower_Integration(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("test requires root privileges (loop device setup)")
	}

	// Check that losetup is available.
	if _, err := exec.LookPath("losetup"); err != nil {
		t.Skip("losetup not found in PATH")
	}

	const (
		diskSize     = 100 * 1024 * 1024 // 100 MiB
		partSize     = 20 * 1024 * 1024  // 20 MiB (leaves ~80 MiB free)
		sectorSize   = 512
		partTypeGUID = "0FC63DAF-8483-4772-8E79-3D69D8477DE4" // Linux filesystem
	)

	tmpDir := t.TempDir()
	imgPath := tmpDir + "/disk.img"

	// Create a sparse file.
	f, err := os.Create(imgPath)
	require.NoError(t, err)
	require.NoError(t, f.Truncate(diskSize))
	require.NoError(t, f.Close())

	// Attach as loop device.
	out, err := exec.Command("losetup", "--find", "--show", imgPath).CombinedOutput()
	if err != nil {
		t.Skipf("cannot attach loop device (no loop device support in this environment): %s", string(out))
	}
	loopDev := strings.TrimSpace(string(out))
	require.NotEmpty(t, loopDev)

	t.Cleanup(func() {
		_ = exec.Command("losetup", "-d", loopDev).Run()
	})

	// Write a GPT with one partition that only takes 20 MiB.
	blkDev, err := block.NewFromPath(loopDev, block.OpenForWrite())
	require.NoError(t, err)

	gptDev, err := gpt.DeviceFromBlockDevice(blkDev)
	require.NoError(t, err)

	table, err := gpt.New(gptDev)
	require.NoError(t, err)

	partType := uuid.MustParse(partTypeGUID)
	_, _, err = table.AllocatePartition(partSize, "STATE", partType)
	require.NoError(t, err)

	require.NoError(t, table.Write())
	require.NoError(t, blkDev.Close())

	// Read back and verify partition does not fill the disk.
	blkDev2, err := block.NewFromPath(loopDev, block.OpenForWrite())
	require.NoError(t, err)

	gptDev2, err := gpt.DeviceFromBlockDevice(blkDev2)
	require.NoError(t, err)

	tableBefore, err := gpt.Read(gptDev2)
	require.NoError(t, err)

	partsBefore := tableBefore.Partitions()
	require.Len(t, partsBefore, 1)
	originalLastLBA := partsBefore[0].LastLBA
	require.NoError(t, blkDev2.Close())

	// Now call GrowPartition via our implementation.
	grower := &GPTPartitionGrower{}
	err = grower.GrowPartition(loopDev, 1)
	require.NoError(t, err)

	// Read back the GPT and verify partition grew.
	blkDev3, err := block.NewFromPath(loopDev)
	require.NoError(t, err)

	gptDev3, err := gpt.DeviceFromBlockDevice(blkDev3)
	require.NoError(t, err)

	tableAfter, err := gpt.Read(gptDev3)
	require.NoError(t, err)

	partsAfter := tableAfter.Partitions()
	require.Len(t, partsAfter, 1)
	require.NoError(t, blkDev3.Close())

	// The partition should have grown significantly.
	assert.Greater(t, partsAfter[0].LastLBA, originalLastLBA,
		"partition LastLBA should have grown after GrowPartition")

	// Verify it grew by a meaningful amount (at least 50 MiB worth of sectors).
	grownSectors := partsAfter[0].LastLBA - originalLastLBA
	grownBytes := grownSectors * sectorSize
	assert.Greater(t, grownBytes, uint64(50*1024*1024),
		"partition should have grown by at least 50 MiB")
}
