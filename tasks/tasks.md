# Implementation Tasks — k3os-bin

## Task Index

| Task ID | Title | Status | Priority | Dependencies |
|---------|-------|--------|----------|--------------|
| TASK-001 | Add testify dependency and test infrastructure | Done | High | — |
| TASK-002 | Add unit tests for `internal/system` package | Done | High | TASK-001 |
| TASK-003 | Add unit tests for `internal/config` (model, write, coerce) | Done | High | TASK-001 |
| TASK-004 | Add unit tests for `internal/config` (read, merge) | Planned | High | TASK-001, TASK-003 |
| TASK-005 | Add unit tests for `internal/mode` package | Planned | High | TASK-001 |
| TASK-006 | Introduce interfaces for OS-dependent operations | Planned | High | TASK-001 |
| TASK-007 | Add unit tests for `internal/cc` applier functions | Planned | High | TASK-006 |
| TASK-008 | Add unit tests for `internal/module` and `internal/sysctl` | Planned | High | TASK-006 |
| TASK-009 | Replace `pkg/errors` with `fmt.Errorf` + `%w` | Planned | Medium | TASK-001 |
| TASK-010 | Upgrade Go version to ≥1.22 | Planned | Medium | TASK-009 |
| TASK-011 | Migrate `urfave/cli` v1 → v3 | Planned | Medium | TASK-010 |
| TASK-012 | Migrate `reexec` package to `github.com/moby/sys/reexec` | Planned | Medium | TASK-010 |
| TASK-013 | Add `linux/riscv64` to GoReleaser build matrix | Planned | Low | TASK-010 |
| TASK-014 | Integrate `whydeadcode` analysis | Planned | Low | TASK-010 |
| TASK-015 | Create Dependabot configuration | Planned | Low | TASK-010 |

---

## TASK-001: Add testify dependency and test infrastructure

- **Status**: Done
- **Priority**: High
- **PRD Reference**: Testing Requirements
- **Dependencies**: —
- **Complexity**: Small (S)

### Description

Add `testify` as a test dependency and establish baseline test infrastructure.

### Implementation Checklist

- [x] Add `github.com/stretchr/testify` to `go.mod` as a test dependency
- [x] Verify existing test (`internal/config/read_test.go`) still passes
- [x] Migrate existing test to use `testify/assert` and `testify/require`
- [x] Create a simple smoke test in `internal/version/` to validate testify works
- [x] Verify golangci-lint (no testify-specific config required)
- [x] Verify CI pipeline passes with the new dependency
- [x] Update `docs/status.md`

### Acceptance Criteria

- `testify` is available as a dependency
- Existing test passes with `go test ./...`
- At least one test uses `testify/assert` or `testify/require`
- CI pipeline passes

### Edge Cases / Known Blockers

- Existing test uses `gotest.tools/v3/assert` — confirm it can coexist with testify during migration

---

## TASK-002: Add unit tests for `internal/system` package

- **Status**: Done
- **Priority**: High
- **PRD Reference**: Testing Requirements
- **Dependencies**: TASK-001
- **Complexity**: Small (S)

### Description

The `internal/system` package has pure functions (`RootPath`, `DataPath`, `LocalPath`, `StatePath`) that are trivial to test and will serve as a pattern for other packages.

### Implementation Checklist

- [x] Write tests for `RootPath` with various inputs (empty, single, multiple path elements)
- [x] Write tests for `DataPath` with various inputs
- [x] Write tests for `LocalPath` with various inputs
- [x] Write tests for `StatePath` with various inputs
- [x] Test edge cases: empty string, path with special characters, absolute vs relative
- [x] Achieve ≥90% coverage for `system/system.go`
- [x] Run all tests and verify pass

### Acceptance Criteria

- All path functions tested with multiple scenarios
- Coverage ≥90% for `system/system.go`
- Tests pass on both macOS and Linux (CI)

---

## TASK-003: Add unit tests for `internal/config` (model, write, coerce)

- **Status**: Done
- **Priority**: High
- **PRD Reference**: Testing Requirements
- **Dependencies**: TASK-001
- **Complexity**: Medium (M)

### Description

Test the pure-logic portions of the config package: data model, YAML serialization, and type coercion mappers.

### Implementation Checklist

