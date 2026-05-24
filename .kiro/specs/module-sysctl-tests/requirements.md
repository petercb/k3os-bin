# Requirements Document

## Introduction

TASK-008 deprecates the standalone `internal/module` and `internal/sysctl` packages (whose logic is already superseded by `cc.Applier.ApplyModules` and `cc.Applier.ApplySysctls` at 93.5% coverage), adds Linux-only integration tests for the real OS adapter implementations (`osimpl.LinuxModuleLoader` and `osimpl.LinuxSysctlApplier`), and updates any remaining callers to use the interface-based approach.

## Glossary

- **LinuxModuleLoader**: The `osimpl.LinuxModuleLoader` struct in `internal/iface/osimpl/module.go` that implements `iface.ModuleLoader` by reading `/proc/modules` and calling `modprobe.Load()`.
- **LinuxSysctlApplier**: The `osimpl.LinuxSysctlApplier` struct in `internal/iface/osimpl/sysctl.go` that implements `iface.SysctlApplier` by writing values to `/proc/sys/` paths.
- **Standalone_Module_Package**: The `internal/module/module.go` file containing `LoadModules(cfg)` — now superseded by `cc.Applier.ApplyModules`.
- **Standalone_Sysctl_Package**: The `internal/sysctl/sysctl.go` file containing `ConfigureSysctl(cfg)` — now superseded by `cc.Applier.ApplySysctls`.
- **Integration_Test**: A test that exercises real OS adapter code on a Linux system, guarded by `//go:build linux` build tags.
- **Docker_Test_Runner**: The `docker run --rm -v "$(pwd)":/app -w /app golang:1.21.9-bookworm go test ...` command used to run Linux-only tests on macOS.
- **Feature_Branch**: A Git branch created from master following the pattern `feature/task-008-module-sysctl-tests`.

## Requirements

### Requirement 1: Deprecate Standalone Module Package

**User Story:** As a maintainer, I want the standalone `internal/module` package removed, so that there is a single implementation path for kernel module loading via the interface-based `cc.Applier`.

#### Acceptance Criteria

1. WHEN the deprecation commit is applied, THE Build_System SHALL compile successfully without the `internal/module/module.go` file present.
2. WHEN the deprecation commit is applied, THE Build_System SHALL contain zero import references to `github.com/petercb/k3os-bin/internal/module` across the codebase.
3. WHEN the deprecation commit is applied, THE Build_System SHALL pass `golangci-lint run ./...` with zero errors related to the removal.

### Requirement 2: Deprecate Standalone Sysctl Package

**User Story:** As a maintainer, I want the standalone `internal/sysctl` package removed, so that there is a single implementation path for sysctl configuration via the interface-based `cc.Applier`.

#### Acceptance Criteria

1. WHEN the deprecation commit is applied, THE Build_System SHALL compile successfully without the `internal/sysctl/sysctl.go` file present.
2. WHEN the deprecation commit is applied, THE Build_System SHALL contain zero import references to `github.com/petercb/k3os-bin/internal/sysctl` across the codebase.
3. WHEN the deprecation commit is applied, THE Build_System SHALL pass `golangci-lint run ./...` with zero errors related to the removal.

### Requirement 3: Update Callers to Use Interface-Based Approach

**User Story:** As a maintainer, I want any code that previously imported `internal/module` or `internal/sysctl` updated to use `cc.Applier` or direct `iface` implementations, so that the codebase has a single consistent pattern for OS operations.

#### Acceptance Criteria

1. WHEN the caller update commit is applied, THE Build_System SHALL compile with zero unresolved import errors.
2. WHEN the caller update commit is applied, THE Build_System SHALL pass the full test suite via `docker run --rm -v "$(pwd)":/app -w /app golang:1.21.9-bookworm go test ./...`.
3. THE Build_System SHALL maintain identical runtime behavior for module loading and sysctl application after the caller migration.

### Requirement 4: Integration Tests for LinuxModuleLoader

**User Story:** As a maintainer, I want integration tests for `osimpl.LinuxModuleLoader`, so that the `/proc/modules` parsing and module name extraction logic is verified on a real Linux system.

#### Acceptance Criteria

