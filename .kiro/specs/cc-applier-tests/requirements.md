# Requirements Document

## Introduction

Add unit tests for the `internal/cc` package — specifically `apply.go` (the `Applier` struct and phase-chain functions) and `funcs.go` (the individual applier methods). This is TASK-007 in the project task list. Tests must use `testify/mock` to substitute all OS-dependent interfaces, run on macOS via Docker (`golang:1.21.9-bookworm`), and achieve ≥60% coverage for both files.

---

## Requirements

### 1. Mock Infrastructure

#### 1.1 One mock file per interface

Each `iface.*` interface used by the `cc` package must have its own `*_mock_test.go` file inside `internal/cc/`. The mock type must be named `Mock<Interface>` and embed `mock.Mock`.

- `MockFileSystem` → `filesystem_mock_test.go`
- `MockCommandRunner` → `command_mock_test.go`
- `MockModuleLoader` → `module_mock_test.go`
- `MockSysctlApplier` → `sysctl_mock_test.go`
- `MockHostnameSetter` → `hostname_mock_test.go`

The `MockMounter` interface is defined in `iface` but is not used by any current applier function; it must be omitted unless a test requires it.

#### 1.2 Mock method signatures must match the interface exactly

Every method on each mock must have the same signature as the corresponding `iface` interface method. Mocks must delegate to `m.Called(...)` and return `args.Error(0)` (or the appropriate typed getter) so `AssertExpectations` works correctly.

---

### 2. Individual Applier Tests

Each applier function in `funcs.go` must have at least one happy-path test and at least one error-case test.

#### 2.1 ApplyModules

- **Happy path**: module not yet loaded → `LoadModule` is called once with the correct name and params.
- **Happy path (skip)**: module already in the loaded map → `LoadModule` is never called.
- **Error case**: `LoadedModules` returns an error → the error is propagated.
- **Error case**: `LoadModule` returns an error → the error is propagated and wrapped with context.

#### 2.2 ApplySysctls

- **Happy path**: one or more sysctl entries → `Set` is called once per entry with the correct key/value.
- **Happy path (empty)**: empty sysctls map → `Set` is never called, no error.
- **Error case**: `Set` returns an error → the error is propagated.

#### 2.3 ApplyHostname

- **Happy path**: hostname set in config → `SetHostname` is called, `WriteFile` is called for `/etc/hostname`, `Open` is called for `/etc/hosts`.
- **Happy path (empty)**: empty hostname → no calls to `SetHostname`, no error.
- **Error case**: `SetHostname` returns an error → the error is propagated.

#### 2.4 ApplyDNS

- **Happy path (with DNS)**: DNS nameservers configured → `WriteFile` is called for `/etc/connman/main.conf` with `FallbackNameservers` line containing the configured servers.
- **Happy path (default DNS)**: no DNS nameservers → `WriteFile` is called with `FallbackNameservers=8.8.8.8`.
- **Happy path (with NTP)**: NTP servers configured → written file contains `FallbackTimeservers` line.
- **Error case**: `WriteFile` returns an error → the error is propagated and wrapped.

#### 2.5 ApplyWifi

- **Happy path (empty)**: no wifi entries → no FS calls, no error.
- **Happy path**: one or more wifi entries → `MkdirAll` called for `/var/lib/connman`, `WriteFile` called for `settings` and `cloud-config.config` with correct SSID/passphrase content.
- **Error case**: `MkdirAll` returns an error → the error is propagated and wrapped.
- **Error case**: `WriteFile` for settings returns an error → the error is propagated and wrapped.

#### 2.6 ApplyPassword

- **Happy path (plain)**: password without `$` prefix → `RunWithStdin` called with `"rancher:<password>"` and no `-e` flag.
- **Happy path (hashed)**: password with `$` prefix → `RunWithStdin` called with `-e` flag.
- **Happy path (empty)**: empty password → `RunWithStdin` never called, no error.
- **Error case**: `RunWithStdin` returns an error → the error is propagated.

#### 2.7 ApplyRuncmd / ApplyBootcmd / ApplyInitcmd

These three functions share the same structure. For each:

- **Happy path**: one or more commands → `RunShell` called once per command in order.
- **Happy path (empty)**: empty command list → `RunShell` never called, no error.
- **Error case**: `RunShell` returns an error → the error is propagated.

#### 2.8 ApplyWriteFiles

- **Happy path**: one write_files entry with plain content → `CreateTemp`, `Chmod`, `Rename` called in order; no error returned.
- **Happy path (empty)**: no write_files entries → no FS calls.
- **Error case**: `CreateTemp` returns an error → error is logged (not returned, since `WriteFiles` swallows errors); function returns `nil`.

#### 2.9 ApplySSHKeys / ApplySSHKeysWithNet

- **Happy path**: mock FS returns a valid `/etc/passwd` with a `rancher` entry; `Stat` on the SSH dir returns `os.ErrNotExist`; `MkdirAll`, `Chown`, `Create`, `Chmod`, `Stat`, `ReadFile`, `CreateTemp`, `Rename` are called in the expected sequence.
- **Error case**: `ReadFile("/etc/passwd")` returns an error → the error is propagated.
- **Distinction**: `ApplySSHKeys` passes `withNet=false`; `ApplySSHKeysWithNet` passes `withNet=true`. Both must be tested.

