# Plan: QEMU Integration Test for MBR Partition Growing

## Status: Planned

## Context

PR #61 added pure Go MBR partition growing support. The unit tests verify the
`growMBR()` function works correctly with disk image files, but there is no
end-to-end integration test that exercises the full flow:

1. Boot k3os with an MBR-partitioned disk
2. Disk handler detects K3OS_STATE partition
3. Growpart marker triggers partition expansion
4. `PartitionGrower.GrowPartition()` detects MBR and uses `rekby/mbr`
5. Partition expands to fill available space
6. `e2fsck` + `resize2fs` resize the filesystem
7. Boot continues normally

## Goal

Add a `qemu-integration-disk-mbr` CI job that mirrors the existing
`qemu-integration-disk` job but with an MBR-partitioned disk image.
The test validates that partition growing works on MBR disks (the RPi4 use case).

## Design

### Disk Image: `build-disk-image-mbr.sh`

Creates an MBR-partitioned disk image similar to the RPi4 layout:

```
+--------+------------------+-------------------+
| MBR    | Partition 1      | Free space        |
| 512B   | ext4 K3OS_STATE  | (for grow test)   |
|        | ~30MB            | ~98MB             |
+--------+------------------+-------------------+
Total: 128MB
```

Key differences from the existing `build-disk-image.sh`:
- Uses `sfdisk --label dos` to create an MBR partition table
- Partition 1 starts at sector 2048, sized to ~30MB (leaves ~98MB free)
- Includes a `k3os/system/growpart` marker file: `/dev/xxx 1`
- The marker's dummy device `/dev/xxx` triggers the BlockProber fallback path

### Test Runner: `run-qemu-disk-mbr.sh`

Nearly identical to `run-qemu-disk.sh` with:
- Different disk image path (`.cache/test-state-disk-mbr.qcow2`)
- Different serial log path (`.cache/serial-output-disk-mbr.log`)
- Same kernel cmdline: `k3os.test_mode k3os.test_expected_mode=local k3os.debug`
- Same verifier checks (pivot_root existence proves disk handler succeeded)

### Verifier Enhancements

Add a new check to the test mode verifier when a growpart marker was present:
- `checkPartitionGrown`: After pivot_root, verify the root filesystem size
  is larger than the original partition size. Use `Statfs` to check available
  space indicates the partition was expanded.

Alternatively, check via the serial log for:
- `disk: detected partition table` log entry confirming MBR detection
- Absence of `disk: partition grow failed` error

### CI Job: `qemu-integration-disk-mbr`

```yaml
qemu-integration-disk-mbr:
  machine:
    image: ubuntu-2404:current
  resource_class: large
  steps:
    - checkout
    - run: Install QEMU, Go, sfdisk
    - run: make build
    - run: make qemu-integration-disk-mbr
    - store_artifacts: serial-output-disk-mbr.log
```

### Makefile Targets

```makefile
qemu-disk-image-mbr: build
	integration/qemu/build-disk-image-mbr.sh

qemu-integration-disk-mbr: qemu-build-initramfs qemu-disk-image-mbr
	integration/qemu/run-qemu-disk-mbr.sh
```

## Implementation Tasks

### 1. Create `build-disk-image-mbr.sh`

- Create a 128MB raw disk image
- Use `sfdisk --label dos` to write MBR with one Linux (0x83) partition:
  - Start: sector 2048
  - Size: ~60000 sectors (~30MB) — leaves room to grow
- Format the partition as ext4 with label K3OS_STATE using `mkfs.ext4`
- Mount the partition (via loop device with offset) and populate:
  - Same contents as `build-disk-image.sh` (k3os binary, /etc, /usr/etc, etc.)
  - Add `k3os/system/growpart` containing `/dev/xxx 1`
- Convert to qcow2

### 2. Create `run-qemu-disk-mbr.sh`

- Copy of `run-qemu-disk.sh` adapted for:
  - MBR disk image path
  - MBR serial log path
  - Additional validation: grep serial log for "detected partition table"
    with type "dos" — proves the MBR detection code was exercised

### 3. Add serial log validation for grow

After QEMU exits, check the serial log for:
- `diskutil: detected partition table.*type=dos` — MBR was detected
- NOT `disk: partition grow failed` — grow succeeded
- `disk: probed filesystem.*fstype=ext4` — mount with explicit type

### 4. Update Makefile and CI config

- Add `qemu-disk-image-mbr` and `qemu-integration-disk-mbr` targets
- Add `qemu-integration-disk-mbr` job to `.circleci/config.yml`
- Add to workflow `requires` for `release` job

### 5. Optional: Verifier `checkPartitionGrown`

Add a filesystem size check to the verifier:
```go
func (v *Verifier) checkPartitionGrown() Check {
    // Read /proc/cmdline for k3os.test_expect_grown flag
    // If present, check filesystem size via Statfs
    // Root FS should be > 50MB (original was ~30MB)
}
```

This is more robust than serial log grepping but adds complexity.
Recommend as a follow-up if serial log checking proves insufficient.

## Files to Create/Modify

| Action | File |
|--------|------|
| Create | `integration/qemu/build-disk-image-mbr.sh` |
| Create | `integration/qemu/run-qemu-disk-mbr.sh` |
| Modify | `Makefile` — add `qemu-disk-image-mbr` and `qemu-integration-disk-mbr` targets |
| Modify | `.circleci/config.yml` — add `qemu-integration-disk-mbr` job |
| Modify | `integration/qemu/README.md` — document new test |

## Dependencies

- `sfdisk` — needed in CI for creating MBR partition table (already available
  on `ubuntu-2404:current` machine images, also used by existing e2fsprogs)
- `losetup` — needed for mounting partition within raw image (with offset)
- `e2fsprogs` — needed for `mkfs.ext4` (already installed in disk test job)

## Risk Assessment

- Low risk: This is a new CI job that doesn't affect existing tests
- The MBR disk image creation requires `sfdisk` and `losetup`, both standard
  Linux utilities available in the CI machine image
- The test validates a real-world scenario (RPi4 first boot) that was previously
  untested in CI

## Success Criteria

1. CI job passes: the MBR disk image is created, QEMU boots successfully,
   disk mode is detected, growpart triggers MBR partition expansion
2. Serial log shows "detected partition table" with type "dos"
3. Serial log does NOT show "partition grow failed"
4. Structured verifier reports 8/8 checks passed (including pivot_root)
5. The partition is actually larger after grow (validated via serial log or
   verifier check)

## Estimated Effort

- `build-disk-image-mbr.sh`: ~60 lines (similar to existing script + sfdisk)
- `run-qemu-disk-mbr.sh`: ~20 line diff from `run-qemu-disk.sh`
- CI config changes: ~30 lines
- Total: ~120 lines of new code, 1-2 hours implementation
