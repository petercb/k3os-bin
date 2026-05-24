# Design Document

## Overview

This document describes the test architecture for `internal/cc` (TASK-007). No production code changes are required beyond adding one unexported field to `Applier`. All new files are `*_test.go` files co-located in `internal/cc/`.

---

## Architecture

The test suite follows the existing project pattern: white-box tests in the same package (`package cc`), using `testify/mock` to substitute all OS-dependent interfaces. The `Applier` struct already accepts all dependencies via injection (TASK-006), so tests simply construct an `Applier` with mock implementations.

```
internal/cc/
├── apply.go                        # existing — Applier struct, chain functions
├── funcs.go                        # existing — individual applier methods
├── filesystem_mock_test.go         # NEW — MockFileSystem + MockFile
├── command_mock_test.go            # NEW — MockCommandRunner
├── module_mock_test.go             # NEW — MockModuleLoader
├── sysctl_mock_test.go             # NEW — MockSysctlApplier
├── hostname_mock_test.go           # NEW — MockHostnameSetter
├── apply_test.go                   # NEW — runApplies + chain function tests
└── funcs_test.go                   # NEW — individual applier function tests
```

The only production code change is adding an unexported `modePrefix []string` field to `Applier` in `apply.go`, and updating `ApplyK3S` and `ApplyInstall` to call `mode.Get(a.modePrefix...)`. The zero value (nil slice) preserves existing behavior.

---

## Components and Interfaces

### Mock Types

Each mock lives in its own `*_mock_test.go` file in `package cc`. All mocks embed `mock.Mock` and implement the corresponding `iface.*` interface exactly.

| Mock Type | File | Interface |
|-----------|------|-----------|
| `MockFileSystem` | `filesystem_mock_test.go` | `iface.FileSystem` |
| `MockFile` | `filesystem_mock_test.go` | `iface.File` |
| `MockCommandRunner` | `command_mock_test.go` | `iface.CommandRunner` |
| `MockModuleLoader` | `module_mock_test.go` | `iface.ModuleLoader` |
| `MockSysctlApplier` | `sysctl_mock_test.go` | `iface.SysctlApplier` |
| `MockHostnameSetter` | `hostname_mock_test.go` | `iface.HostnameSetter` |

### MockFileSystem

Implements all 12 methods of `iface.FileSystem`. The `ReadFile`, `Stat`, `Open`, `Create`, and `CreateTemp` methods handle nil returns safely by checking `args.Get(0) == nil` before type-asserting.

### MockFile

A minimal `iface.File` implementation used when `Open`, `Create`, or `CreateTemp` must return a file handle. Uses an internal `bytes.Buffer` for `Read`/`Write` so tests can inspect written content without touching the real filesystem.

```go
type MockFile struct {
    mock.Mock
    buf bytes.Buffer
}
func (f *MockFile) Read(p []byte) (int, error)  { return f.buf.Read(p) }
func (f *MockFile) Write(p []byte) (int, error) { return f.buf.Write(p) }
func (f *MockFile) Close() error                { return f.Called().Error(0) }
func (f *MockFile) Name() string                { return f.Called().String(0) }
```

### newTestApplier Helper

Both test files use a shared constructor in `funcs_test.go`:

```go
func newTestApplier(
    fs  *MockFileSystem,
    cmd *MockCommandRunner,
    mod *MockModuleLoader,
    sys *MockSysctlApplier,
    hn  *MockHostnameSetter,
) *Applier {
    return &Applier{
        FS:       fs,
        Cmd:      cmd,
        Modules:  mod,
        Sysctl:   sys,
        Hostname: hn,
    }
}
```

Callers pass `nil` for interfaces not needed by the function under test.

---

## Data Models

### Applier (modified)

```go
type Applier struct {
    FS         iface.FileSystem
    Cmd        iface.CommandRunner
    Modules    iface.ModuleLoader
    Sysctl     iface.SysctlApplier
    Mounter    iface.Mounter
    Hostname   iface.HostnameSetter
    modePrefix []string  // NEW: injected in tests; nil = production default
}
```

