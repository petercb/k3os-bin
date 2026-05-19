# Execution Plan: TASK-002 — Add unit tests for `internal/system` package

## Task Summary

| Field | Value |
|-------|-------|
| **Task ID** | TASK-002 |
| **Title** | Add unit tests for `internal/system` package |
| **Status** | Planned |
| **Priority** | High |
| **Dependencies** | TASK-001 |
| **Complexity** | Small (S) |
| **Branch** | `feature/task-002-system-tests` (from `master`) |

---

## Current State Analysis

### `internal/system/system.go`

- The `system` package provides consistent paths used throughout `k3os-bin`.
- Contains 4 exported functions: `RootPath`, `DataPath`, `LocalPath`, and `StatePath`.
- These are pure functions utilizing `filepath.Join` against package-level default directories.
- They accept variadic `elem ...string`.
- No tests currently exist for this package.

### Testing Guidelines Alignment

- **Category**: Pure Logic Tests (no OS dependencies).
- **Target Coverage**: ≥80% (aiming for 100% since it's trivial).
- **Pattern**: Table-Driven Tests.
- **Tools**: `testing` (stdlib), `testify/assert`.

---

## Pre-Implementation Checklist

- [ ] Confirm on `master` branch: `git checkout master`
- [ ] Create feature branch: `git checkout -b feature/task-002-system-tests`
- [ ] Ensure Docker is available for local testing on macOS/Windows.

---

## Implementation Steps

### Step 1: Create table-driven tests for `RootPath`

**What**: Add a test function `TestRootPath` using table-driven tests.

**TDD approach**:

1. **Write failing test first**: Create `internal/system/system_test.go` and implement `TestRootPath` using table-driven tests checking against various scenarios (empty args, single arg, multiple args, absolute path arg).
2. **Run test**: Verify it fails or passes. (It should pass immediately as we are writing tests for existing logic).
3. **Verify**: Test coverage.

**Scenarios**:

- No arguments -> `/k3os/system`
- Single argument (`"foo"`) -> `/k3os/system/foo`
- Multiple arguments (`"foo"`, `"bar"`) -> `/k3os/system/foo/bar`
- Absolute path argument (`"/foo"`) -> `/k3os/system/foo` (due to `filepath.Join` behavior)

### Step 2: Create tests for `DataPath`, `LocalPath`, and `StatePath`

**What**: Add test functions `TestDataPath`, `TestLocalPath`, and `TestStatePath`.

**How**: Use the exact same table-driven pattern, just substituting the target function and the expected prefix.

- `DataPath` prefix: `/k3os/data`
- `LocalPath` prefix: `/var/lib/rancher/k3os`
- `StatePath` prefix: `/run/k3os`

**Files changed**: `internal/system/system_test.go`

### Step 3: Run all tests and check coverage

**What**: Ensure tests pass and coverage is 100% for `internal/system/system.go`.

**How**:

```bash
# Run tests
docker run --rm -v "$(pwd)":/app -w /app golang:1.21.9-bookworm go test -v ./internal/system/...

# Check coverage
docker run --rm -v "$(pwd)":/app -w /app golang:1.21.9-bookworm go test -coverprofile=coverage.out ./internal/system/...
docker run --rm -v "$(pwd)":/app -w /app golang:1.21.9-bookworm go tool cover -func=coverage.out
```

### Step 4: Verify CI pipeline locally

**What**: Confirm full CI equivalent passes.

**How**:

```bash
docker run --rm -v "$(pwd)":/app -w /app golang:1.21.9-bookworm go test -race -covermode=atomic -failfast ./...
docker run --rm -v "$(pwd)":/app -w /app golang:1.21.9-bookworm sh -c 'go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest && golangci-lint run ./...'
```

### Step 5: Update project tracking documents

**What**: Update statuses to reflect completion.

**Changes**:

- Update `tasks/tasks.md`: Mark all checkboxes in TASK-002, change status to Done.
- Update `docs/status.md`: Move TASK-002 to Completed Features.
- Update `docs/log.md`: Log the work done and a brief retrospective.
- Create task summary at `tasks/TASK-002.md`.

---

## File Change Manifest

| File | Action | Description |
|------|--------|-------------|
| `internal/system/system_test.go` | **New** | Test suite for path functions |
| `docs/status.md` | Modified | Task completion status update |
| `docs/log.md` | Modified | Activity log entry |
| `tasks/tasks.md` | Modified | Checklist items marked done, status → Done |
| `tasks/TASK-002.md` | **New** | Completed task summary |

---

## Estimated Effort

| Step | Time |
|------|------|
| Step 1: RootPath tests | ~5 min |
| Step 2: Other path tests | ~5 min |
| Step 3: Coverage check | ~2 min |
| Step 4: CI verification | ~3 min |
| Step 5: Status update | ~5 min |
| **Total** | **~20 min** |
