# Implementation Plan: Replace `pkg/errors` with `fmt.Errorf` + `%w`

## Overview

Migrate all `github.com/pkg/errors` call sites to Go standard library equivalents (`fmt.Errorf` with `%w` and `errors.New`), remove the dependency from `go.mod`, and update project documentation. Uses TDD workflow (tests first) with per-package commits on a dedicated feature branch. All Go commands run via Docker on macOS.

## Tasks

- [x] 1. Create feature branch and commit spec
  - [x] 1.1 Create branch `feature/task-009-pkg-errors-migration` from master and commit the spec
    - Run `git checkout -b feature/task-009-pkg-errors-migration`
    - Stage `.kiro/specs/pkg-errors-migration/` directory
    - Commit with message `docs: add pkg-errors-migration spec`
    - _Requirements: 7.4_

- [x] 2. TDD: Write tests for `internal/util` package
  - [x] 2.1 Create `internal/util/prompt_test.go` with error path tests
    - Create test file with `package util` declaration
    - Implement `TestMaskPassword_IOError` — use `os.Pipe()`, close read end, verify returned error wraps the pipe error via `errors.Is`
    - Implement `TestMaskPassword_Interrupt` — write byte `0x03` to pipe, verify error message is `"interrupted"`
    - Implement `TestMaskPassword_MaxLengthExceeded` — write 513+ printable bytes followed by newline, verify error contains `"maximum password length"`
    - Use `github.com/stretchr/testify/assert` and `require`
    - _Requirements: 4.1, 4.2, 4.3, 4.4_
  - [x] 2.2 Write property test for MaskPassword I/O error propagation
    - **Property 2: MaskPassword I/O error propagation**
    - **Validates: Requirements 4.1, 1.1, 1.5**
    - For any error returned by the underlying reader, verify `errors.Is(returnedErr, originalIOErr)` is true
  - [x] 2.3 Run tests via Docker to confirm they pass (or fail only on pre-migration code as expected)
    - Run: `docker run --rm -v "$(pwd)":/app -w /app golang:1.21.9-bookworm go test -v -race ./internal/util/...`
    - Tests for `errors.Is` wrapping may fail until migration is applied — this is expected TDD behavior
    - _Requirements: 4.4, 6.2_

- [x] 3. Migrate `internal/util` and verify
  - [x] 3.1 Apply migration transforms to `internal/util/prompt.go`
    - Replace `errors.Wrapf(err, "failed to set password")` → `fmt.Errorf("failed to set password: %w", err)`
    - Replace `errors.Wrapf(err, "failed to confirm password")` → `fmt.Errorf("failed to confirm password: %w", err)`
    - Replace `errors.New("interrupted")` → `errors.New("interrupted")` (stdlib `errors` package)
    - Replace import `"github.com/pkg/errors"` with `"errors"`
    - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.5_
  - [x] 3.2 Run tests and lint for `internal/util`
    - Run: `docker run --rm -v "$(pwd)":/app -w /app golang:1.21.9-bookworm go test -v -race ./internal/util/...`
    - Run: `golangci-lint run --new-from-rev=master ./internal/util/...`
    - All tests must pass, zero lint findings
    - _Requirements: 6.2, 6.3_
  - [x] 3.3 Commit `internal/util` migration
    - Stage `internal/util/prompt.go` and `internal/util/prompt_test.go`
    - Commit with message `test(util): add MaskPassword error path tests`
    - Then amend or create second commit: `refactor(util): replace pkg/errors with fmt.Errorf + %w`
    - Or combine as: `refactor(util): replace pkg/errors with fmt.Errorf and add tests`
    - _Requirements: 7.1, 7.4_

- [x] 4. Checkpoint - Verify `internal/util` migration
  - Ensure all tests pass, ask the user if questions arise.

- [x] 5. TDD: Write tests for `internal/enterchroot` package
  - [x] 5.1 Extract `procFilesystemsPath` variable and create `internal/enterchroot/enter_test.go`
    - Add `var procFilesystemsPath = "/proc/filesystems"` to `enter.go`
    - Update `inProcFS()` to use `procFilesystemsPath` instead of hardcoded path
    - Create test file with `//go:build linux` tag and `package enterchroot` declaration
    - Implement helper `writeTempFile(t *testing.T, content string) string`
    - Implement `TestInProcFS_WithSquashfs` — temp file contains `"squashfs"`, assert returns true
    - Implement `TestInProcFS_WithoutSquashfs` — temp file without `"squashfs"`, assert returns false
    - Implement `TestCheckSquashfs_ReturnsError_WhenNotSupported` — assert error contains `"squashfs"`
    - Implement `TestCheckSquashfs_ReturnsNil_WhenSupported` — assert no error
    - Use `t.Cleanup` to restore `procFilesystemsPath` after each test
    - _Requirements: 5.1, 5.2, 5.3, 5.4_
  - [x] 5.2 Write property test for inProcFS filesystem detection
    - **Property 3: inProcFS filesystem detection correctness**
    - **Validates: Requirements 5.2, 5.3**
    - For any content string, `inProcFS()` returns true iff content contains `"squashfs"`
  - [x] 5.3 Run tests via Docker to confirm they pass
    - Run: `docker run --rm -v "$(pwd)":/app -w /app golang:1.21.9-bookworm go test -v -race ./internal/enterchroot/...`
    - Tests should pass since `procFilesystemsPath` extraction is a non-breaking refactor
    - _Requirements: 5.4, 6.2_

