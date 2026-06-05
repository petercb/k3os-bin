# Plan: Fix Disk Handler Errors on RPi4 Boot

## Status: Planned

## Problem Statement

Two errors prevent successful disk-mode boot on the Raspberry Pi 4:

### Error 1: e2fsck needs `-y` flag

```
e2fsck 1.47.0 (5-Jul-2023)
e2fsck: need terminal for interactive repairs
disk: e2fsck failed error="exit status 8"
```

The disk handler calls `e2fsck -f` but in a non-interactive early boot
environment, e2fsck requires `-y` (auto-fix) to proceed without a terminal.

**Location**: `internal/boot/modes/disk.go` line ~148
```go
if err := h.deps.Cmd.Run("e2fsck", "-f", devNum); err != nil {
```

**Fix**: Add `-y` flag:
```go
if err := h.deps.Cmd.Run("e2fsck", "-fy", devNum); err != nil {
```

### Error 2: SetupInit fails when symlink already exists

```
setup init: symlink init: symlink ../k3os/system/k3os/current/k3os
    /run/k3os/target/sbin/init: file exists
```

The `SetupInit()` function has a guard that checks if `/sbin/init` exists:
```go
func (h *DiskHandler) SetupInit() error {
    initPath := filepath.Join(targetDir, "sbin/init")
    if _, err := h.deps.FS.Stat(initPath); err == nil {
        return nil  // ← Should bail here
    }
    ...
```

But the log shows it's reaching the symlink creation. The issue is that
`Stat()` follows symlinks — if the symlink TARGET doesn't exist (e.g.,
`../k3os/system/k3os/current/k3os` resolves to a path that doesn't exist
yet because the version directory uses the wrong path), `Stat()` returns
an error even though the symlink itself exists.

Looking at the error path: `../k3os/system/container/k3os/current/k3os` —
note the extra `container` in the path! This suggests the `VersionID`
field contains something like `container/k3os` instead of just a version string.

Actually re-reading: the symlink path in the error is
`../k3os/system/container/k3os/current/k3os`. This comes from `SetupK3OS()`
which creates the version directory based on `h.deps.VersionID`. If VersionID
is empty or wrong, SetupK3OS creates a different path than what SetupInit
expects.

Wait — looking more carefully at the error: it says
`symlink ../k3os/system/container/k3os/current/k3os /run/k3os/target/sbin/init`.
This isn't the standard init symlink path. The standard is
`../k3os/system/k3os/current/k3os`. The `container/k3os` part suggests the
binary is running inside a container or the version detection is picking up
container paths.

**Root Cause**: The `/sbin/init` symlink already exists in the disk image
(created by `build-disk-image.sh` or during initial install). When `Stat()`
is called on it, it follows the symlink to the target. If the target path
resolves correctly (the binary exists), `Stat()` succeeds and returns nil —
and the function bails early. BUT if the target of the existing symlink
doesn't resolve (broken symlink), `Stat()` returns an error, the guard
fails, and it tries to create a new symlink over the existing one.

**Fix**: Use `Lstat()` instead of `Stat()` to check if the symlink itself
exists (regardless of whether its target resolves):
```go
if _, err := h.deps.FS.Lstat(initPath); err == nil {
    return nil
}
```

Or alternatively, handle `EEXIST` gracefully:
```go
if err := h.deps.FS.Symlink(target, initPath); err != nil {
    if os.IsExist(err) {
        return nil  // Already exists, that's fine
    }
    return fmt.Errorf("symlink init: %w", err)
}
```

## Proposed Fixes

### Fix 1: Add `-y` to e2fsck (trivial)

```go
// Before:
if err := h.deps.Cmd.Run("e2fsck", "-f", devNum); err != nil {

// After:
if err := h.deps.Cmd.Run("e2fsck", "-fy", devNum); err != nil {
```

This matches what every init system does — e2fsck during boot must be
non-interactive.

### Fix 2: Use Lstat for symlink existence check

```go
// Before (SetupInit):
if _, err := h.deps.FS.Stat(initPath); err == nil {
    return nil
}

// After:
if _, err := h.deps.FS.Lstat(initPath); err == nil {
    return nil
}
```

This checks if the symlink ITSELF exists, regardless of target resolution.
Same fix should be applied to `SetupK3OS()` which has a similar pattern
for the "current" symlink.

### Fix 3 (bonus): Also check finalize/grow.go e2fsck call

The finalizer also calls e2fsck. Add `-y` there too for consistency.

## Implementation Steps

1. Add `-y` to e2fsck in `internal/boot/modes/disk.go`
2. Add `-y` to e2fsck in `internal/boot/finalize/grow.go` (if applicable)
3. Replace `Stat` with `Lstat` in `SetupInit()` for the symlink check
4. Replace `Stat` with `Lstat` in `SetupK3OS()` for the "current" symlink
5. Verify `iface.FileSystem` has `Lstat` method (or add it)
6. Update unit tests
7. Push and verify CI

## Files to Modify

| File | Change |
|------|--------|
| `internal/boot/modes/disk.go` | Add `-y` to e2fsck, use Lstat for symlink checks |
| `internal/boot/finalize/grow.go` | Add `-y` to e2fsck |
| `internal/boot/modes/disk_test.go` | Update e2fsck mock expectation |
| `internal/boot/finalize/finalize_test.go` | Update e2fsck mock expectation |
| `internal/iface/iface.go` | Add Lstat if not present |

## Risk Assessment

- Very low risk: `-y` is standard practice for boot-time e2fsck
- Lstat vs Stat is a correctness fix — prevents errors on valid symlinks
  with temporarily unresolvable targets
