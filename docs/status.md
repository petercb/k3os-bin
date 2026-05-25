# Project Status

## Completed Features

- TASK-001: Add testify dependency and test infrastructure (`github.com/stretchr/testify` v1.11.1, migrated `read_test.go`, added `version` smoke tests)
- TASK-006: Introduce interfaces for OS-dependent operations
- TASK-002: Add unit tests for `internal/system` package
- TASK-003: Add unit tests for `internal/config` (model, write, coerce)
- TASK-004: Add unit tests for `internal/config` (read, merge)
- TASK-005: Add unit tests for `internal/mode` package (7 tests, 100% coverage)
- TASK-007: Add unit tests for `internal/cc` applier functions (31 tests, 93.5% coverage)
- TASK-008: Add integration tests for osimpl module/sysctl adapters, remove dead standalone packages
- TASK-009: Replace `pkg/errors` with `fmt.Errorf` + `%w` (migrated `internal/util` and `internal/enterchroot`, removed dependency from `go.mod`)
- TASK-010: Upgrade Go version to 1.24 (updated `go.mod`, CircleCI, devcontainer; added loop-capture regression test)
- TASK-012: Migrate `reexec` package to `github.com/moby/sys/reexec` (consolidated `/init` + `/sbin/init` into basename-only `"init"` registration)

## In Progress

- Project Onboarding
  - ✅ Codebase analysis and architecture mapping
  - ✅ `docs/PRD.md` created
  - ✅ `docs/technical.md` created
  - ✅ `docs/architecture.mermaid` created
  - ✅ `docs/unit_testing_guideline.md` created
  - ✅ `tasks/tasks.md` created
  - 🏗️ Awaiting user review and approval of onboarding docs

## Pending

- TASK-016: Fix flaky TestFuzzyNames test in internal/config
- TASK-011: Migrate `urfave/cli` v1 → v3
- TASK-013: Add `linux/riscv64` to GoReleaser build matrix
- TASK-014: Integrate `whydeadcode` analysis
- TASK-015: Create Dependabot configuration

## Known Issues

- Near-zero test coverage (only 1 test file: `internal/config/read_test.go`)
- Uses deprecated `github.com/pkg/errors` in several packages
- `urfave/cli` v1 is unmaintained; v3 is the current version
- `rc` package uses `log` (stdlib) instead of `logrus` for consistency

## Process Violations

- 2026-05-23 22:59 - TASK-006 - Verification found TASK-006 marked `Done` while its implementation checklist remained unchecked and changed-package lint still failed. Resolved at 2026-05-23 23:43: checklist updated and scoped changed-package lint/tests pass.

## Decision History

- [2026-05-18] TASK-001 — Added `github.com/stretchr/testify` v1.11.1 as a direct `go.mod` dependency. Migrated `TestAuthorizedKeys` with preserved behavior (`len == 1`); error message still references "expected 2" for investigation in TASK-004. No golangci-lint config changes required.
- [2026-05-17] PROJECT-ONBOARDING — Chose `testify` over `gotest.tools` for testing framework. Rationale: richer assertion API, built-in mocking support, wider community adoption.
- [2026-05-17] PROJECT-ONBOARDING — Chose `master`-only branching (no `develop` branch). Rationale: project owner preference; simpler workflow.
- [2026-05-17] PROJECT-ONBOARDING — Decided to introduce interfaces for OS-dependent operations to enable mocking. Alternatives considered: build-tag-based test stubs (rejected: more complex, less flexible).
- [2026-05-17] PROJECT-ONBOARDING — Decided to keep existing test file location convention (alongside source) rather than a root-level `/tests` directory. Rationale: follows Go conventions; package-level tests are idiomatic Go.

## Next Steps

- TASK-016: Fix flaky TestFuzzyNames test in internal/config (High priority, unblocked)
- TASK-011: Migrate `urfave/cli` v1 → v3 (Medium priority, unblocked)
