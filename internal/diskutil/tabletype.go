//go:build linux

package diskutil

import (
	"os"
)

// PartitionTableType represents the type of partition table on a device.
type PartitionTableType string

const (
	// PartitionTableGPT indicates a GUID Partition Table.
	PartitionTableGPT PartitionTableType = "gpt"
	// PartitionTableMBR indicates an MBR (DOS) partition table.
	PartitionTableMBR PartitionTableType = "dos"
	// PartitionTableUnknown indicates the partition table type could not be determined.
	PartitionTableUnknown PartitionTableType = "unknown"
)

const (
	// mbrSignatureOffset is the offset of the MBR boot signature (0x55AA).
	mbrSignatureOffset = 510
	// mbrPartitionTableOffset is the start of the MBR partition table entries.
	mbrPartitionTableOffset = 446
	// mbrPartitionTypeOffset is the offset of the type byte within an entry.
	mbrPartitionTypeOffset = 4
	// mbrPartitionTypeGPTProtective is the partition type for GPT protective MBR.
	mbrPartitionTypeGPTProtective = 0xEE
	// gptHeaderOffset is the byte offset of the GPT header (sector 1 for 512-byte sectors).
	gptHeaderOffset = 512
	// gptMagic is the GPT header signature.
	gptMagic = "EFI PART"
	// minDeviceSize is the minimum size needed to read partition table info.
	minDeviceSize = gptHeaderOffset + 8
)

// ProbePartitionTableType reads the first two sectors of a device (or file)
// and determines whether it contains a GPT or MBR partition table.
//
// Detection logic:
//  1. Check for MBR boot signature (0x55AA at offset 510).
//  2. If signature present, check for GPT protective MBR (type 0xEE in first
//     partition entry) AND "EFI PART" magic at sector 1.
//  3. If both GPT indicators are present → GPT.
//  4. If MBR signature present but no GPT indicators → MBR.
//  5. Otherwise → Unknown.
func ProbePartitionTableType(devicePath string) PartitionTableType {
	f, err := os.Open(devicePath)
	if err != nil {
		return PartitionTableUnknown
	}
	defer f.Close() //nolint:errcheck

	buf := make([]byte, minDeviceSize)
	n, err := f.Read(buf)
	if err != nil || n < minDeviceSize {
		return PartitionTableUnknown
	}

	// Check MBR boot signature.
	if buf[mbrSignatureOffset] != 0x55 || buf[mbrSignatureOffset+1] != 0xAA {
		return PartitionTableUnknown
	}

	// Check for GPT: protective MBR (first partition type = 0xEE) + GPT header magic.
	firstPartType := buf[mbrPartitionTableOffset+mbrPartitionTypeOffset]
	hasGPTMagic := string(buf[gptHeaderOffset:gptHeaderOffset+8]) == gptMagic

	if firstPartType == mbrPartitionTypeGPTProtective && hasGPTMagic {
		return PartitionTableGPT
	}

	// Has MBR signature but no GPT indicators → DOS/MBR partition table.
	return PartitionTableMBR
}
