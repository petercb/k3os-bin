//go:build linux

package osimpl

import "github.com/petercb/k3os-bin/internal/diskutil"

// PartitionGrower is a type alias for diskutil.PartitionGrower, which
// supports both GPT and MBR partition tables and satisfies iface.PartitionGrower.
type PartitionGrower = diskutil.PartitionGrower
