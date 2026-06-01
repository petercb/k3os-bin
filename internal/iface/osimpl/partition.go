//go:build linux

package osimpl

import "github.com/petercb/k3os-bin/internal/diskutil"

// PartitionGrower wraps diskutil.GPTPartitionGrower to satisfy iface.PartitionGrower.
type PartitionGrower struct {
	impl diskutil.GPTPartitionGrower
}

// GrowPartition grows the specified partition to fill available space.
func (p *PartitionGrower) GrowPartition(device string, partNum int) error {
	return p.impl.GrowPartition(device, partNum)
}
