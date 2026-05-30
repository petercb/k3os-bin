# Specification: Adopt Declarative Namespace Pattern for doMounts()

## Overview

Refactor the imperative `doMounts()` function in `internal/cli/rc/rc.go` to
use a declarative data-driven pattern inspired by u-root's `pkg/libinit`
`Creator` interface.

## Motivation

The current `doMounts()` function is ~150 lines of procedural code making
sequential calls to `mount()`, `mkdir()`, `mkchar()`, and `symlink()` helpers.
This pattern:

- Is hard to review (must read every line to understand the namespace)
- Is hard to test (each mount is buried in procedural code)
- Makes it easy to introduce ordering bugs
- Requires modifying code to add/remove mounts

The u-root `pkg/libinit` package demonstrates a cleaner approach: define the
desired namespace as a slice of typed structs, then iterate and apply. This is
data-driven, self-documenting, and trivially testable.

## Current Code (excerpt from rc.go doMounts)

```go
func doMounts() {
    mountSilent("proc", "/proc", "proc", nodev|nosuid|noexec|relatime, "")
    mount("tmpfs", "/run", "tmpfs", nodev|nosuid|noexec|relatime, "size=10%,mode=755")
    mount("tmpfs", "/tmp", "tmpfs", nodev|nosuid|noexec|relatime, "size=10%,mode=1777")
    mkdir("/var/cache", 0o755)
    // ... 100+ more lines
    mount("dev", "/dev", "devtmpfs", nosuid|noexec|relatime, "size=10m,...")
    mkchar("/dev/console", 0o600, 5, 1)
    symlink("/proc/self/fd", "/dev/fd")
    // ... etc
}
```

## Target Pattern (from u-root pkg/libinit)

```go
type Creator interface {
    Create() error
    fmt.Stringer
}

// Types: Dir, Mount, Dev, Symlink, CpDir
var Namespace = []Creator{
    Dir{Name: "/proc", Mode: 0o555},
    Mount{Source: "proc", Target: "/proc", FSType: "proc", Flags: nodev|nosuid|noexec},
    Dir{Name: "/tmp", Mode: 0o1777},
    Mount{Source: "tmpfs", Target: "/tmp", FSType: "tmpfs", Flags: nodev|nosuid|noexec},
    Dev{Name: "/dev/console", Mode: 0o600, Major: 5, Minor: 1},
    Symlink{Target: "/proc/self/fd", NewPath: "/dev/fd"},
    // ...
}
```

## Design

### Do NOT import u-root directly

The pattern is simple enough to implement locally. Importing u-root's
`pkg/libinit` would bring in unwanted transitive dependencies (netlink,
kmodule, etc.).

### New package: `internal/namespace`

```go
package namespace

type Creator interface {
    Create() error
    fmt.Stringer
}

type Dir struct { Name string; Mode os.FileMode }
type Mount struct { Source, Target, FSType string; Flags uintptr; Data string }
type Dev struct { Name string; Mode, Major, Minor uint32 }
type Symlink struct { Target, NewPath string }

func Apply(creators []Creator, logger *slog.Logger) error {
    for _, c := range creators {
        if err := c.Create(); err != nil {
            logger.Warn("namespace: create failed", "item", c, "error", err)
        }
    }
    return nil
}
```

### Refactored rc.go

```go
var rcNamespace = []namespace.Creator{
    namespace.Mount{Source: "proc", Target: "/proc", FSType: "proc", Flags: nodev|nosuid|noexec|relatime},
    namespace.Mount{Source: "tmpfs", Target: "/run", FSType: "tmpfs", Flags: nodev|nosuid|noexec|relatime, Data: "size=10%,mode=755"},
    // ... all mounts/dirs/devs/symlinks declared as data
}

func doMounts() {
    namespace.Apply(rcNamespace, slog.Default())
}
```

## Benefits

- **Readable**: The namespace is a scannable data table
- **Testable**: Tests can inspect the slice directly, or test `Apply` with mock creators
- **Extensible**: Adding a mount is adding a line, not writing procedural code
- **Documentable**: The slice IS the documentation of the namespace

## Acceptance Criteria

- [ ] `internal/namespace` package created with Creator types
- [ ] `doMounts()` refactored to use declarative namespace slice
- [ ] Cgroup mounts included in the declaration
- [ ] Unit tests verify the namespace slice contents
- [ ] `Apply()` tested with both success and error cases
- [ ] Behavior is identical to the imperative version
- [ ] `golangci-lint run ./...` passes