- [x] Write tests for `CloudConfig` struct initialization and field access
- [x] Write tests for `Debug()` method on `CloudConfig`
- [x] Write tests for `Write()` — serialize config to YAML and verify output
- [x] Write tests for `ToBytes()` — verify Install field is excluded
- [x] Write tests for `PrintInstall()` — verify only install fields are serialized
- [x] Write tests for `toYAMLKeys()` — key conversion from camelCase to yaml_key
- [x] Write tests for type coercion mappers (`NewToMap`, `NewToSlice`, `NewToBool`, `FuzzyNames`)
- [x] Create test fixtures in `internal/config/testdata/`
- [x] Achieve ≥70% coverage for tested files

### Acceptance Criteria

- Config serialization round-trips correctly
- Type coercion handles edge cases (empty strings, unexpected types)
- Test fixtures are reusable for TASK-004

---

## TASK-004: Add unit tests for `internal/config` (read, merge)

- **Status**: Planned
- **Priority**: High
- **PRD Reference**: Testing Requirements
- **Dependencies**: TASK-001, TASK-003
- **Complexity**: Medium (M)

### Description

Test the config reading and merging logic, including multi-source merge, cmdline parsing, and reader chaining.

### Implementation Checklist

- [ ] Write tests for `readCmdline()` — parse kernel command line parameters
- [ ] Write tests for `readFileFunc()` — read YAML from files
- [ ] Write tests for `merge()` — multi-source config merge priority
- [ ] Write tests for `readLocalConfigs()` — config.d directory scanning
- [ ] Write tests for `readersToObject()` — reader chain to CloudConfig conversion
- [ ] Write tests for `mapToEnv()` — config to environment variable conversion
- [ ] Create fixture files for multi-source merge scenarios
- [ ] Achieve ≥60% coverage for `config/read.go`

### Acceptance Criteria

- Config merge priority is verified (system < local < config.d)
- Cmdline parsing handles key=value, key, and quoted values
- Edge cases: missing files, empty config.d, malformed YAML

---

## TASK-005: Add unit tests for `internal/mode` package

- **Status**: Planned
- **Priority**: High
- **PRD Reference**: Testing Requirements
- **Dependencies**: TASK-001
- **Complexity**: Small (S)

### Description

Test boot mode detection from the mode file.

### Implementation Checklist

- [ ] Write test for `Get()` with mode file containing "live"
- [ ] Write test for `Get()` with mode file containing "local"
- [ ] Write test for `Get()` with missing mode file (returns empty string)
- [ ] Write test for `Get()` with prefix parameter
- [ ] Use `t.TempDir()` to create temporary mode files
- [ ] Achieve ≥90% coverage for `mode/mode.go`

### Acceptance Criteria

- All three modes tested (live, local, absent)
- Tests work on macOS and Linux

---

## TASK-006: Introduce interfaces for OS-dependent operations

- **Status**: Planned
- **Priority**: High
- **PRD Reference**: Testing Requirements, Interfaces for Testability
- **Dependencies**: TASK-001
- **Complexity**: Medium (M)

### Description

Introduce interfaces for OS-dependent operations to enable mocking in tests. This is a prerequisite for testing the `cc` applier functions and other OS-interactive code.

### Implementation Checklist

- [ ] Define `Mounter` interface in `internal/mount`
- [ ] Define `ModuleLoader` interface in `internal/module`
- [ ] Define `SysctlApplier` interface in `internal/sysctl`
- [ ] Define `FileWriter` interface for file write operations used by appliers
- [ ] Define `CommandRunner` interface in `internal/command`
- [ ] Refactor `cc/funcs.go` applier functions to accept interfaces (via options pattern or struct injection)
- [ ] Ensure all existing functionality continues to work unchanged
- [ ] Run full test suite and verify all tests pass

### Acceptance Criteria

- Interfaces defined and documented
- Applier functions can accept mock implementations
- No behavioral changes to production code
- All existing tests pass

### Edge Cases / Known Blockers

- The `cc/funcs.go` appliers currently use package-level function calls — will need refactoring to accept dependencies
- Must preserve backward compatibility of public API

---

## TASK-007: Add unit tests for `internal/cc` applier functions

- **Status**: Planned
- **Priority**: High
- **PRD Reference**: Testing Requirements
- **Dependencies**: TASK-006
- **Complexity**: Large (L)

### Description

Test each cloud-config applier function using mock implementations of OS-dependent interfaces.

