# Replace Disk Shell-outs with go-blockdevice

## Summary

Migrate disk-related shell-outs to pure Go using
`github.com/siderolabs/go-blockdevice/v2` for partition table operations
and the existing `BlockProber` sysfs interface for device discovery.

## Motivation

Reducing external binary dependencies improves:
- Boot reliability (no PATH or missing binary issues)
- Testability (pure Go interfaces are mockable)
- Static binary correctness (no runtime dependency on parted/partprobe/lsblk)

## Libraries Evaluated

### siderolabs/go-blockdevice v2 (selected for partition ops)

- Pure Go GPT read/write/grow
- Kernel partition table sync via BLKPG ioctls (replaces partprobe)
- Block device operations (size, sector size, properties)
- Requires Go 1.25+
- CGO_ENABLED=0 compatible
- Used by Talos Linux in production

### diskfs/go-diskfs (not selected)

- Designed for creating disk images and filesystems from scratch
- Supports FAT32, ISO9660, squashfs, ext4 filesystem creation
- Overkill for our use case (we only need partition table manipulation)
- Heavier dependency graph
- Also requires Go 1.25+

## Shell-outs Replaced

| Shell-out | Location | Replacement |
|-----------|----------|-------------|
| `parted dev resizepart num 100%` | modes/disk.go, finalize/grow.go | `diskutil.GPTPartitionGrower` using go-blockdevice GPT |
| `partprobe dev` | modes/disk.go, finalize/grow.go | Handled internally by `gpt.Table.Write()` kernel sync |
| `lsblk -r -o NAME,TYPE` | cliinstall/ask.go | `BlockProber.ListDisks()` (sysfs /sys/block/) |
| `losetup -d /dev/loop0` | modes/disk.go PivotAndExec | `LoopDetacher.DetachPath()` via LOOP_CLR_FD ioctl |
| `hwclock --hctosys --utc` | cli/rc/rc.go | `RTCClockSyncer` using u-root pkg/rtc + Settimeofday |

## Shell-outs Retained (and why)

| Shell-out | Location | Reason |
|-----------|----------|--------|
| `resize2fs` | modes/disk.go, finalize/grow.go | No pure Go ext4 filesystem resize exists |
| `e2fsck` | modes/disk.go | No pure Go ext4 filesystem check exists |
| `chpasswd` | cliinstall/ask.go, command/command.go | PAM/shadow file interaction |
| `nsenter ... reboot` | cli/upgrade/upgrade.go | PID namespace entry for reboot |
| `passwd -d rancher` | modes/live.go | PAM password removal |
| `cp -r` / `cp -f` | modes/disk.go | File copy (could use otiai10/copy but low priority) |

## Shell-outs Eliminated

| Shell-out | Location | Replacement |
|-----------|----------|-------------|
| `mdev -s` | cli/rc/rc.go | `internal/devpopulate.PopulateDev()` — pure Go sysfs walk + blkid probe |
| `parted resizepart` | modes/disk.go | `PartitionGrower.GrowPartition()` — GPT: pure Go via go-blockdevice/v2; MBR: pure Go via rekby/mbr |
| `partprobe` | modes/disk.go | Handled by go-blockdevice/v2 BLKPG ioctls (GPT path) |

## New Interfaces

```go
// PartitionGrower grows a partition to fill available space.
// Supports both GPT (pure Go) and MBR (via sfdisk shell-out).
type PartitionGrower interface {
    GrowPartition(device string, partNum int) error
}

// LoopDetacher abstracts loop device detachment by path.
type LoopDetacher interface {
    DetachPath(device string) error
}
```

## Testing Strategy

- `internal/diskutil/partition_test.go`: Unit tests for GPTPartitionGrower
  with real GPT images created in-memory
- Mock-based tests in modes/ and finalize/ for interface usage
- All existing tests continue to pass unchanged

## Dependencies Added

- `github.com/siderolabs/go-blockdevice/v2 v2.0.6`
  - Transitive: github.com/google/uuid, github.com/siderolabs/gen,
    go.uber.org/zap, golang.org/x/text
