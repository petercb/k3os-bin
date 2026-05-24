# Task Summary: TASK-006 - Introduce interfaces for OS-dependent operations

## Status: Done

## 🎯 What Was Done

- Created `internal/iface` package containing interfaces for file system (`FileSystem`), commands (`CommandRunner`), module loading (`ModuleLoader`), sysctls (`SysctlApplier`), mounting (`Mounter`), and hostname (`HostnameSetter`).
- Created `internal/iface/osimpl` package containing production implementations of these interfaces wrapping `os`, `exec`, `unix`, and `modprobe`.
- Added Linux build tags and non-Linux unsupported stubs for Linux-only adapters so host-side package checks compile without invoking Linux syscalls.
- Refactored `internal/cc/apply.go` to use an `Applier` struct to hold these dependencies.
- Updated `internal/cc/funcs.go` so all `Apply*` functions are methods on `Applier` and delegate OS operations to the injected interfaces.
- Updated downstream packages (`internal/hostname`, `internal/ssh`, `internal/writefile`) to accept interfaces, facilitating their unit testing in future tasks.
- Fixed a pre-existing flaky test in `internal/config/rename_test.go` and resolved linter issues.

## 🔗 PRD Alignment

- Aligns with the engineering patterns in @{docs/technical.md} requiring testable code and dependency injection.
- Unblocks TASK-007 and TASK-008 for writing comprehensive unit tests for the `cc` appliers and downstream packages.

## 💻 Code Implemented/Modified

- **Key Source Files:**
  - `internal/iface/iface.go` (New interfaces)
  - `internal/iface/osimpl/*.go` (New production implementations)
  - `internal/mount/unsupported.go` (Non-Linux mount stubs)
  - `internal/cc/apply.go` (Refactored to `Applier` struct)
  - `internal/cc/funcs.go` (Refactored to methods using interfaces)
  - `internal/hostname/hostname.go` (Accepts interfaces)
  - `internal/ssh/ssh.go` (Accepts interfaces)
  - `internal/writefile/writefile.go` (Accepts interfaces)
  - `internal/cli/config/config.go` (Uses `cc.NewDefaultApplier()`)
- **Key Functions/Modules Changed:**
  - `Applier` struct and its methods in `cc`
  - `ssh.SetAuthorizedKeys`
  - `writefile.WriteFiles`
  - `hostname.SetHostname`

## 🧪 Tests Written/Modified

- **Key Test Files:**
  - `internal/config/rename_test.go` (Fixed flaky test `TestFuzzyNames` related to map iteration order)
- **Coverage Notes:**
  - This task introduces the interfaces that will be used for testing in subsequent tasks. Owner-approved scoped verification confirmed compilation and linting for the changed packages while avoiding unrelated project-wide issues.

## 🧐 Final Review Results

- **CODE_REVIEWER_MODE Summary:**
  - Verified that all `os.*` and `exec.*` calls in `cc/funcs.go` were properly abstracted.
  - Verified interfaces map perfectly to existing logic.
  - Verified `FileSystem` returns the lightweight `iface.File` interface rather than leaking `*os.File`, preserving in-memory mockability for upcoming tests.
- **TECH_DEBT_REFACTOR Summary:**
  - Cleaned up several unchecked errors found by `golangci-lint` (e.g. `defer fs.Remove` in `ssh.go`).
  - Addressed some tech debt by fixing the flaky test in `rename_test.go`.

## 🪵 Link to Main Log Entry

- For detailed activity, see log entries around 2026-05-22 and 2026-05-23 23:43 in @{docs/log.md} for task TASK-006.
