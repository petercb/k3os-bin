# Specification: Replace internal/loopdev with u-root pkg/mount/loop

## Overview

Replace the custom `internal/loopdev` package with the well-tested
`github.com/u-root/u-root/pkg/mount/loop` package for loop device management.

## Motivation

The `internal/loopdev` package (~200 lines) implements loop device operations
(find free device, attach file, detach, set autoclear). The u-root
`pkg/mount/loop` package provides equivalent functionality with:

- Battle-tested code used in LinuxBoot firmware across production systems
- Better error handling and edge case coverage
- Integration with u-root's mount abstraction
- Active maintenance and community support
- Reduced maintenance burden for this project

## Current internal/loopdev API

```go
type Attacher interface {
    Attach(file string, offset uint64, readOnly bool) (LoopDevice, error)
}

type LoopDevice interface {
    Path() string
    Detach() error
    SetAutoclear() error
}
```

## u-root pkg/mount/loop API

```go
// New allocates a loop device and attaches source to it.
func New(source, fstype string, data string) (*Loop, error)

// FindDevice finds a free /dev/loopN device.
func FindDevice() (string, error)

// SetFile attaches a file to a loop device.
func SetFile(devicename, file string) error

// ClearFile detaches a file from a loop device.
func ClearFile(devicename string) error

// Loop.Mount mounts the loop device at path.
func (l *Loop) Mount(path string, flags uintptr, opts ...func() error) (*mount.MountPoint, error)

// Loop.Free frees the loop device.
func (l *Loop) Free() error
```

## Design

### Adapter approach

Create an adapter that wraps u-root's loop package behind our existing
`iface.LoopAttacher` / `iface.LoopDevice` interfaces:

```go
package loopdev

import "github.com/u-root/u-root/pkg/mount/loop"

type urootAttacher struct{}

func (a *urootAttacher) Attach(file string, offset uint64, readOnly bool) (iface.LoopDevice, error) {
    devName, err := loop.FindDevice()
    if err != nil {
        return nil, err
    }
    if err := loop.SetFileWithOffset(devName, file, offset, readOnly); err != nil {
        return nil, err
    }
    return &urootDevice{path: devName}, nil
}

type urootDevice struct{ path string }

func (d *urootDevice) Path() string    { return d.path }
func (d *urootDevice) Detach() error   { return loop.ClearFile(d.path) }
func (d *urootDevice) SetAutoclear() error { /* set LO_FLAGS_AUTOCLEAR via ioctl */ }
```

### Offset support consideration

The current `enterchroot` code uses loop devices with an **offset** (to mount
the squashfs appended after `_SQMAGIC_` in the binary). u-root's `loop.New()`
does not currently support offsets. Options:

1. **Contribute offset support upstream** to u-root (preferred)
2. **Use `SetFile` with offset ioctl directly** (the low-level `LOOP_SET_FD` +
   `LOOP_SET_STATUS64` ioctls support `lo_offset`)
3. **Keep a thin wrapper** that uses u-root for device finding but handles
   offset attachment locally

### Migration steps

1. Evaluate if u-root's loop package supports offset (check latest source)
2. If yes: replace `internal/loopdev` with a thin adapter
3. If no: contribute offset support or keep hybrid approach
4. Update `enterchroot` to use the new adapter
5. Remove `internal/loopdev` package
6. Update `iface.LoopAttacher` interface if needed

## Dependencies Added

- `github.com/u-root/u-root/pkg/mount/loop`
- Transitive: `github.com/u-root/u-root/pkg/mount` (mount abstraction)

## Risk: Offset Support

The `_SQMAGIC_` flow requires mounting a squashfs at a byte offset within the
binary. This is a critical path — if u-root's loop package doesn't support
offsets, we must either:
- Extend it (PR upstream)
- Keep a local offset-aware wrapper that delegates to u-root for device allocation

## Acceptance Criteria

- [ ] Loop device operations use u-root's package (directly or via adapter)
- [ ] Offset-based attachment still works for squashfs-in-binary flow
- [ ] `internal/loopdev` package removed (or reduced to thin wrapper)
- [ ] All existing tests pass
- [ ] `enterchroot` tests verify loop device behavior
- [ ] `golangci-lint run ./...` passes
- [ ] No regression in initrd boot sequence