### Implementation Checklist

- [ ] Write tests for `ApplyModules` — verifies module loading is called correctly
- [ ] Write tests for `ApplySysctls` — verifies sysctl values are written
- [ ] Write tests for `ApplyHostname` — verifies hostname is set
- [ ] Write tests for `ApplyDNS` — verifies connman config is written correctly
- [ ] Write tests for `ApplyWifi` — verifies WiFi config files are generated
- [ ] Write tests for `ApplyPassword` — verifies chpasswd is called
- [ ] Write tests for `ApplySSHKeys` / `ApplySSHKeysWithNet` — verifies key file generation
- [ ] Write tests for `ApplyWriteFiles` — verifies files are written with correct permissions/encoding
- [ ] Write tests for `ApplyEnvironment` — verifies env file is written
- [ ] Write tests for `runApplies` — verifies error aggregation
- [ ] Write tests for `RunApply`, `BootApply`, `InitApply`, `InstallApply` — verifies correct applier chains
- [ ] Achieve ≥60% coverage for `cc/funcs.go` and `cc/apply.go`

### Acceptance Criteria

- Each applier function has at least one passing and one error-case test
- Mock expectations verify correct behavior
- Error aggregation works correctly in `runApplies`

---

## TASK-008: Add unit tests for `internal/module` and `internal/sysctl`

- **Status**: Planned
- **Priority**: High
- **PRD Reference**: Testing Requirements
- **Dependencies**: TASK-006
- **Complexity**: Small (S)

### Description

Test module loading and sysctl application using mock interfaces.

### Implementation Checklist

- [ ] Write tests for `LoadModules` — already loaded modules are skipped
- [ ] Write tests for `LoadModules` — modules with parameters
- [ ] Write tests for `LoadModules` — error handling (missing `/proc/modules`)
- [ ] Write tests for `ConfigureSysctl` — key.value to /proc/sys/key/value path conversion
- [ ] Write tests for `ConfigureSysctl` — error handling (write failure)
- [ ] Write tests for `ConfigureSysctl` — empty sysctls map
- [ ] Achieve ≥80% coverage for both packages

### Acceptance Criteria

- Skip-already-loaded logic verified
- Path conversion for sysctls verified
- Error cases covered

---

## TASK-009: Replace `pkg/errors` with `fmt.Errorf` + `%w`

- **Status**: Planned
- **Priority**: Medium
- **PRD Reference**: Modernization Requirements
- **Dependencies**: TASK-001
- **Complexity**: Small (S)

### Description

Replace all uses of `github.com/pkg/errors` (`errors.Wrap`, `errors.Wrapf`, `errors.New`) with standard Go error wrapping.

### Implementation Checklist

- [ ] Identify all files importing `github.com/pkg/errors`
- [ ] Replace `errors.Wrap(err, msg)` with `fmt.Errorf("%s: %w", msg, err)`
- [ ] Replace `errors.Wrapf(err, fmt, args)` with `fmt.Errorf(fmt + ": %w", args..., err)`
- [ ] Replace `errors.New(msg)` with `fmt.Errorf(msg)` or `errors.New(msg)` (stdlib)
- [ ] Remove `github.com/pkg/errors` from `go.mod`
- [ ] Run `go mod tidy`
- [ ] Run all tests and verify pass
- [ ] Run linter and verify clean

### Acceptance Criteria

- No imports of `github.com/pkg/errors` remain
- All error wrapping uses `fmt.Errorf` with `%w` verb
- All tests pass

---

## TASK-010: Upgrade Go version to ≥1.22

- **Status**: Planned
- **Priority**: Medium
- **PRD Reference**: Modernization Requirements
- **Dependencies**: TASK-009
- **Complexity**: Medium (M)

### Description

Upgrade the Go version in `go.mod` and CI configuration.

### Implementation Checklist

- [ ] Update `go.mod` to `go 1.22` (or latest stable)
- [ ] Update CircleCI config to use matching Go version
- [ ] Update GoReleaser if needed
- [ ] Run `go mod tidy`
- [ ] Run all tests and verify pass
- [ ] Run linter and fix any new warnings from updated linters
- [ ] Verify binary builds and runs correctly

### Acceptance Criteria

- `go.mod` specifies ≥1.22
- CI uses matching Go version
- All tests and lints pass

---

## TASK-011: Migrate `urfave/cli` v1 → v3