#### 2.10 ApplyEnvironment

- **Happy path (new file)**: `ReadFile("/etc/environment")` returns `os.ErrNotExist`; configured env vars are written to the file.
- **Happy path (merge)**: `ReadFile("/etc/environment")` returns existing content; new vars are merged and the file is rewritten.
- **Happy path (empty)**: empty environment map → no FS calls, no error.
- **Error case**: `WriteFile` returns an error → the error is propagated and wrapped.

#### 2.11 ApplyDataSource

- **Happy path**: one or more data sources → `WriteFile` called for `/etc/conf.d/cloud-config` with correct `command_args` content.
- **Happy path (empty)**: no data sources → no FS calls, no error.
- **Error case**: `WriteFile` returns an error → the error is propagated and wrapped.

#### 2.12 ApplyK3S

- **Mode file injection**: tests write a mode file under `t.TempDir()` and the `Applier.modePrefix` field is set to the temp dir so `mode.Get(a.modePrefix...)` reads from it.
- **Happy path (install mode)**: mode is `"install"` → function returns `nil` immediately, no FS or Cmd calls.
- **Happy path (k3s exists, no restart)**: `/sbin/k3s` exists, `restart=false` → `RunWithEnv` is not called.
- **Happy path (k3s exists, restart)**: `/sbin/k3s` exists, `restart=true` → `RunWithEnv` called with `INSTALL_K3S_SKIP_DOWNLOAD=true` and `INSTALL_K3S_BIN_DIR=/sbin`.
- **Happy path (server URL set)**: `ServerURL` non-empty → `K3S_URL` env var included, `agent` appended to args.
- **Error case**: `mode.Get()` fails (unreadable mode file) → the error is propagated.

#### 2.13 ApplyInstall

- **Happy path (not install mode)**: mode is not `"install"` → `Run("k3os", "install")` is never called, no error.
- **Happy path (install mode)**: mode is `"install"` → `Run("k3os", "install")` is called once.
- **Error case**: `mode.Get()` fails → the error is propagated.

---

### 3. Chain Function Tests

#### 3.1 runApplies — error aggregation

- **Happy path**: all appliers succeed → `runApplies` returns `nil`.
- **Single error**: one applier fails → `runApplies` returns a non-nil error.
- **Multiple errors**: two or more appliers fail → `runApplies` returns a `cli.MultiError` containing all errors; no applier is skipped (all run even after earlier failures).

#### 3.2 RunApply chain composition

- All mock interfaces set to return `nil` for every call.
- After calling `RunApply`, assert that the following mock methods were called at least once (in any order): `LoadedModules`, `WriteFile` (for DNS), and the `ApplySSHKeysWithNet` path (not `ApplySSHKeys`).

#### 3.3 BootApply chain composition

- Verify `ApplyDataSource`, `ApplySSHKeys` (not `ApplySSHKeysWithNet`), and `ApplyBootcmd` are in the chain.

#### 3.4 InitApply chain composition

- Verify `ApplyModules`, `ApplySysctls`, `ApplyHostname`, `ApplyWriteFiles`, `ApplyEnvironment`, and `ApplyInitcmd` are in the chain; confirm `ApplyRuncmd` and `ApplySSHKeys` are NOT called.

#### 3.5 InstallApply chain composition

- Verify only `ApplyK3SWithRestart` is in the chain.

---

### 4. Coverage

#### 4.1 Coverage target

Running `go test -coverprofile=coverage.out ./internal/cc/...` must produce ≥60% statement coverage for both `cc/funcs.go` and `cc/apply.go`.

#### 4.2 Coverage verification command

```bash
docker run --rm -v "$(pwd)":/app -w /app golang:1.21.9-bookworm \
  go test -coverprofile=coverage.out ./internal/cc/... && \
  go tool cover -func=coverage.out | grep 'internal/cc'
```

---

### 5. Test Quality

#### 5.1 No happy-path-only tests

Every applier function must have at least one error-case test in addition to its happy-path test(s).

#### 5.2 Mock expectations verified

Every test that sets up mock expectations must call `mock.AssertExpectations(t)` at the end.

#### 5.3 Table-driven tests where applicable

Functions with multiple input variants (e.g., `ApplyPassword` plain vs. hashed, `ApplyDNS` with/without NTP) must use table-driven tests with `t.Run`.

#### 5.4 No real filesystem or OS calls

Tests must not write to or read from the real filesystem, execute real shell commands, or load real kernel modules. All OS interactions must go through mock implementations.

#### 5.5 Tests must pass in Docker

All tests in `internal/cc/` must pass when run inside `golang:1.21.9-bookworm` with no special privileges.

---

## Glossary

| Term | Definition |
|------|-----------|
| Applier | A method on `*Applier` with signature `func(*config.CloudConfig) error` that applies one aspect of a cloud-config |
| Chain function | `RunApply`, `BootApply`, `InitApply`, or `InstallApply` — composes multiple appliers for a boot phase |
| `runApplies` | Internal method that runs a list of appliers and aggregates errors via `cli.NewMultiError` |
| `modePrefix` | Unexported `[]string` field added to `Applier` to redirect `mode.Get()` to a temp directory in tests |
| Mock | A `testify/mock`-based test double implementing one of the `iface.*` interfaces |
