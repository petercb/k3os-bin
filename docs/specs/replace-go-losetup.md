# Replace go-losetup with internal/loopdev

## Status

Completed

## Context

The project previously used `github.com/freddierice/go-losetup/v2` (v2.0.1) for
Linux loop device management. This external dependency is unmaintained and adds
supply-chain risk for a security-critical boot component.

## Decision

Replace the external dependency with a thin internal package (`internal/loopdev`)
that wraps Linux loop device ioctls directly via `golang.org/x/sys/unix`.

## Rationale

- **KISS**: The actual ioctl surface used is small (5 commands). A focused
  internal wrapper is simpler to maintain than tracking an external package.
- **No new dependencies**: `golang.org/x/sys/unix` is already a project
  dependency used elsewhere. This change adds zero new external modules.
- **Unmaintained upstream**: The go-losetup repository has no recent activity
  and no guarantee of compatibility with future Go releases.
- **Testability**: The new package uses an internal `syscaller` interface,
  allowing unit tests to verify logic flow without requiring root or real loop
  devices.

## Interface Design

Two interfaces are defined in `internal/iface/iface.go`:

```go
type LoopDevice interface {
    Path() string
    Detach() error
    SetAutoclear() error
}

type LoopAttacher interface {
    Attach(backingFile string, offset uint64, readOnly bool) (LoopDevice, error)
}
```

The `SetAutoclear` method replaces the previous `GetInfo`/`SetInfo` pattern,
encapsulating the flag manipulation internally.

## Implementation

- `internal/loopdev/loopdev.go` contains the production implementation using
  direct ioctl syscalls via `unix.Syscall`.
- `internal/loopdev/loopdev_test.go` uses a mock `syscaller` to test all logic
  paths without root privileges.
- `internal/enterchroot/enter.go` uses a package-level `loopAttacher` variable
  (defaulting to `loopdev.NewAttacher()`) that tests can override.

## Ioctl Commands Used

| Constant           | Value  | Purpose                          |
|--------------------|--------|----------------------------------|
| LOOP_SET_FD        | 0x4C00 | Associate backing file with loop |
| LOOP_CLR_FD        | 0x4C01 | Detach backing file              |
| LOOP_SET_STATUS64  | 0x4C04 | Set loop device parameters       |
| LOOP_GET_STATUS64  | 0x4C05 | Read loop device parameters      |
| LOOP_CTL_GET_FREE  | 0x4C82 | Find next free loop device       |
