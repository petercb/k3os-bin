//go:build linux

package diskutil

import (
	"testing"

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
	assert.Contains(t, err.Error(), "open block device")
}

func TestGPTPartitionGrower_ImplementsInterface(t *testing.T) {
	t.Parallel()

	var _ PartitionGrower = (*GPTPartitionGrower)(nil)
}
