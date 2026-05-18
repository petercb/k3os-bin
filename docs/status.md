# Project Status

## Completed Features

- (none yet — project onboarding in progress)

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

- TASK-001: Add testify dependency and test infrastructure
- TASK-002: Add unit tests for `internal/system` package
- TASK-003: Add unit tests for `internal/config` (model, write, coerce)
- TASK-004: Add unit tests for `internal/config` (read, merge)
- TASK-005: Add unit tests for `internal/mode` package
- TASK-006: Introduce interfaces for OS-dependent operations
- TASK-007: Add unit tests for `internal/cc` applier functions
- TASK-008: Add unit tests for `internal/module` and `internal/sysctl`
- TASK-009: Replace `pkg/errors` with `fmt.Errorf` + `%w`
- TASK-010: Upgrade Go version to ≥1.22
- TASK-011: Migrate `urfave/cli` v1 → v3
- TASK-012: Migrate `reexec` package to `github.com/moby/sys/reexec`
- TASK-013: Add `linux/riscv64` to GoReleaser build matrix
- TASK-014: Integrate `whydeadcode` analysis
- TASK-015: Create Dependabot configuration

## Known Issues

- Near-zero test coverage (only 1 test file: `internal/config/read_test.go`)
- Uses deprecated `github.com/pkg/errors` in several packages
- `urfave/cli` v1 is unmaintained; v3 is the current version
- Uses deprecated `github.com/moby/moby/pkg/reexec`
- `rc` package uses `log` (stdlib) instead of `logrus` for consistency
- Go 1.21.9 is nearing end of support

## Decision History

- [2026-05-17] PROJECT-ONBOARDING — Chose `testify` over `gotest.tools` for testing framework. Rationale: richer assertion API, built-in mocking support, wider community adoption.
- [2026-05-17] PROJECT-ONBOARDING — Chose `master`-only branching (no `develop` branch). Rationale: project owner preference; simpler workflow.
- [2026-05-17] PROJECT-ONBOARDING — Decided to introduce interfaces for OS-dependent operations to enable mocking. Alternatives considered: build-tag-based test stubs (rejected: more complex, less flexible).
- [2026-05-17] PROJECT-ONBOARDING — Decided to keep existing test file location convention (alongside source) rather than a root-level `/tests` directory. Rationale: follows Go conventions; package-level tests are idiomatic Go.

## Next Steps

- Review and approve onboarding docs (PRD, technical, architecture, testing guidelines)
- Review and approve task plan (`tasks/tasks.md`)