- [x] 6. Migrate `internal/enterchroot` and verify
  - [x] 6.1 Apply migration transforms to `internal/enterchroot/enter.go`
    - Replace all `errors.Wrap(err, msg)` → `fmt.Errorf("%s: %w", msg, err)` (7 sites)
    - Replace all `errors.Wrapf(err, fmt, args...)` → `fmt.Errorf(fmt + ": %w", args..., err)` (4 sites)
    - Replace `errors.New(msg)` → `errors.New(msg)` from stdlib (2 sites: `"failed to bind mount"` and `checkSquashfs`)
    - Handle chained squashfs wrapping: `err = fmt.Errorf("mounting squashfs: %w", err)` then `err = fmt.Errorf("%s: %w", squashErr.Error(), err)`
    - Replace import `"github.com/pkg/errors"` with `"errors"`
    - _Requirements: 2.1, 2.2, 2.3, 2.5, 2.6_
  - [x] 6.2 Apply migration transforms to `internal/enterchroot/ensureloop.go`
    - Replace `errors.Wrapf(err, "failed to mount proc")` → `fmt.Errorf("failed to mount proc: %w", err)`
    - Replace `errors.Wrapf(err, "failed to mount dev")` → `fmt.Errorf("failed to mount dev: %w", err)`
    - Remove import `"github.com/pkg/errors"`
    - _Requirements: 2.4, 2.5_
  - [x] 6.3 Run tests and lint for `internal/enterchroot`
    - Run: `docker run --rm -v "$(pwd)":/app -w /app golang:1.21.9-bookworm go test -v -race ./internal/enterchroot/...`
    - Run: `golangci-lint run --new-from-rev=master ./internal/enterchroot/...`
    - All tests must pass, zero lint findings
    - _Requirements: 6.2, 6.3_
  - [x] 6.4 Commit `internal/enterchroot` migration
    - Stage `internal/enterchroot/enter.go`, `internal/enterchroot/ensureloop.go`, and `internal/enterchroot/enter_test.go`
    - Commit with message `refactor(enterchroot): replace pkg/errors with fmt.Errorf and add tests`
    - _Requirements: 7.2, 7.4_

- [x] 7. Remove `pkg/errors` from `go.mod` and verify
  - [x] 7.1 Run `go mod tidy` and verify dependency removal
    - Run: `docker run --rm -v "$(pwd)":/app -w /app golang:1.21.9-bookworm go mod tidy`
    - Verify `github.com/pkg/errors` is no longer in `go.mod`
    - Verify `github.com/pkg/errors` is no longer in `go.sum`
    - If a transitive dependency still requires it, document the reason
    - _Requirements: 3.1, 3.2, 3.3_
  - [x] 7.2 Run full test suite
    - Run: `docker run --rm -v "$(pwd)":/app -w /app golang:1.21.9-bookworm go test -race -covermode=atomic -failfast ./...`
    - All tests must pass
    - _Requirements: 6.1, 6.2_
  - [x] 7.3 Run lint on all changes
    - Run: `golangci-lint run --new-from-rev=master`
    - Zero new findings
    - _Requirements: 6.3_
  - [x] 7.4 Commit dependency removal
    - Stage `go.mod` and `go.sum`
    - Commit with message `chore: remove pkg/errors dependency`
    - _Requirements: 7.3, 7.4_

- [x] 8. Checkpoint - Full verification
  - Ensure all tests pass, ask the user if questions arise.

- [x] 9. Update documentation
  - [x] 9.1 Update `docs/status.md`, `docs/log.md`, and `tasks/tasks.md`
    - Mark TASK-009 as done in `docs/status.md`
    - Add log entry to `docs/log.md` describing the migration (date, what was done, packages affected)
    - Mark TASK-009 as done in `tasks/tasks.md`
    - _Requirements: 8.1, 8.2, 8.3_
  - [x] 9.2 Commit documentation updates
    - Stage `docs/status.md`, `docs/log.md`, `tasks/tasks.md`
    - Commit with message `docs: mark TASK-009 pkg/errors migration complete`
    - _Requirements: 7.4, 8.1, 8.2, 8.3_

- [x] 10. Final checkpoint
  - Ensure all tests pass, ask the user if questions arise.

## Notes

- Tasks marked with `*` are optional and can be skipped for faster MVP
- All Go commands (build, test, mod tidy) run via Docker: `docker run --rm -v "$(pwd)":/app -w /app golang:1.21.9-bookworm ...`
- Lint runs natively on macOS: `golangci-lint run --new-from-rev=master`
- TDD workflow: tests are written BEFORE the migration is applied (tasks 2.x before 3.x, tasks 5.x before 6.x)
- The `procFilesystemsPath` variable extraction in task 5.1 is a prerequisite refactor that enables testability
- Do NOT merge back to master — that is out of scope for this task
- Property tests validate universal correctness properties from the design document
- Unit tests validate specific examples and edge cases

## Task Dependency Graph

```json
{
  "waves": [
    { "id": 0, "tasks": ["1.1"] },
    { "id": 1, "tasks": ["2.1"] },
    { "id": 2, "tasks": ["2.2", "2.3"] },
    { "id": 3, "tasks": ["3.1"] },
    { "id": 4, "tasks": ["3.2"] },
    { "id": 5, "tasks": ["3.3"] },
    { "id": 6, "tasks": ["5.1"] },
    { "id": 7, "tasks": ["5.2", "5.3"] },
    { "id": 8, "tasks": ["6.1", "6.2"] },
    { "id": 9, "tasks": ["6.3"] },
    { "id": 10, "tasks": ["6.4"] },
    { "id": 11, "tasks": ["7.1"] },
    { "id": 12, "tasks": ["7.2"] },
    { "id": 13, "tasks": ["7.3"] },
    { "id": 14, "tasks": ["7.4"] },
    { "id": 15, "tasks": ["9.1"] },
    { "id": 16, "tasks": ["9.2"] }
  ]
}
```
