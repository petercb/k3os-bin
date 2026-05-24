# Requirements Document

## Introduction

Replace all usages of the third-party `github.com/pkg/errors` package with Go standard library equivalents (`fmt.Errorf` with `%w` verb and `errors.New`). This eliminates a deprecated dependency, aligns the codebase with modern Go error-wrapping idioms, and removes the package from `go.mod`. The migration covers three source files across two packages (`internal/enterchroot` and `internal/util`), uses TDD workflow, and produces per-package commits on a dedicated feature branch.

## Glossary

- **Migration_Tool**: The set of manual code transformations applied to replace `pkg/errors` call sites
- **Build_System**: The Go toolchain (`go build`, `go test`, `go mod tidy`) executed via Docker on macOS
- **Linter**: `golangci-lint` configured per `.golangci.yaml`, run with `--new-from-rev=master`
- **Enterchroot_Package**: The `internal/enterchroot` package containing `enter.go` and `ensureloop.go`
- **Util_Package**: The `internal/util` package containing `prompt.go`
- **Error_Chain**: The linked list of wrapped errors traversable via `errors.Is` and `errors.As`
- **Feature_Branch**: The Git branch `feature/task-009-pkg-errors-migration`

## Requirements

### Requirement 1: Migrate `internal/util` package

**User Story:** As a maintainer, I want `internal/util/prompt.go` to use only stdlib error handling, so that the codebase has no remaining `pkg/errors` imports in this package.

#### Acceptance Criteria

1. WHEN the Migration_Tool is applied to `internal/util/prompt.go`, THE Util_Package SHALL replace each `errors.Wrapf(err, fmt, args...)` call with `fmt.Errorf(fmt + ": %w", args..., err)`
2. WHEN the Migration_Tool is applied to `internal/util/prompt.go`, THE Util_Package SHALL replace each `errors.New(msg)` call with `errors.New(msg)` from the `errors` standard library package
3. WHEN the migration is complete, THE Util_Package SHALL contain zero import references to `github.com/pkg/errors`
4. WHEN the migration is complete, THE Util_Package SHALL preserve the original error message text for each wrapped error (excluding the `: ` separator appended by `%w`)
5. WHEN a wrapped error is returned from `PromptPassword` or `MaskPassword`, THE Error_Chain SHALL be unwrappable via `errors.Is` and `errors.As` to reach the original cause

### Requirement 2: Migrate `internal/enterchroot` package

**User Story:** As a maintainer, I want `internal/enterchroot` to use only stdlib error handling, so that the codebase has no remaining `pkg/errors` imports in this package.

#### Acceptance Criteria

1. WHEN the Migration_Tool is applied to `enter.go`, THE Enterchroot_Package SHALL replace each `errors.Wrap(err, msg)` call with `fmt.Errorf("%s: %w", msg, err)`
2. WHEN the Migration_Tool is applied to `enter.go`, THE Enterchroot_Package SHALL replace each `errors.Wrapf(err, fmt, args...)` call with `fmt.Errorf(fmt + ": %w", args..., err)`
3. WHEN the Migration_Tool is applied to `enter.go`, THE Enterchroot_Package SHALL replace each `errors.New(msg)` call with `errors.New(msg)` from the `errors` standard library package
4. WHEN the Migration_Tool is applied to `ensureloop.go`, THE Enterchroot_Package SHALL replace each `errors.Wrapf(err, fmt, args...)` call with `fmt.Errorf(fmt + ": %w", args..., err)`
5. WHEN the migration is complete, THE Enterchroot_Package SHALL contain zero import references to `github.com/pkg/errors`
6. WHEN a wrapped error is returned from any function in the Enterchroot_Package, THE Error_Chain SHALL be unwrappable via `errors.Is` and `errors.As` to reach the original cause

### Requirement 3: Remove dependency from module

**User Story:** As a maintainer, I want `github.com/pkg/errors` removed from `go.mod`, so that the project carries no unused dependencies.

#### Acceptance Criteria

1. WHEN all source migrations are complete, THE Build_System SHALL remove `github.com/pkg/errors` from `go.mod` via `go mod tidy`
2. WHEN `go mod tidy` completes, THE Build_System SHALL produce a `go.sum` file with no references to `github.com/pkg/errors`
3. IF `go mod tidy` fails due to a transitive dependency still requiring `pkg/errors`, THEN THE Build_System SHALL retain the indirect dependency and document the reason

### Requirement 4: Test coverage for `internal/util` error paths

**User Story:** As a maintainer, I want unit tests covering `prompt.go` error paths, so that the migration is verified and regressions are caught.

#### Acceptance Criteria

1. THE Util_Package SHALL have test functions that verify `MaskPassword` returns a wrapped error when the reader produces an I/O error
2. THE Util_Package SHALL have test functions that verify `MaskPassword` returns an `errors.New("interrupted")` error when Ctrl+C (byte 3) is received
3. THE Util_Package SHALL have test functions that verify `MaskPassword` returns a max-length error when input exceeds 512 bytes
4. WHEN tests are written, THE Util_Package tests SHALL use `errors.Is` or string matching to confirm error wrapping preserves the cause

### Requirement 5: Test coverage for `internal/enterchroot` helpers

**User Story:** As a maintainer, I want unit tests for `enterchroot` helpers that do not require root or Linux, so that the migration is verified on any platform.

#### Acceptance Criteria

1. THE Enterchroot_Package SHALL have a test function that verifies `checkSquashfs` returns an error containing "squashfs" when `/proc/filesystems` does not list squashfs
2. THE Enterchroot_Package SHALL have a test function that verifies `inProcFS` returns false when `/proc/filesystems` does not contain "squashfs"
3. THE Enterchroot_Package SHALL have a test function that verifies `inProcFS` returns true when `/proc/filesystems` contains "squashfs"
4. WHEN tests target Linux-only code, THE Enterchroot_Package tests SHALL use `//go:build linux` build tags

### Requirement 6: Build and lint verification

**User Story:** As a maintainer, I want the migrated code to pass build and lint checks, so that CI remains green.

#### Acceptance Criteria

1. WHEN the migration is complete, THE Build_System SHALL compile the project without errors using `go build ./...`
2. WHEN the migration is complete, THE Build_System SHALL pass all tests using `go test -race -covermode=atomic -failfast ./...` via Docker
3. WHEN the migration is complete, THE Linter SHALL report zero new findings when run with `golangci-lint run --new-from-rev=master`

### Requirement 7: Per-package commit granularity

**User Story:** As a maintainer, I want each package migration in its own commit, so that changes are reviewable and individually revertable.

#### Acceptance Criteria

1. THE Feature_Branch SHALL contain a separate commit for the `internal/util` package migration (source changes and tests)
2. THE Feature_Branch SHALL contain a separate commit for the `internal/enterchroot` package migration (source changes and tests)
3. THE Feature_Branch SHALL contain a separate commit for the `go.mod` and `go.sum` dependency removal
4. WHEN commits are created, THE Feature_Branch SHALL use conventional commit format with type `refactor` for source changes and type `test` for test additions

### Requirement 8: Documentation updates

**User Story:** As a maintainer, I want project tracking documents updated to reflect the completed migration, so that status is visible to the team.

#### Acceptance Criteria

1. WHEN the migration is complete, THE Feature_Branch SHALL contain an update to `docs/status.md` marking TASK-009 as done
2. WHEN the migration is complete, THE Feature_Branch SHALL contain an update to `docs/log.md` with an entry describing the migration
3. WHEN the migration is complete, THE Feature_Branch SHALL contain an update to `tasks/tasks.md` marking TASK-009 as done
