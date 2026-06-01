//go:build linux

package osimpl

import "github.com/petercb/k3os-bin/internal/diskutil"

// PartitionGrower is a type alias for diskutil.GPTPartitionGrower, which
// directly satisfies iface.PartitionGrower.
type PartitionGrower = diskutil.GPTPartitionGrower