- **Status**: Planned
- **Priority**: Medium
- **PRD Reference**: Modernization Requirements
- **Dependencies**: TASK-010
- **Complexity**: Large (L)

### Description

Migrate from `urfave/cli` v1 to v3. This is a significant refactor affecting all CLI command definitions.

### Implementation Checklist

- [ ] Research `urfave/cli` v3 API changes vs v1
- [ ] Update `go.mod` to use `urfave/cli/v3`
- [ ] Migrate `cli/app/app.go` to v3 API
- [ ] Migrate `cli/rc/rc.go` to v3 API
- [ ] Migrate `cli/config/config.go` to v3 API
- [ ] Migrate `cli/install/install.go` to v3 API
- [ ] Migrate `cli/upgrade/upgrade.go` to v3 API
- [ ] Update `cc/apply.go` (uses `cli.NewMultiError`)
- [ ] Run all tests and verify pass
- [ ] Run linter and verify clean
- [ ] Verify binary behavior is identical

### Acceptance Criteria

- All CLI commands work identically to v1 behavior
- No `urfave/cli` v1 imports remain
- All tests pass

### Edge Cases / Known Blockers

- v3 has different flag definition syntax
- `cli.NewMultiError` may have changed
- Symlink-based subcommand dispatch must still work

---

## TASK-012: Migrate `reexec` package to `github.com/moby/sys/reexec`

- **Status**: Planned
- **Priority**: Medium
- **PRD Reference**: Modernization Requirements
- **Dependencies**: TASK-010
- **Complexity**: Small (S)

### Description

Migrate from the deprecated `github.com/moby/moby/pkg/reexec` package to the modern `github.com/moby/sys/reexec` package.

### Implementation Checklist

- [ ] Update `go.mod` to include `github.com/moby/sys/reexec`
- [ ] Replace imports in `main.go` and `internal/cli/app/app.go` (and any other files)
- [ ] Run `go mod tidy`
- [ ] Run all tests and verify pass
- [ ] Verify binary reexec behavior functions identically

### Acceptance Criteria

- No imports of `github.com/moby/moby/pkg/reexec` remain
- All tests pass
- Binary executes subcommands and init modes correctly

---

## TASK-013: Add `linux/riscv64` to GoReleaser build matrix

- **Status**: Planned
- **Priority**: Low
- **PRD Reference**: Future Enhancements
- **Dependencies**: TASK-010
- **Complexity**: Small (S)

### Description

Add RISC-V 64-bit support to the GoReleaser build matrix.

### Implementation Checklist

- [ ] Add `riscv64` to `goarch` list in `.goreleaser.yaml`
- [ ] Verify cross-compilation succeeds locally
- [ ] Test in CI (build only, no runtime test)
- [ ] Update documentation

### Acceptance Criteria

- GoReleaser produces a `linux/riscv64` binary
- CI build succeeds for all architectures

---

## TASK-014: Integrate `whydeadcode` analysis

- **Status**: Planned
- **Priority**: Low
- **PRD Reference**: Future Enhancements
- **Dependencies**: TASK-010
- **Complexity**: Small (S)

### Description

Integrate `https://github.com/aarzilli/whydeadcode` to analyze and prune any unreachable or dead code across the binary.

### Implementation Checklist

- [ ] Install `whydeadcode` tool locally / in CI environment
- [ ] Run `whydeadcode` against the codebase
- [ ] Review reported dead code and verify if it should be pruned or retained
- [ ] Remove confirmed dead code and run full test suite

### Acceptance Criteria

- Codebase is analyzed for dead code
- Unnecessary dead code is pruned without affecting functionality

---

## TASK-015: Create Dependabot configuration

- **Status**: Planned
- **Priority**: Low
- **PRD Reference**: Future Enhancements
- **Dependencies**: TASK-010
- **Complexity**: Small (S)

### Description

Create a `.github/dependabot.yml` configuration file to automate dependency updates for Go modules and GitHub Actions/CircleCI.

### Implementation Checklist

- [ ] Create `.github/dependabot.yml`
- [ ] Configure `gomod` ecosystem with weekly/monthly schedule
- [ ] (Optional) Configure `docker` or `github-actions` ecosystems if applicable
- [ ] Validate configuration file

### Acceptance Criteria

- Valid `dependabot.yml` is present in the repository
