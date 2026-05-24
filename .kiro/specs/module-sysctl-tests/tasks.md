# Implementation Plan: module-sysctl-tests

## Overview

Remove the dead-code standalone packages (`internal/module` and `internal/sysctl`) and add Linux-only integration tests for the real OS adapter implementations (`osimpl.LinuxModuleLoader` and `osimpl.LinuxSysctlApplier`). Work is done on a feature branch with four conventional commits, each passing lint independently.

## Tasks

- [x] 1. Create feature branch and commit spec files
  - [x] 1.1 Create feature branch from master and commit spec
    - Branch from current `master` HEAD to `feature/task-008-module-sysctl-tests`
    - Stage and commit the spec files (`.kiro/specs/module-sysctl-tests/`) with message: `docs(spec): add TASK-008 module-sysctl-tests spec`
    - Verify `golangci-lint run ./...` passes at this commit
    - _Requirements: 6.1, 7.1, 7.2_

- [x] 2. Remove standalone packages
  - [x] 2.1 Delete `internal/module/` and `internal/sysctl/` directories
    - Remove `internal/module/module.go` (confirmed zero callers)
    - Remove `internal/sysctl/sysctl.go` (confirmed zero callers)
    - Verify no import references remain via `grep -r "internal/module\|internal/sysctl" --include="*.go" .`
    - Verify `go build ./...` succeeds without the removed files
    - Commit with message: `refactor(module,sysctl): remove standalone packages in favor of iface-based appliers`
    - Verify `golangci-lint run ./...` passes at this commit
    - _Requirements: 1.1, 1.2, 1.3, 2.1, 2.2, 2.3, 3.1, 6.2, 6.5_

- [x] 3. Add integration tests for osimpl adapters
  - [x] 3.1 Create `internal/iface/osimpl/module_test.go`
    - Add `//go:build linux` build tag
    - Use `package osimpl_test` (external test package)
    - Implement `TestLinuxModuleLoader_LoadedModules_ReturnsNonEmpty` — verifies non-empty map returned
    - Implement `TestLinuxModuleLoader_LoadedModules_NamesHaveNoWhitespace` — verifies no spaces/tabs/leading/trailing whitespace in module names
    - Implement `TestLinuxModuleLoader_LoadedModules_ExtractsOnlyFirstField` — verifies names match `^[a-zA-Z0-9_]+$` regex
    - Use `testify/assert` and `testify/require` for all assertions
    - _Requirements: 4.1, 4.2, 4.3, 4.4, 4.5_

  - [x] 3.2 Create `internal/iface/osimpl/sysctl_test.go`
    - Add `//go:build linux` build tag
    - Use `package osimpl_test` (external test package)
    - Implement `TestLinuxSysctlApplier_Set_WritesToCorrectPath` — reads current value of `net.ipv4.ip_forward`, writes it back to verify dot-to-path conversion
    - Implement `TestLinuxSysctlApplier_Set_DotConversion` — reads current `kernel.hostname`, writes it back to verify multi-segment path conversion
    - Implement `TestLinuxSysctlApplier_Set_NonExistentPath_ReturnsError` — verifies error returned for `nonexistent.fake.key`
    - Use `testify/assert` and `testify/require` for all assertions
    - _Requirements: 5.1, 5.2, 5.3, 5.4, 5.5_

  - [x] 3.3 Run tests via Docker and verify coverage
    - Execute: `docker run --rm -v "$(pwd)":/app -w /app golang:1.21.9-bookworm go test -v ./internal/iface/osimpl/...`
    - Execute: `docker run --rm -v "$(pwd)":/app -w /app golang:1.21.9-bookworm go test -coverprofile=coverage.out ./internal/iface/osimpl/...`
    - Verify all tests pass with exit code 0
    - Verify coverage ≥80% for `module.go` and `sysctl.go`
    - _Requirements: 4.6, 5.6, 8.1, 8.2, 8.3, 9.1, 9.2_

  - [x] 3.4 Commit integration tests
    - Stage `internal/iface/osimpl/module_test.go` and `internal/iface/osimpl/sysctl_test.go`
    - Commit with message: `test(osimpl): add Linux-only integration tests for module and sysctl adapters`
    - Verify `golangci-lint run ./...` passes at this commit
    - _Requirements: 6.3, 6.5_

- [x] 4. Checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [x] 5. Update documentation and finalize
  - [x] 5.1 Update `docs/status.md`
    - Move TASK-008 from "Pending" to "Completed Features" with description: `TASK-008: Add integration tests for osimpl module/sysctl adapters, remove dead standalone packages`
    - Update "Next Steps" section to remove TASK-008
    - _Requirements: 6.4_

  - [x] 5.2 Update `docs/log.md`
    - Add a dated entry documenting the actions taken, files created/removed, coverage results, and retrospective
    - _Requirements: 6.4_

  - [x] 5.3 Update `tasks/tasks.md`
    - Mark TASK-008 status as `Done`
    - Check off all implementation checklist items
    - Update description to reflect the actual work (removal + integration tests, not unit tests of standalone packages)
    - _Requirements: 6.4_

  - [x] 5.4 Commit documentation updates
    - Stage `docs/status.md`, `docs/log.md`, `tasks/tasks.md`
    - Commit with message: `docs: update status for TASK-008 completion`
    - Verify `golangci-lint run ./...` passes at this commit
    - _Requirements: 6.4, 6.5_

- [x] 6. Final checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.
  - Verify the branch has exactly 4 commits on top of master
  - Do NOT merge back to master
  - _Requirements: 7.3_

## Notes

- Tests run via Docker on macOS: `docker run --rm -v "$(pwd)":/app -w /app golang:1.21.9-bookworm go test ...`
- The `//go:build linux` tag ensures tests are skipped on macOS native builds
- Coverage target is ≥80% for both `module.go` (~85% expected, `LoadModule` delegates to modprobe) and `sysctl.go` (100% expected)
- Each commit must independently pass `golangci-lint run ./...`
- The branch must NOT be merged back to master
- No correctness properties section in design requires property-based tests — standard integration tests only

## Task Dependency Graph

```json
{
  "waves": [
    { "id": 0, "tasks": ["1.1"] },
    { "id": 1, "tasks": ["2.1"] },
    { "id": 2, "tasks": ["3.1", "3.2"] },
    { "id": 3, "tasks": ["3.3"] },
    { "id": 4, "tasks": ["3.4"] },
    { "id": 5, "tasks": ["5.1", "5.2", "5.3"] },
    { "id": 6, "tasks": ["5.4"] }
  ]
}
```
