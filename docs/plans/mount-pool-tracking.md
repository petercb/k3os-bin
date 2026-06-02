# Mount Pool Tracking

## Summary

Introduce a `mount.Pool` type that records every filesystem mounted during
early boot, enabling ordered reverse teardown for graceful shutdown.

## Motivation

The k3OS init binary mounts dozens of filesystems during the rc (run-control)
phase. Currently these mounts are fire-and-forget with no way to cleanly undo
them. A future supervisor/shutdown hook needs to unmount filesystems in the
exact reverse of the order they were mounted to avoid busy-device errors and
data loss.

## Design

### mount.Point and mount.Pool

`internal/mount/pool.go` provides two types:

- **Point** -- records the parameters of a completed mount (source, target,
  fstype, flags, data).
- **Pool** -- a thread-safe ordered collection of Points with an
  `UnmountAll(flags int) error` method that iterates in reverse, collecting
  any errors with `errors.Join`.

Pool accepts an `UnmountFunc` at construction time, keeping the type portable
and testable without requiring linux syscalls in unit tests.

### Trackable interface

`internal/namespace/apply_tracked.go` defines:

```go
type Trackable interface {
    AsMountPoint() *mount.Point
}
```

The existing `namespace.Mount` struct implements `Trackable`, returning a Point
derived from its fields.

### ApplyTracked

`ApplyTracked` mirrors `Apply` (iterate creators, log errors, never abort) but
additionally records successful Trackable creators into the provided Pool. When
pool is nil it behaves identically to Apply.

### Boot wiring

`internal/cli/rc/rc.go` exports `MountPool`, a package-level `*mount.Pool`
initialized with the real `mount.Unmount` syscall wrapper. `doMounts()` calls
`namespace.ApplyTracked` so every successful mount during rc is recorded.

## Shutdown usage (future)

```go
import "github.com/petercb/k3os-bin/internal/cli/rc"

// During shutdown:
if err := rc.MountPool.UnmountAll(0); err != nil {
    log.Printf("unmount errors: %v", err)
}
```

## Interface additions

Two new interfaces in `internal/iface/iface.go`:

- **Unmounter** -- `Unmount(target string, flags int) error`
- **TrackedMounter** -- composes Mounter + Unmounter

`LinuxMounter` in `internal/iface/osimpl/` satisfies TrackedMounter via a new
`Unmount` method that delegates to `mount.Unmount`.

## Testing

- `internal/mount/pool_test.go` -- portable unit tests covering add, reverse
  unmount order, flag passthrough, error collection, and copy semantics.
- `internal/namespace/apply_tracked_test.go` -- linux-only tests verifying pool
  recording for Trackable creators, skipping non-Trackable creators, and nil
  pool safety.
