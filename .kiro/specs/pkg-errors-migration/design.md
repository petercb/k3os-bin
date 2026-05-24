# Design Document

## Overview

This document describes the technical design for migrating `github.com/pkg/errors` to Go standard library error handling (`fmt.Errorf` + `%w` and `errors.New`). The migration is mechanical but requires careful attention to error chain preservation and test coverage for the affected code paths.

## Architecture

The migration touches two packages in the `internal/` tree:

```
internal/
├── enterchroot/
│   ├── enter.go         (12 call sites: Wrap, Wrapf, New)
│   └── ensureloop.go    (2 call sites: Wrapf)
└── util/
    └── prompt.go        (3 call sites: Wrapf, New)
```

No new packages or interfaces are introduced. The change is purely a dependency swap at the call-site level, preserving identical runtime behavior (error messages and chain traversal).

## Components and Interfaces

### Migration Transforms

Three mechanical patterns are applied:

| Original | Replacement |
|----------|-------------|
| `errors.Wrap(err, msg)` | `fmt.Errorf("%s: %w", msg, err)` |
| `errors.Wrapf(err, fmt, args...)` | `fmt.Errorf(fmt + ": %w", args..., err)` |
| `errors.New(msg)` | `errors.New(msg)` (stdlib `errors` package) |

### Import Changes

Each migrated file replaces:
```go
"github.com/pkg/errors"
```
with (as needed):
```go
"errors"
"fmt"
```

The `fmt` import already exists in all three files. The `errors` import is only needed in files that use `errors.New` (`enter.go` and `prompt.go`).

## Detailed Design

### `internal/util/prompt.go` Migration

**Current call sites (3):**

1. `PromptPassword` — `errors.Wrapf(err, "failed to set password")`
2. `PromptPassword` — `errors.Wrapf(err, "failed to confirm password")`
3. `MaskPassword` — `errors.New("interrupted")`

**After migration:**

```go
// PromptPassword wrapping
fmt.Errorf("failed to set password: %w", err)
fmt.Errorf("failed to confirm password: %w", err)

// MaskPassword sentinel
errors.New("interrupted")
```

Import block changes from `"github.com/pkg/errors"` to `"errors"`. The `"fmt"` import is already present.

### `internal/enterchroot/enter.go` Migration

**Current call sites (12):**

| Function | Pattern | Context Message |
|----------|---------|-----------------|
| `Mount` | `Wrap` | `"creating loopback device"` |
| `Mount` | `Wrapf` | `"failed to exec enter-root"` |
| `run` | `Wrapf` | `"checking %s mounted"` |
| `run` | `Wrapf` | `"mkdir %s"` |
| `run` | `Wrapf` | `"remounting data %s"` |
| `run` | `New` | `"failed to bind mount"` |
| `run` | `Wrap` | `"mounting squashfs"` |
| `run` | `Wrap` | `squashErr.Error()` (chained) |
| `run` | `Wrapf` | `"failed to symlink %s"` |
| `run` | `Wrap` | `"pivot_root failed"` |
| `run` | `Wrapf` | `"making . private %s"` |
| `run` | `Wrap` | `"failed to find /usr/init"` |

**After migration (representative examples):**

```go
// Wrap → fmt.Errorf
fmt.Errorf("creating loopback device: %w", err)
fmt.Errorf("pivot_root failed: %w", err)

// Wrapf → fmt.Errorf (args before err)
fmt.Errorf("checking %s mounted: %w", data, err)
fmt.Errorf("failed to symlink %s: %w", p, err)

// New → errors.New
errors.New("failed to bind mount")
```

**Special case — chained wrapping in squashfs mount:**

```go
// Current:
//   err = errors.Wrap(err, "mounting squashfs")
//   if squashErr != nil {
//       err = errors.Wrap(err, squashErr.Error())
//   }

// After:
err = fmt.Errorf("mounting squashfs: %w", err)
if squashErr != nil {
    err = fmt.Errorf("%s: %w", squashErr.Error(), err)
}
```

**`checkSquashfs` function:**

```go
// Current:
return errors.New("This kernel does not support squashfs...")

// After:
return errors.New("This kernel does not support squashfs...")
```

Import block adds `"errors"` and removes `"github.com/pkg/errors"`.

### `internal/enterchroot/ensureloop.go` Migration

**Current call sites (2):**

```go
errors.Wrapf(err, "failed to mount proc")
errors.Wrapf(err, "failed to mount dev")
```

**After migration:**

```go
fmt.Errorf("failed to mount proc: %w", err)
fmt.Errorf("failed to mount dev: %w", err)
```

Import block removes `"github.com/pkg/errors"`. The `"fmt"` import is already present.

### `go.mod` Cleanup

After all source migrations, run:
```bash
go mod tidy
```

This removes `github.com/pkg/errors v0.9.1` from `go.mod` and its checksum from `go.sum`, provided no transitive dependency still requires it.

## Testing Strategy

### `internal/util` Tests

Tests target `MaskPassword` since it contains all three error paths and accepts injected `io.Reader`/`io.Writer` parameters (testable without OS interaction).

**Test file:** `internal/util/prompt_test.go`

