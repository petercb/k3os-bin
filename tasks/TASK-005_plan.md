# Execution Plan: TASK-005 — Add unit tests for `internal/mode` package

## Task Summary

| Field | Value |
|-------|-------|
| **Task ID** | TASK-005 |
| **Title** | Add unit tests for `internal/mode` package |
| **Status** | Planned |
| **Priority** | High |
| **Dependencies** | TASK-001 |
| **Complexity** | Small (S) |
| **Branch** | `feature/task-005-mode-tests` (from `develop`) |

---

## Current State Analysis

### `internal/mode/mode.go`

- Contains a single pure function `Get(prefix ...string) (string, error)`.
- It reads the file at `<prefix>/<system.StatePath("mode")>`.
- Returns `""` without error if the file does not exist (`os.IsNotExist`).
- Returns the trimmed string content of the file if it exists.
- Returns an error for other file read failures.

### Testing Guidelines Alignment

- **Category**: Pure Logic Tests.
- **Target Coverage**: ≥90% for `mode/mode.go`.
- **Tools**: `testing` (stdlib), `testify/assert`, `testify/require`.
- **Fixtures**: We will use `t.TempDir()` to dynamically create the required directory structure `system.StatePath("mode")` implies, and write test files instead of static fixtures.

---

## Pre-Implementation Checklist

- [ ] Confirm on `develop` branch: `git checkout develop`
- [ ] Create feature branch: `git checkout -b feature/task-005-mode-tests`
- [ ] Ensure local Go environment or Docker is available for testing.

---

## Implementation Steps

### Step 1: Tests for `Get()` (`mode_test.go`)

**What**: Test the `Get()` function with various mode file states.
**TDD approach**:

1. Write a helper function/logic in the test to scaffold `t.TempDir()` and create the expected path from `system.StatePath("mode")`.
2. Write test for `Get()` with mode file containing `"live"`. Check for `"live"` return value.
3. Write test for `Get()` with mode file containing `"local"`. Check for `"local"` return value.
4. Write test for `Get()` with whitespace like `"  live  \n"`. Check that it trims space and returns `"live"`.
5. Write test for `Get()` with a missing mode file. Check that it returns `""` and no error.
6. Write test for `Get()` with multiple prefix parameters to verify `filepath.Join(prefix...)` logic works.

### Step 2: Run all tests and check coverage

**What**: Ensure tests pass and coverage is ≥90% for `mode.go`.
**How**:

```bash
go test -v ./internal/mode/...
go test -coverprofile=coverage.out ./internal/mode/...
go tool cover -func=coverage.out
```

### Step 3: Update project tracking documents

**What**: Update statuses to reflect completion of TASK-005.
**Changes**:

- Update `tasks/tasks.md`: Mark all checkboxes in TASK-005, change status to Done.
- Update `docs/status.md`: Move TASK-005 to Completed Features.
- Update `docs/log.md`: Log the work done and a brief retrospective.
- Create task summary at `tasks/TASK-005.md`.

---

## File Change Manifest

| File | Action | Description |
|------|--------|-------------|
| `internal/mode/mode_test.go` | **New** | Test suite for boot mode detection |
| `docs/status.md` | Modified | Task completion status update |
| `docs/log.md` | Modified | Activity log entry |
| `tasks/tasks.md` | Modified | Checklist items marked done, status → Done |
| `tasks/TASK-005.md` | **New** | Completed task summary |

---

## Estimated Effort

| Step | Time |
|------|------|
| Step 1: `Get()` tests | ~15 min |
| Step 2: Coverage verification | ~5 min |
| Step 3: Status update | ~5 min |
| **Total** | **~25 min** |
