# Plan: Fix QEMU Disk Integration Test

## Status: In Progress (PR #58)

## Problem Statement

The QEMU disk integration test (`qemu-integration-disk`) is not properly testing
disk boot mode. Despite having a K3OS_STATE-labeled virtio disk attached, the
mode detector falls through to `"local"` mode because `FindByLabel("K3OS_STATE")`
fails — the `/dev/disk/by-label/K3OS_STATE` symlink is never created by
`devpopulate.PopulateDev()`.

The test then reports `passed: true` because the verifier doesn't catch mode
handler failures for non-disk modes.

## Root Cause Analysis

### Why the symlink isn't created

`devpopulate.PopulateDev()` runs during `doHotplug()` as part of `rc.Run()`.
It walks `/sys/class/block`, finds device entries, and calls
`blkid.ProbePath("/dev/vda")` to detect the filesystem label.

Likely failure points:

1. **devtmpfs timing**: The `doMounts()` mounts devtmpfs on `/dev`, but the
   virtio disk driver may not have finished enumeration yet. If `/dev/vda`
   doesn't exist when devpopulate runs, the probe is skipped.

2. **blkid probe failure on raw disk**: The test disk is a raw file formatted
   directly as ext4 (no partition table). The `go-blockdevice/v2/blkid` package
   may not handle whole-disk ext4 filesystems correctly (it might expect a
   partition table on whole-disk devices where `WholeDisk=true`).

3. **Module loading order**: The virtio-blk driver may need to be loaded before
   the disk appears in sysfs. The `modaliases()` call runs AFTER devpopulate,
   but the device needs to exist first.

### Why the test reports success

The verifier's `mode_execution` check soft-passes for `"local"` mode (which is
correct — the test wasn't designed to validate local mode execution). The overall
test result is `passed: true` even though disk mode was never entered.

## Proposed Fixes

### Phase 1: Make devpopulate work with the test disk (Critical)

**Option A**: Load virtio-blk module before devpopulate runs.
Add `modaliases(glob("/sys/bus/*/devices/*/modalias")...)` BEFORE the
`devpopulate.PopulateDev()` call in `doHotplug()`. This ensures the virtio-blk
driver is loaded, the device appears in `/sys/class/block/`, and devtmpfs
creates `/dev/vda` — all before we try to probe it.

**Option B**: Add retry/wait logic to devpopulate for device nodes to settle.
After `doMounts()` mounts devtmpfs and sysfs, give the kernel a moment to
finish device enumeration. This could be a short sleep or a wait-for-settle
approach.

**Option C**: Include kernel modules (virtio-blk) in the test initramfs.
Download and extract the k3os-modules tarball so `modaliases()` can actually
load the virtio_blk driver.

### Phase 2: Fix the non-disk test's local mode handler (Low priority)

The `"local"` mode handler fails on `/etc/ssh` not existing. Fix:
- Add `mkdir -p "${WORK_DIR}/etc/ssh"` in `build-initramfs.sh`
- This is a band-aid; the real fix is Phase 3.

### Phase 3: Improve test verifier robustness (Medium priority)

1. **Add serial log error scanning**: After QEMU exits, scan the serial log for
   `level=ERROR` entries and fail the test if unexpected errors are found.
   Currently the test only checks the structured JSON results.

2. **Validate mode matches expected**: The disk test should verify that the
   detected mode is `"disk"`, not just that "a valid mode was detected".
   Add a `expected_mode` field that tests can set via kernel cmdline
   (e.g., `k3os.test_expected_mode=disk`).

3. **Make mode_execution checks mode-aware**: The verifier should FAIL (not
   soft-pass) if the detected mode doesn't match what the test expects.

### Phase 4: Include kernel modules in test initramfs (Nice to have)

Download `k3os-modules-amd64.tar.gz` from the kernel release and extract it
into the test initramfs at `/lib/modules/<version>/`. This eliminates the
"module aliases not found" warning and ensures `modaliases()` can actually
load kernel modules (including virtio_blk, ext4, etc.).

## Implementation Order

1. Phase 1 Option A (reorder modaliases before devpopulate) — simplest fix
2. Phase 3.1 (serial log error scanning) — prevents false positives
3. Phase 3.2 (expected mode validation) — ensures disk test validates disk mode
4. Phase 2 (/etc/ssh) — trivial fix
5. Phase 4 (kernel modules) — if Phase 1 Option A alone isn't sufficient

## Files to Modify

| File | Change |
|------|--------|
| `internal/cli/rc/rc.go` | Move `modaliases()` before `devpopulate.PopulateDev()` |
| `integration/qemu/build-initramfs.sh` | Add `/etc/ssh` dir, optionally extract modules |
| `integration/qemu/run-qemu-disk.sh` | Add serial log error scanning |
| `integration/qemu/run-qemu.sh` | Add serial log error scanning |
| `internal/boot/testmode/testmode.go` | Add expected mode validation |
| `integration/qemu/download-kernel.sh` | Add modules tarball to download list |

## Risk Assessment

- Phase 1 Option A is low risk — moving modaliases() before devpopulate is the
  correct logical order (load drivers first, then probe devices).
- Phase 3 changes only affect test infrastructure, not production code.
- Phase 4 increases CI cache size but improves test fidelity.
