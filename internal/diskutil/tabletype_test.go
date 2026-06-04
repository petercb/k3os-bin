//go:build linux

package diskutil

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProbePartitionTableType_GPT(t *testing.T) {
	t.Parallel()

	// Create a minimal GPT disk image:
	// - Protective MBR at sector 0 with partition type 0xEE
	// - GPT header at sector 1 with magic "EFI PART"
	imgPath := t.TempDir() + "/gpt.img"
	img := make([]byte, 1024) // 2 sectors

	// MBR boot signature at bytes 510-511
	img[510] = 0x55
	img[511] = 0xAA
	// First partition entry starts at offset 446, type byte at offset 450
	// Set type to 0xEE (GPT protective MBR)
	img[450] = 0xEE

	// GPT header magic "EFI PART" at sector 1 (offset 512)
	copy(img[512:520], []byte("EFI PART"))

	require.NoError(t, os.WriteFile(imgPath, img, 0o644))

	result := ProbePartitionTableType(imgPath)
	assert.Equal(t, PartitionTableGPT, result)
}

func TestProbePartitionTableType_MBR(t *testing.T) {
	t.Parallel()

	// Create a minimal MBR disk image:
	// - Boot signature 0x55AA at bytes 510-511
	// - Partition type != 0xEE (use 0x83 for Linux)
	// - Must be at least minDeviceSize (520) bytes for detection.
	imgPath := t.TempDir() + "/mbr.img"
	img := make([]byte, minDeviceSize)

	// MBR boot signature
	img[510] = 0x55
	img[511] = 0xAA
	// First partition entry type at offset 450 = 0x83 (Linux)
	img[450] = 0x83

	require.NoError(t, os.WriteFile(imgPath, img, 0o644))

	result := ProbePartitionTableType(imgPath)
	assert.Equal(t, PartitionTableMBR, result)
}

func TestProbePartitionTableType_Unknown_NoSignature(t *testing.T) {
	t.Parallel()

	// Create a file with no valid partition table signature.
	imgPath := t.TempDir() + "/empty.img"
	img := make([]byte, minDeviceSize)
	// No 0x55AA signature

	require.NoError(t, os.WriteFile(imgPath, img, 0o644))

	result := ProbePartitionTableType(imgPath)
	assert.Equal(t, PartitionTableUnknown, result)
}

func TestProbePartitionTableType_Unknown_FileTooSmall(t *testing.T) {
	t.Parallel()

	imgPath := t.TempDir() + "/tiny.img"
	img := make([]byte, 100) // Too small to contain a partition table

	require.NoError(t, os.WriteFile(imgPath, img, 0o644))

	result := ProbePartitionTableType(imgPath)
	assert.Equal(t, PartitionTableUnknown, result)
}

func TestProbePartitionTableType_Unknown_NonexistentFile(t *testing.T) {
	t.Parallel()

	result := ProbePartitionTableType("/nonexistent/device/path")
	assert.Equal(t, PartitionTableUnknown, result)
}

func TestProbePartitionTableType_MBR_MultiplePartitions(t *testing.T) {
	t.Parallel()

	// MBR with two partitions (FAT32 + Linux), none is 0xEE
	imgPath := t.TempDir() + "/mbr_multi.img"
	img := make([]byte, minDeviceSize)

	// MBR boot signature
	img[510] = 0x55
	img[511] = 0xAA
	// Partition 1 at offset 446: type at +4 = offset 450 → 0x0C (FAT32 LBA)
	img[450] = 0x0C
	// Partition 2 at offset 462: type at +4 = offset 466 → 0x83 (Linux)
	img[466] = 0x83

	require.NoError(t, os.WriteFile(imgPath, img, 0o644))

	result := ProbePartitionTableType(imgPath)
	assert.Equal(t, PartitionTableMBR, result)
}
