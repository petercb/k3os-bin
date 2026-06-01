# Implementation Plan: Adopt Declarative Namespace Pattern

**Spec**: `docs/specs/adopt-declarative-namespace-pattern.md`
**Status**: Complete
**Branch**: `feat/declarative-namespace-pattern`

## Summary

Refactored the imperative `doMounts()` function in `internal/cli/rc/rc.go` to
use a declarative data-driven pattern inspired by u-root's `pkg/libinit`
Creator interface, implemented locally without importing u-root.

## Features Implemented

### FEAT-001: Create internal/namespace package

Created the `internal/namespace` package with:

- **Creator interface** - `Create() error` + `fmt.Stringer`
- **Dir** - creates directories via `os.MkdirAll`
- **Mount** - mounts filesystems via `unix.Mount` with `Silent` field for
  error suppression (matching the `mountSilent` vs `mount` distinction)
- **Dev** - creates device nodes via `unix.Mknod`
- **Symlink** - creates symbolic links via `unix.Symlink`
- **Apply()** - iterates creators, logs errors via `slog.Logger`, never stops
  on failure

Files:
- `internal/namespace/namespace.go`
- `internal/namespace/apply.go`
- `internal/namespace/namespace_test.go`

### FEAT-002: Refactor doMounts() to declarative pattern

Refactored `internal/cli/rc/rc.go`:

- Added **Write** type for file-write operations (e.g., cgroup hierarchy)
- Added **CgroupMounts** type encapsulating the dynamic `/proc/cgroups`
  parsing and per-subsystem mount logic
- Declared all namespace operations as a `rcNamespace` slice of `Creator`
- Replaced `doMounts()` body with a single `namespace.Apply()` call
- Removed obsolete helper functions: `mount()`, `mountSilent()`, `mkchar()`,
  `symlink()`, `mkdir()`, `cgroupList()`
- Kept helpers still used by other functions: `write()`, `read()`, `readdir()`,
  `glob()`, `exists()`, `modaliases()`

Files:
- `internal/namespace/write.go`
- `internal/namespace/write_test.go`
- `internal/namespace/cgroup.go`
- `internal/namespace/cgroup_test.go`
- `internal/cli/rc/rc.go` (refactored)
- `internal/cli/rc/rc_test.go` (new)

## Design Decisions

1. **Silent field on Mount**: Rather than a separate type or error strategy,
   the `Silent bool` field on `Mount` mirrors the original code's distinction
   between `mount()` (logs errors) and `mountSilent()` (ignores errors).

2. **CgroupMounts as a Creator**: The dynamic cgroup logic (reading
   `/proc/cgroups`) is encapsulated in a single `Creator` implementation rather
   than trying to flatten it into static data.

3. **Write type**: The `write()` call for `memory.use_hierarchy` is represented
   as a `Write{}` entry in the namespace slice, keeping the declaration purely
   data-driven.

4. **Build tags**: All files use `//go:build linux` since they wrap Linux
   syscalls.

## Verification

- `go test -race -covermode=atomic -failfast ./...` - all tests pass
- `golangci-lint run ./...` - zero issues
- `CGO_ENABLED=0 go build -o /dev/null .` - builds successfully
- Behavior is identical to the imperative version
