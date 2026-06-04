//go:build linux

package diskutil

import (
	"os"
	"testing"

	"github.com/rekby/mbr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGrowMBR_InvalidPartNum(t *testing.T) {
	t.Parallel()

	err := growMBR("/dev/nonexistent", 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid MBR partition number")

	err = growMBR("/dev/nonexistent", 5)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid MBR partition number")
}

func TestGrowMBR_NonexistentDevice(t *testing.T) {
	t.Parallel()

	err := growMBR("/dev/nonexistent-xyz-device", 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "open device")
}

func TestGrowMBR_EmptyPartition(t *testing.T) {
	t.Parallel()

	// Create a disk image with MBR but partition 2 is empty.
	const diskSize = 100 * 1024 * 1024 // 100 MiB
	imgPath := t.TempDir() + "/mbr-empty.img"

	f, err := os.Create(imgPath)
	require.NoError(t, err)
	require.NoError(t, f.Truncate(diskSize))

	// Write a valid MBR with partition 1 only.
	mbrTable := createTestMBR(t, 2048, 40960) // partition 1: start=2048, len=40960
	_, err = f.Seek(0, 0)
	require.NoError(t, err)
	require.NoError(t, mbrTable.Write(f))
	require.NoError(t, f.Close())

	// Trying to grow partition 2 (which is empty) should error.
	err = growMBR(imgPath, 2)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}

func TestGrowMBR_GrowsPartition(t *testing.T) {
	t.Parallel()

	// Create a 100 MiB disk image with a 20 MiB partition.
	const diskSize = 100 * 1024 * 1024 // 100 MiB
	const partStart = uint32(2048)     // sector 2048
	const partLen = uint32(40960)      // 20 MiB in sectors

	imgPath := t.TempDir() + "/mbr-grow.img"

	f, err := os.Create(imgPath)
	require.NoError(t, err)
	require.NoError(t, f.Truncate(diskSize))

	// Write MBR with a small partition 1.
	mbrTable := createTestMBR(t, partStart, partLen)
	_, err = f.Seek(0, 0)
	require.NoError(t, err)
	require.NoError(t, mbrTable.Write(f))
	require.NoError(t, f.Close())

	// Verify initial partition size.
	verifyPartLen(t, imgPath, 1, partLen)

	// Grow partition 1.
	err = growMBR(imgPath, 1)
	require.NoError(t, err)

	// Verify partition grew to fill the disk.
	totalSectors := uint32(diskSize / sectorSize)
	expectedLen := totalSectors - partStart
	verifyPartLen(t, imgPath, 1, expectedLen)
}

func TestGrowMBR_AlreadyMaxSize(t *testing.T) {
	t.Parallel()

	// Create a disk where the partition already fills the disk.
	const diskSize = 100 * 1024 * 1024
	const partStart = uint32(2048)
	totalSectors := uint32(diskSize / sectorSize)
	maxLen := totalSectors - partStart

	imgPath := t.TempDir() + "/mbr-full.img"

	f, err := os.Create(imgPath)
	require.NoError(t, err)
	require.NoError(t, f.Truncate(diskSize))

	mbrTable := createTestMBR(t, partStart, maxLen)
	_, err = f.Seek(0, 0)
	require.NoError(t, err)
	require.NoError(t, mbrTable.Write(f))
	require.NoError(t, f.Close())

	// Grow should be a no-op (no error, no change).
	err = growMBR(imgPath, 1)
	require.NoError(t, err)

	// Verify length unchanged.
	verifyPartLen(t, imgPath, 1, maxLen)
}

// createTestMBR creates an MBR with one Linux partition (type 0x83).
func createTestMBR(t *testing.T, startLBA, lenLBA uint32) *mbr.MBR {
	t.Helper()

	// Build a raw MBR manually with signature.
	raw := make([]byte, 512)
	raw[510] = 0x55
	raw[511] = 0xAA

	// Create MBR from raw bytes by reading them.
	r := &byteReader{data: raw}
	table, err := mbr.Read(r)
	require.NoError(t, err)

	part := table.GetPartition(1)
	part.SetType(0x83) // Linux
	part.SetLBAStart(startLBA)
	part.SetLBALen(lenLBA)

	return table
}

// verifyPartLen reads the MBR from the image and asserts partition length.
func verifyPartLen(t *testing.T, imgPath string, partNum int, expectedLen uint32) {
	t.Helper()

	f, err := os.Open(imgPath)
	require.NoError(t, err)
	defer f.Close() //nolint:errcheck

	table, err := mbr.Read(f)
	require.NoError(t, err)

	part := table.GetPartition(partNum)
	require.NotNil(t, part)
	assert.Equal(t, expectedLen, part.GetLBALen(), "partition %d length mismatch", partNum)
}

// byteReader is a simple io.Reader wrapping a byte slice.
type byteReader struct {
	data []byte
	pos  int
}

func (r *byteReader) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, os.ErrClosed
	}
	n = copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}