```go
package util

import (
    "errors"
    "io"
    "os"
    "strings"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)
```

**Test cases:**

1. **I/O error propagation** — Create a pipe, close the read end, call `MaskPassword`. Verify the returned error wraps the pipe error via `errors.Is`.

2. **Ctrl+C interrupt** — Write byte `0x03` to a pipe, call `MaskPassword`. Verify error message is `"interrupted"`.

3. **Max-length exceeded** — Write 513+ printable bytes (followed by a newline to avoid blocking) to a pipe, call `MaskPassword`. Verify error message contains `"maximum password length"`.

**Approach for file descriptor:** `MaskPassword` takes `*os.File`. Tests use `os.Pipe()` to create a real file descriptor that bypasses the `term.IsTerminal` check (pipes are not terminals).

### `internal/enterchroot` Tests

Tests target the pure helper functions that don't require root, Linux mounts, or chroot.

**Test file:** `internal/enterchroot/enter_test.go` (with `//go:build linux` tag since the source files are Linux-only)

**Testable functions:**

1. **`inProcFS`** — Reads `/proc/filesystems`. To make this testable, extract the file path into a package-level variable:

```go
// enter.go
var procFilesystemsPath = "/proc/filesystems"

func inProcFS() bool {
    bytes, err := os.ReadFile(procFilesystemsPath)
    // ...
}
```

Tests override `procFilesystemsPath` to point to a temp file with controlled content.

2. **`checkSquashfs`** — Calls `inProcFS` internally. With the refactored path variable, tests verify:
   - When temp file contains `"squashfs"`: `checkSquashfs()` returns `nil`
   - When temp file does NOT contain `"squashfs"`: `checkSquashfs()` returns an error containing `"squashfs"`

**Test structure:**

```go
//go:build linux

package enterchroot

import (
    "os"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestInProcFS_WithSquashfs(t *testing.T) {
    tmp := writeTempFile(t, "nodev\tsquashfs\n")
    procFilesystemsPath = tmp
    t.Cleanup(func() { procFilesystemsPath = "/proc/filesystems" })

    assert.True(t, inProcFS())
}

func TestInProcFS_WithoutSquashfs(t *testing.T) {
    tmp := writeTempFile(t, "nodev\text4\n")
    procFilesystemsPath = tmp
    t.Cleanup(func() { procFilesystemsPath = "/proc/filesystems" })

    assert.False(t, inProcFS())
}

func TestCheckSquashfs_ReturnsError_WhenNotSupported(t *testing.T) {
    tmp := writeTempFile(t, "nodev\text4\n")
    procFilesystemsPath = tmp
    t.Cleanup(func() { procFilesystemsPath = "/proc/filesystems" })

    err := checkSquashfs()
    require.Error(t, err)
    assert.Contains(t, err.Error(), "squashfs")
}

func TestCheckSquashfs_ReturnsNil_WhenSupported(t *testing.T) {
    tmp := writeTempFile(t, "\tsquashfs\n")
    procFilesystemsPath = tmp
    t.Cleanup(func() { procFilesystemsPath = "/proc/filesystems" })

    assert.NoError(t, checkSquashfs())
}
```

## Data Models

No new data models. The migration preserves existing error types and interfaces.

## Error Handling

The migration preserves all existing error semantics:

- **Wrapped errors** remain traversable via `errors.Is` and `errors.As`
- **Sentinel errors** (`"interrupted"`, `"failed to bind mount"`) remain comparable via string matching or direct equality
- **Error messages** are identical (the `: ` separator between context and cause is preserved by both `pkg/errors.Wrap` and `fmt.Errorf("%s: %w", ...)`)

**Behavioral difference:** `pkg/errors` attaches stack traces; `fmt.Errorf` does not. This project uses `logrus` for logging with context, so stack traces from `pkg/errors` were never surfaced to users. No behavioral regression.

## Interfaces

No new interfaces are introduced. The migration is internal to function bodies.

The only structural change is the introduction of a `procFilesystemsPath` package-level variable in `internal/enterchroot/enter.go` to enable test injection of `/proc/filesystems` content.

## Correctness Properties

*A property is a characteristic or behavior that should hold true across all valid executions of a system — essentially, a formal statement about what the system should do. Properties serve as the bridge between human-readable specifications and machine-verifiable correctness guarantees.*

### Property 1: Error wrapping preserves cause chain

*For any* cause error passed through a `fmt.Errorf("...: %w", ..., err)` call site in the migrated code, `errors.Is(wrappedErr, cause)` SHALL return true, confirming the error chain is intact and the original cause is reachable.

**Validates: Requirements 1.5, 2.6**

### Property 2: MaskPassword I/O error propagation

*For any* error returned by the underlying reader during `MaskPassword` execution, the function SHALL return an error that wraps the original I/O error such that `errors.Is(returnedErr, originalIOErr)` is true.

**Validates: Requirements 4.1, 1.1, 1.5**

### Property 3: inProcFS filesystem detection correctness

*For any* content of `/proc/filesystems`, `inProcFS()` SHALL return true if and only if the content contains the substring `"squashfs"`.

**Validates: Requirements 5.2, 5.3**
