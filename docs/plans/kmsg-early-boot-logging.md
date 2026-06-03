# Plan: kmsg Early Boot Logging

## Problem

When k3os boots as PID 1 on a bare initramfs, any log output directed to
`os.Stderr` is effectively invisible. There is no console device configured, no
journald, and no persistent filesystem yet. Debugging early boot failures
requires kernel-level visibility that only the kernel ring buffer provides.

Previously, `enter.go` set up a `slog.TextHandler` targeting `os.Stderr`, and
`setResourceLimit()` used `log.Printf`. Both outputs were lost before the
console device was available.

## Solution

Wire `slog` to `/dev/kmsg` via the `github.com/siderolabs/go-kmsg` library.
This ensures all structured log output appears in `dmesg`, which is always
available as long as the kernel is running (no filesystem dependency beyond the
`/dev/kmsg` character device).

### Library: go-kmsg v0.1.6

- Repository: `github.com/siderolabs/go-kmsg`
- Provides `kmsg.Writer` which handles:
  - Line splitting for multi-line messages
  - Truncation to 976 characters (kernel printk buffer limit)
- License: **MPL-2.0** (file-level copyleft; not viral like GPL). Compatible
  with the project's MIT/Apache licensing. The MPL-2.0 license notice must be
  preserved in the dependency tree but does not require relicensing this project.

## Design

### Package: `internal/klog`

A small package (`klog.go` with `//go:build linux`) that exports:

- **`Setup() *EarlyLogger`** - Opens `/dev/kmsg`, wraps with `kmsg.Writer`,
  creates `slog.NewTextHandler` targeting that writer, calls `slog.SetDefault`.
  Returns an `EarlyLogger` struct holding the file handle and a shared
  `slog.LevelVar`.
- **`EarlyLogger.SetDebug()`** - Lowers level to `slog.LevelDebug`.
- **`EarlyLogger.Level() *slog.LevelVar`** - Returns the shared level variable
  for external adjustment (e.g., passing to init orchestrator).
- **`EarlyLogger.Close()`** - Closes the `/dev/kmsg` file descriptor.

### Fallback Behavior

If `/dev/kmsg` cannot be opened (containers, tests, non-Linux environments),
`Setup()` transparently falls back to `os.Stderr`. A warning is logged via the
fallback handler. Callers do not need to check for errors or handle the
degraded case differently.

## Integration Points

### 1. `internal/enterchroot/enter.go` - `Enter()`

At the top of `Enter()`, `klog.Setup()` replaces the previous
`slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, ...)))` call. The
returned `EarlyLogger` is used to enable debug if `ENTER_DEBUG` is set, and its
`LevelVar` is passed downstream.

The `setResourceLimit()` function now uses `slog.Warn` instead of
`log.Printf`, routing resource limit warnings through the same kmsg-backed
handler.

### 2. `main.go` - `postChroot()`

After chroot/pivot, `postChroot()` calls `klog.Setup()` again (new process
context after exec). The returned `EarlyLogger.Level()` is passed to the `Init`
struct so the init orchestrator shares the same level control.

### 3. `internal/boot/init.go` - `Init.Run()`

The `Init` struct has a new `LogLevel *slog.LevelVar` field. When this field is
set (non-nil), `Init.Run()` uses the pre-configured slog default handler rather
than creating its own. This allows the init orchestrator to inherit the
kmsg-backed handler established by `postChroot()`.

After `ConsoleRedirect` succeeds, logs continue to flow to kmsg. The kernel
ring buffer remains useful for debugging regardless of console state.

## Message Flow

```text
Enter() --> klog.Setup() --> /dev/kmsg --> kernel ring buffer --> dmesg
   |
   v (exec into /sbin/init)
postChroot() --> klog.Setup() --> /dev/kmsg --> kernel ring buffer --> dmesg
   |
   v
Init.Run() (inherits slog default) --> /dev/kmsg --> kernel ring buffer
```

## Testing

- `internal/klog/klog_test.go` achieves 94.1% coverage by overriding the
  package-level `openKmsg` variable with test doubles.
- The fallback path is tested by making `openKmsg` return an error.
- Integration with `enter.go` and `init.go` is tested via their existing test
  suites which exercise the slog output paths.