1. THE Integration_Test file SHALL use the `//go:build linux` build tag.
2. WHEN the test file is executed on Linux, THE Integration_Test SHALL verify that `LoadedModules()` returns a non-empty map of currently loaded kernel modules.
3. WHEN the test file is executed on Linux, THE Integration_Test SHALL verify that `LoadedModules()` returns module names without trailing whitespace or parameters.
4. WHEN `/proc/modules` contains a line with format `<name> <size> <refcount> <deps> <state> <offset>`, THE LinuxModuleLoader SHALL extract only the first field as the module name key.
5. THE Integration_Test SHALL use `testify/assert` and `testify/require` for assertions.
6. THE Integration_Test SHALL achieve at least 80% code coverage for `internal/iface/osimpl/module.go`.

### Requirement 5: Integration Tests for LinuxSysctlApplier

**User Story:** As a maintainer, I want integration tests for `osimpl.LinuxSysctlApplier`, so that the dot-notation to `/proc/sys/` path conversion logic is verified on a real Linux system.

#### Acceptance Criteria

1. THE Integration_Test file SHALL use the `//go:build linux` build tag.
2. WHEN `Set("net.ipv4.ip_forward", "1")` is called, THE LinuxSysctlApplier SHALL write the value to the file path `/proc/sys/net/ipv4/ip_forward`.
3. WHEN a sysctl key contains dots, THE LinuxSysctlApplier SHALL convert each dot to a path separator in the `/proc/sys/` hierarchy.
4. IF the target `/proc/sys/` path does not exist, THEN THE LinuxSysctlApplier SHALL return a non-nil error.
5. THE Integration_Test SHALL use `testify/assert` and `testify/require` for assertions.
6. THE Integration_Test SHALL achieve at least 80% code coverage for `internal/iface/osimpl/sysctl.go`.

### Requirement 6: Commit Structure

**User Story:** As a maintainer, I want a clean commit history following conventional commits, so that changes are reviewable and bisectable.

#### Acceptance Criteria

1. THE Feature_Branch SHALL contain a spec commit as the first commit with message format `docs(spec): add TASK-008 module-sysctl-tests spec`.
2. THE Feature_Branch SHALL contain a deprecation commit with message format `refactor(module,sysctl): remove standalone packages in favor of iface-based appliers`.
3. THE Feature_Branch SHALL contain a test commit with message format `test(osimpl): add Linux-only integration tests for module and sysctl adapters`.
4. THE Feature_Branch SHALL contain a docs commit with message format `docs: update status for TASK-008 completion`.
5. WHEN each commit is applied incrementally, THE Build_System SHALL pass `golangci-lint run ./...` at that commit.

### Requirement 7: Branch and Workflow

**User Story:** As a maintainer, I want work done on a feature branch from master without merging back, so that the changes can be reviewed independently.

#### Acceptance Criteria

1. THE Feature_Branch SHALL be created from the current `master` branch HEAD.
2. THE Feature_Branch SHALL follow the naming pattern `feature/task-008-module-sysctl-tests`.
3. THE Feature_Branch SHALL NOT be merged back into `master` as part of this task.

### Requirement 8: Test Execution via Docker

**User Story:** As a developer on macOS, I want to run Linux-only integration tests via Docker, so that I can verify the tests pass without a native Linux environment.

#### Acceptance Criteria

1. WHEN `docker run --rm -v "$(pwd)":/app -w /app golang:1.21.9-bookworm go test -v ./internal/iface/osimpl/...` is executed, THE Docker_Test_Runner SHALL run the Linux-tagged integration tests successfully.
2. WHEN `docker run --rm -v "$(pwd)":/app -w /app golang:1.21.9-bookworm go test -coverprofile=coverage.out ./internal/iface/osimpl/...` is executed, THE Docker_Test_Runner SHALL produce a coverage report showing at least 80% coverage for the tested adapter files.
3. WHEN the tests are run via Docker, THE Docker_Test_Runner SHALL complete with exit code 0 for all passing tests.

### Requirement 9: Coverage Target

**User Story:** As a maintainer, I want at least 80% test coverage for the osimpl adapter files, so that the real OS interaction code has adequate verification.

#### Acceptance Criteria

1. WHEN coverage is measured for `internal/iface/osimpl/module.go`, THE Integration_Test SHALL achieve at least 80% statement coverage.
2. WHEN coverage is measured for `internal/iface/osimpl/sysctl.go`, THE Integration_Test SHALL achieve at least 80% statement coverage.
3. IF coverage falls below 80% for either file, THEN THE Build_System SHALL report the coverage gap in the test output.
