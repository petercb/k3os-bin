//go:build linux

// Package diskutil provides pure Go disk manipulation utilities.
package diskutil

import (
	"fmt"

	"github.com/siderolabs/go-blockdevice/v2/block"
	"github.com/siderolabs/go-blockdevice/v2/partitioning/gpt"
)

// GPTPartitionGrower implements iface.PartitionGrower using go-blockdevice/v2.
type GPTPartitionGrower struct{}

// GrowPartition opens the block device, reads its GPT, and grows the
// specified partition (1-indexed) to fill all available contiguous space.
// It writes the updated GPT and syncs the kernel partition table via BLKPG
// ioctls (no external partprobe needed).
func (g *GPTPartitionGrower) GrowPartition(device string, partNum int) error {
	if partNum < 1 {
		return fmt.Errorf("invalid partition number %d: must be >= 1", partNum)
	}

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
