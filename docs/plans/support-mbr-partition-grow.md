# Plan: Support MBR Partition Growing

## Status: Complete

## Problem Statement

The `GPTPartitionGrower` only supports GPT partition tables. The Raspberry Pi 4
image uses MBR (DOS) partition tables created with `sfdisk --label dos`. When
the disk handler encounters the `growpart` marker on first boot, it calls
`GrowPartition("/dev/mmcblk1", 2)` which tries to read a GPT header. Since the
disk has an MBR, the GPT read returns "zeroed out header" and the partition is
never grown.

The current code handles the error gracefully (logs a warning, skips resize),
but the partition remains at its original size — which defeats the purpose of
the growpart marker (expand root to fill the SD card on first boot).

## Root Cause

```go
// internal/diskutil/partition.go
func (g *GPTPartitionGrower) GrowPartition(device string, partNum int) error {
    // ...
    table, err := gpt.Read(gptDev)  // ← fails on MBR disks
    if err != nil {
        return fmt.Errorf("read GPT from %s: %w", device, err)
    }
    // ...
}
```

The `go-blockdevice/v2` library only provides GPT manipulation. There is no
MBR partition growing support in the library.

## Context: RPi4 Disk Layout

Created by `petercb/k3os` Dockerfile:
```
sfdisk --label dos "${FINAL_IMG}"
  Partition 1: FAT32 boot (K3OS_GRUB, ~10MB)
  Partition 2: ext4 root (K3OS_STATE, sized to fit content)
```

The growpart marker contains `/dev/xxx 99` (placeholder). The disk handler
resolves the actual device via `FindByLabel("K3OS_STATE")` → `/dev/mmcblk1p2`.

## Proposed Solution

### Option A: Shell out to `growpart` / `sfdisk` for MBR (Recommended)

Detect the partition table type before attempting to grow. If GPT, use the
existing pure Go path. If MBR, fall back to shelling out to `sfdisk` (which
is available on the RPi4 image and handles MBR natively):

```go
func (g *PartitionGrower) GrowPartition(device string, partNum int) error {
    tableType := probePartitionTableType(device)
    switch tableType {
    case "gpt":
        return g.growGPT(device, partNum)
    case "dos":
        return g.growMBR(device, partNum)
    default:
        return fmt.Errorf("unsupported partition table type: %s", tableType)
    }
}
```

For MBR growing, use `sfdisk` resize command:
```bash
echo ", +" | sfdisk --no-reread -N <partNum> <device>
```

This tells sfdisk to resize partition N to fill all remaining space.

**Pros**: Simple, proven, `sfdisk` is already installed on the RPi4 image.
**Cons**: Adds a shell-out for MBR (but GPT remains pure Go).

### Option B: Pure Go MBR manipulation

Implement MBR partition table reading/writing in Go. The MBR format is simple
(512-byte header with 4 partition entries at offset 446), but correctly handling
extended partitions, CHS alignment, and writing back is more complex than GPT.

**Pros**: No shell-out.
**Cons**: More code to maintain, MBR is legacy (RPi5 already uses GPT/UEFI),
error-prone edge cases (CHS vs LBA, extended partitions).

### Option C: Use `go-diskfs` library

The `github.com/diskfs/go-diskfs` library supports both MBR and GPT partition
tables. However, this would add a new dependency to the project.

**Pros**: Battle-tested, supports both schemes.
**Cons**: New dependency, potentially large transitive dep tree.

## Recommended Approach: Option A

1. **Detect partition table type**: Read the first 512 bytes of the device.
   Check for GPT protective MBR (byte 450 = 0xEE) or standard MBR (bytes
   510-511 = 0x55AA without 0xEE partition type). Use `blkid.ProbePath()` or
   raw byte reading.

2. **Rename interface implementation**: Rename `GPTPartitionGrower` to
   `PartitionGrower` (it will now handle both schemes).

3. **GPT path**: Keep existing pure Go implementation unchanged.

4. **MBR path**: Shell out to `sfdisk --no-reread -N <partNum> <device>` with
   input `", +"` (grow to fill available space). After sfdisk, call
   `blockdev --rereadpt <device>` or use `BLKRRPART` ioctl to inform the
   kernel of the new partition layout.

5. **Interface remains unchanged**: `PartitionGrower.GrowPartition(device, partNum)`
   — callers don't need to know about the underlying table type.

## Implementation Tasks

1. Add `probePartitionTableType(device string) string` function that returns
   "gpt", "dos", or "unknown" by examining the device's partition table
   signature.

2. Add `growMBR(device string, partNum int) error` that shells out to sfdisk.

3. Rename `GPTPartitionGrower` to `PartitionGrower` and add the dispatch logic.

4. Add unit tests for type detection (mock with file images).

5. Add integration test using a loop device with MBR (similar to existing
   GPT integration test in `partition_test.go`).

6. Update `docs/plans/replace-disk-shellouts-with-go-blockdevice.md` to note
   that MBR growing still uses sfdisk as a shell-out.

## Files to Modify

| File | Change |
|------|--------|
| `internal/diskutil/partition.go` | Add table type detection, MBR grow path |
| `internal/diskutil/partition_test.go` | Add MBR tests |
| `internal/iface/osimpl/block.go` | Add `PartitionGrower` reference (if renaming) |
| `docs/plans/replace-disk-shellouts-with-go-blockdevice.md` | Update retained shell-outs |
| `docs/plans/support-mbr-partition-grow.md` | This file |

## Risk Assessment

- Low risk: MBR path only activates for non-GPT disks (RPi4), GPT path unchanged.
- `sfdisk` is available on all k3os images that use MBR (it's installed during build).
- Fallback behavior on failure: partition stays at original size (same as current).

## Testing Strategy

- Unit test: Create a small MBR disk image file, verify type detection returns "dos".
- Integration test: Create a loop device with MBR, small partition 2, grow it, verify
  new size fills available space.
- CI: The QEMU integration tests use raw ext4 (no partition table) so they won't
  exercise this code path. A separate RPi4-style integration test would be ideal
  but is out of scope for the initial fix.
