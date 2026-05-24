# Implementation Plan: cc-applier-tests (TASK-007)

## Overview

Implementation tasks for adding unit tests to `internal/cc`. All work is test-only except for one minimal production change (adding `modePrefix` to `Applier`).

---

## Tasks

### Phase 1 — Mock Infrastructure

- [x] 1.1 Create `filesystem_mock_test.go` with `MockFileSystem` and `MockFile` types implementing `iface.FileSystem` and `iface.File`
- [x] 1.2 Create `command_mock_test.go` with `MockCommandRunner` implementing `iface.CommandRunner`
- [x] 1.3 Create `module_mock_test.go` with `MockModuleLoader` implementing `iface.ModuleLoader`
- [x] 1.4 Create `sysctl_mock_test.go` with `MockSysctlApplier` implementing `iface.SysctlApplier`
- [x] 1.5 Create `hostname_mock_test.go` with `MockHostnameSetter` implementing `iface.HostnameSetter`
- [x] 1.6 Add unexported `modePrefix []string` field to `Applier` in `apply.go`; update `ApplyK3S` and `ApplyInstall` to call `mode.Get(a.modePrefix...)` instead of `mode.Get()`

### Phase 2 — Individual Applier Tests (funcs_test.go)

- [x] 2.1 Write tests for `ApplyModules`: module not loaded (calls `LoadModule`), module already loaded (skips), `LoadedModules` error, `LoadModule` error
- [x] 2.2 Write tests for `ApplySysctls`: one entry calls `Set`, empty map skips, `Set` error propagated
- [x] 2.3 Write tests for `ApplyHostname`: hostname set (calls `SetHostname` + FS writes), empty hostname (no-op), `SetHostname` error
- [x] 2.4 Write tests for `ApplyDNS`: with DNS servers, default DNS (8.8.8.8), with NTP servers, `WriteFile` error
- [x] 2.5 Write tests for `ApplyWifi`: empty wifi (no-op), one wifi entry (correct file content), `MkdirAll` error, `WriteFile` error
- [x] 2.6 Write tests for `ApplyPassword`: empty (no-op), plain password (no `-e`), hashed password (with `-e`), `RunWithStdin` error
- [x] 2.7 Write tests for `ApplyRuncmd`: empty list (no-op), one command, `RunShell` error
- [x] 2.8 Write tests for `ApplyBootcmd`: empty list (no-op), one command, `RunShell` error
- [x] 2.9 Write tests for `ApplyInitcmd`: empty list (no-op), one command, `RunShell` error
- [x] 2.10 Write tests for `ApplyWriteFiles`: one plain-content entry (full FS call sequence), empty list (no-op), `CreateTemp` error (error logged, nil returned)
- [x] 2.11 Write tests for `ApplySSHKeys` and `ApplySSHKeysWithNet`: happy path with mock passwd content, `ReadFile("/etc/passwd")` error
- [x] 2.12 Write tests for `ApplyEnvironment`: new file (no existing content), merge with existing content, empty map (no-op), `WriteFile` error
- [x] 2.13 Write tests for `ApplyDataSource`: one data source (correct file content), empty list (no-op), `WriteFile` error
- [x] 2.14 Write tests for `ApplyK3S`: install mode (early return), k3s exists + restart=true, k3s exists + restart=false, server URL set (agent mode), `mode.Get` error
- [x] 2.15 Write tests for `ApplyInstall`: not install mode (no `Run` call), install mode (calls `Run("k3os", "install")`), `mode.Get` error

### Phase 3 — Chain and Aggregation Tests (apply_test.go)

- [x] 3.1 Write `TestRunApplies_AllSucceed`: all appliers return nil → result is nil
- [x] 3.2 Write `TestRunApplies_SingleError`: one applier fails → error returned
- [x] 3.3 Write `TestRunApplies_MultipleErrors_AllRun`: two appliers fail → all appliers called, returned error is `cli.MultiError` with two sub-errors
- [x] 3.4 Write `TestRunApply_ChainComposition`: wire all mocks to return nil; call `RunApply`; assert `AssertExpectations` and verify `ApplySSHKeysWithNet` path (not `ApplySSHKeys`)
- [x] 3.5 Write `TestBootApply_ChainComposition`: verify `ApplyDataSource`, `ApplySSHKeys` (not `ApplySSHKeysWithNet`), and `ApplyBootcmd` are in the chain
- [x] 3.6 Write `TestInitApply_ChainComposition`: verify `ApplyModules`, `ApplySysctls`, `ApplyHostname`, `ApplyWriteFiles`, `ApplyEnvironment`, `ApplyInitcmd` are called; `ApplyRuncmd` and `ApplySSHKeys` are NOT called
- [x] 3.7 Write `TestInstallApply_ChainComposition`: verify only `ApplyK3SWithRestart` path is exercised

### Phase 4 — Coverage Verification

- [x] 4.1 Run `go test -coverprofile=coverage.out ./internal/cc/...` inside Docker and verify ≥60% coverage for `cc/funcs.go` and `cc/apply.go`
- [x] 4.2 Run `go test -race -covermode=atomic -failfast ./internal/cc/...` inside Docker and confirm all tests pass with race detector enabled
- [x] 4.3 Run `golangci-lint run ./internal/cc/...` and fix any lint issues in the new test files

---

## Task Dependency Graph

```json
{
  "waves": [
    { "wave": 1, "tasks": ["1.1", "1.2", "1.3", "1.4", "1.5", "1.6"] },
    { "wave": 2, "tasks": ["2.1", "2.2", "2.3", "2.4", "2.5", "2.6", "2.7", "2.8", "2.9", "2.10", "2.11", "2.12", "2.13", "2.14", "2.15"] },
    { "wave": 3, "tasks": ["3.1", "3.2", "3.3", "3.4", "3.5", "3.6", "3.7"] },
    { "wave": 4, "tasks": ["4.1", "4.2", "4.3"] }
  ]
}
```

Phase 1 tasks (1.1–1.6) are independent of each other and can be done in any order. All Phase 2 tasks depend on the relevant mock(s) from Phase 1 being complete. Phase 3 depends on all Phase 2 tasks. Phase 4 depends on all Phase 3 tasks.

---

## Acceptance Criteria

- Every applier function in `funcs.go` has at least one happy-path test and at least one error-case test
- `runApplies` error aggregation is verified: all appliers run even when earlier ones fail, and all errors are collected
- Chain composition is verified for all four phase functions (`RunApply`, `BootApply`, `InitApply`, `InstallApply`)
- Mock expectations are asserted with `mock.AssertExpectations(t)` in every test that sets up expectations
- Coverage ≥60% for both `cc/funcs.go` and `cc/apply.go`
- All tests pass with `-race` flag inside `golang:1.21.9-bookworm`
- No real filesystem, shell, or kernel calls in any test
- Lint passes with zero new issues

---

## Notes

- The `modePrefix []string` field added to `Applier` in task 1.6 is the only production code change. It is unexported and zero-value safe (empty slice → `mode.Get()` with no args → existing behavior preserved).
- `ApplyWriteFiles` delegates to `writefile.WriteFiles` which swallows errors internally (logs them). Tests for this function verify the FS call sequence but expect `nil` return even on `CreateTemp` failure.
- `ApplySSHKeys` / `ApplySSHKeysWithNet` tests require a mock `/etc/passwd` with a valid `rancher:x:1000:1000::/home/rancher:/bin/sh` line to satisfy `findUserHomeDir`.
- Chain composition tests for `RunApply` and `BootApply` must set up mode file injection (via `modePrefix`) to avoid `ApplyK3S` / `ApplyInstall` reading the real filesystem.