### Mode File Injection

`ApplyK3S` and `ApplyInstall` are updated to call `mode.Get(a.modePrefix...)` instead of `mode.Get()`. In tests, `modePrefix` is set to a `t.TempDir()` path where the test has written the desired mode string:

```go
// test setup
root := t.TempDir()
modePath := filepath.Join(root, system.StatePath("mode"))
require.NoError(t, os.MkdirAll(filepath.Dir(modePath), 0o755))
require.NoError(t, os.WriteFile(modePath, []byte("live"), 0o644))

a := newTestApplier(mockFS, mockCmd, nil, nil, nil)
a.modePrefix = []string{root}
```

### Mock passwd Content

Tests for `ApplySSHKeys` / `ApplySSHKeysWithNet` require a mock `/etc/passwd` with a valid `rancher` entry:

```
rancher:x:1000:1000::/home/rancher:/bin/sh
```

This is returned by `mockFS.On("ReadFile", "/etc/passwd").Return([]byte(...), nil)`.

---

## Error Handling

### runApplies Aggregation

`runApplies` collects all errors from all appliers and returns a `cli.MultiError` if any failed. Tests verify:

1. All appliers are called even when earlier ones fail (no short-circuit).
2. The returned error is a `cli.MultiError` containing exactly the expected sub-errors.
3. When all appliers succeed, `nil` is returned.

### ApplyWriteFiles Error Swallowing

`writefile.WriteFiles` logs errors internally and does not return them. `ApplyWriteFiles` always returns `nil`. Tests for error cases verify that the function returns `nil` even when `CreateTemp` fails — they do not assert on the error return value.

### Nil-Safe Mock Returns

All mock methods that return interface values (`Open`, `Create`, `CreateTemp`, `Stat`) check `args.Get(0) == nil` before type-asserting to avoid panics when the mock is set up to return an error with a nil value.

---

## Testing Strategy

### Test File Organization

- `funcs_test.go`: one `Test<FunctionName>` per applier. Functions with multiple scenarios use table-driven tests with `t.Run`.
- `apply_test.go`: `TestRunApplies_*` for aggregation behavior; `Test<Chain>_ChainComposition` for each phase function.

### Table-Driven Tests

Used for appliers with multiple input variants:

- `ApplyPassword`: empty / plain / hashed
- `ApplyDNS`: with DNS / default DNS / with NTP
- `ApplyModules`: not loaded / already loaded
- `ApplyRuncmd` / `ApplyBootcmd` / `ApplyInitcmd`: empty / one command

### Chain Composition Verification

Chain tests wire all mocks to return `nil` for every expected call, then call the chain function and assert `mock.AssertExpectations(t)`. They also use `mock.AssertNotCalled` to verify that appliers outside the chain are not invoked (e.g., `RunShell` must not be called in `InitApply` when `Initcmd` is empty).

---

## Correctness Properties

### Property 1: runApplies never skips appliers on error

**Validates: Requirements 3.1**

For any sequence of N appliers where K fail, all N appliers are called and the returned error contains exactly K sub-errors.

### Property 2: ApplyPassword flag selection is deterministic

**Validates: Requirements 2.6**

For any password string, the `-e` flag is included if and only if the string starts with `$`.

### Property 3: ApplyDNS always writes a FallbackNameservers line

**Validates: Requirements 2.4**

For any `CloudConfig`, the written `/etc/connman/main.conf` always contains exactly one `FallbackNameservers=` line.

### Property 4: ApplyEnvironment merge is idempotent

**Validates: Requirements 2.10**

Calling `ApplyEnvironment` twice with the same config and the same existing file content produces the same output file.

### Property 5: ApplyWifi is a no-op for empty wifi list

**Validates: Requirements 2.5**

For any `CloudConfig` with an empty `K3OS.Wifi` slice, no FS methods are called.
