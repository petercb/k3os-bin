//go:build linux

// Package diskutil provides pure Go disk manipulation utilities.
package diskutil

import (
	"fmt"
	"log/slog"

	"github.com/siderolabs/go-blockdevice/v2/block"
	"github.com/siderolabs/go-blockdevice/v2/partitioning/gpt"
)

// GPTPartitionGrower implements iface.PartitionGrower for GPT disks only.
//
// Deprecated: Use PartitionGrower instead, which handles both GPT and MBR.
type GPTPartitionGrower = PartitionGrower

// PartitionGrower implements iface.PartitionGrower with support for both GPT
// and MBR (DOS) partition tables. It detects the partition table type and
// dispatches to the appropriate grow strategy:
//   - GPT: pure Go via go-blockdevice/v2 (reads/writes GPT directly)
//   - MBR: shells out to sfdisk (the standard Linux MBR manipulation tool)
type PartitionGrower struct{}

// GrowPartition detects the partition table type on the device and grows the
// specified partition (1-indexed) to fill all available contiguous space.
func (g *PartitionGrower) GrowPartition(device string, partNum int) error {
	if partNum < 1 {
		return fmt.Errorf("invalid partition number %d: must be >= 1", partNum)
	}

	tableType := ProbePartitionTableType(device)
	slog.Debug("diskutil: detected partition table", "device", device, "type", string(tableType))

	switch tableType {
	case PartitionTableGPT:
		return growGPT(device, partNum)
	case PartitionTableMBR:
		return growMBR(device, partNum)
	default:
		return fmt.Errorf("unsupported partition table type on %s: %s", device, tableType)
	}
}

// growGPT grows a GPT partition using go-blockdevice/v2.
func growGPT(device string, partNum int) error {
	blockDev, err := block.NewFromPath(device, block.OpenForWrite())
	if err != nil {
		return fmt.Errorf("open block device %s: %w", device, err)
	}
	defer blockDev.Close() //nolint:errcheck

	gptDev, err := gpt.DeviceFromBlockDevice(blockDev)
	if err != nil {
		return fmt.Errorf("create gpt device for %s: %w", device, err)
	}

	table, err := gpt.Read(gptDev)
	if err != nil {
		return fmt.Errorf("read GPT from %s: %w", device, err)
	}

	// Library uses 0-indexed partitions; callers use 1-indexed.
	idx := partNum - 1

	growth, err := table.AvailablePartitionGrowth(idx)
	if err != nil {
		return fmt.Errorf("check available growth for partition %d on %s: %w", partNum, device, err)
	}

	if growth == 0 {
		// Nothing to grow.
		return nil
	}

	if err := table.GrowPartition(idx, growth); err != nil {
		return fmt.Errorf("grow partition %d on %s: %w", partNum, device, err)
	}

	if err := table.Write(); err != nil {
		return fmt.Errorf("write GPT to %s: %w", device, err)
	}

	return nil
}
