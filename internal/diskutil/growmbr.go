//go:build linux

package diskutil

import (
	"fmt"
	"os"
	"unsafe"

	"github.com/rekby/mbr"
	"golang.org/x/sys/unix"
)

const sectorSize = 512

// growMBR grows an MBR partition to fill all remaining space on the disk.
// It reads the MBR, calculates the maximum partition length based on the
// device size, updates the partition entry, and writes the MBR back.
//
// This is a pure Go implementation using github.com/rekby/mbr — no shell-outs.
func growMBR(device string, partNum int) error {
	if partNum < 1 || partNum > 4 {
		return fmt.Errorf("invalid MBR partition number %d: must be 1-4", partNum)
	}

	// Open device for reading and writing.
	f, err := os.OpenFile(device, os.O_RDWR|unix.O_CLOEXEC, 0)
	if err != nil {
		return fmt.Errorf("open device %s: %w", device, err)
	}
	defer f.Close() //nolint:errcheck

	// Get total device size via ioctl.
	diskSize, err := getBlockDeviceSize(f)
	if err != nil {
		return fmt.Errorf("get device size for %s: %w", device, err)
	}

	totalSectors := diskSize / sectorSize
	if totalSectors == 0 {
		return fmt.Errorf("device %s has zero size", device)
	}

	// Read the MBR.
	table, err := mbr.Read(f)
	if err != nil {
		return fmt.Errorf("read MBR from %s: %w", device, err)
	}

	// Get the partition entry.
	part := table.GetPartition(partNum)
	if part == nil {
		return fmt.Errorf("partition %d not found in MBR on %s", partNum, device)
	}
	if part.IsEmpty() {
		return fmt.Errorf("partition %d is empty on %s", partNum, device)
	}

	// Calculate maximum length: from partition start to end of disk.
	start := uint64(part.GetLBAStart())
	maxLen := totalSectors - start
	if maxLen > 0xFFFFFFFF {
		// MBR can only address up to 2TB (uint32 sectors × 512 bytes).
		maxLen = 0xFFFFFFFF
	}

	currentLen := uint64(part.GetLBALen())
	if currentLen >= maxLen {
		// Already at maximum size, nothing to grow.
		return nil
	}

	// Grow the partition.
	part.SetLBALen(uint32(maxLen))

	// Write the updated MBR back to the device.
	if _, err := f.Seek(0, 0); err != nil {
		return fmt.Errorf("seek to start of %s: %w", device, err)
	}
	if err := table.Write(f); err != nil {
		return fmt.Errorf("write MBR to %s: %w", device, err)
	}

	// Sync to ensure the write hits disk.
	if err := f.Sync(); err != nil {
		return fmt.Errorf("sync %s: %w", device, err)
	}

	return nil
}

// getBlockDeviceSize returns the size of a block device in bytes.
// Uses BLKGETSIZE64 ioctl for block devices, falls back to Stat for regular files.
func getBlockDeviceSize(f *os.File) (uint64, error) {
	info, err := f.Stat()
	if err != nil {
		return 0, err
	}

	// For regular files (disk images), use the file size directly.
	if info.Mode().IsRegular() {
		return uint64(info.Size()), nil
	}

	// For block devices, use the BLKGETSIZE64 ioctl.
	var size uint64
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, f.Fd(), unix.BLKGETSIZE64, uintptr(unsafe.Pointer(&size)))
	if errno != 0 {
		return 0, fmt.Errorf("BLKGETSIZE64: %w", errno)
	}
	return size, nil
}
