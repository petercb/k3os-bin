# Replace loopdev with u-root loop package

## Summary

Migrate the `internal/loopdev` package from fully manual ioctl-based loop
device management to a hybrid approach that delegates device discovery to the
`github.com/u-root/u-root/pkg/mount/loop` package while retaining local ioctl
calls for offset-based attachment.

## Motivation

The u-root `pkg/mount/loop` package provides a well-tested implementation of
loop device discovery (`FindDevice`) that handles opening `/dev/loop-control`
and calling `LOOP_CTL_GET_FREE`. Using it reduces our maintenance surface for
device discovery logic.

However, u-root does **not** support setting offsets via `LOOP_SET_STATUS64`,
which we require for attaching images at non-zero offsets. Therefore we adopt a
hybrid approach.

## Design

### Hybrid architecture

1. **Device discovery** - delegated to u-root via an injectable
   `deviceFinder` interface. The production implementation wraps
   `loop.FindDevice()`.
2. **Attachment with offset** - remains local using the existing `syscaller`
   interface for `LOOP_SET_FD` and `LOOP_SET_STATUS64` ioctls.
3. **Detach** - remains local via `syscaller.IoctlSetInt` (LOOP_CLR_FD) to
   preserve mock-injection testability.

### Interface changes

- Add `deviceFinder` interface with `FindDevice() (string, error)` method.
- Remove `IoctlRetInt` from the `syscaller` interface (no longer needed since
  `LOOP_CTL_GET_FREE` is handled by u-root internally).
- Remove `loopCtlGetFree` and `loopControlPath` constants.

### Testability

The `deviceFinder` interface allows tests to inject a mock that returns
deterministic loop device paths without touching `/dev/loop-control`. All other
ioctl interactions remain mockable via the `syscaller` interface.

## Dependencies

- `github.com/u-root/u-root v0.16.0`
