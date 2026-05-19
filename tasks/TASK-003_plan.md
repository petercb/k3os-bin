# Execution Plan: TASK-003 — Add unit tests for `internal/config` (model, write, coerce)

## Task Summary

| Field | Value |
|-------|-------|
| **Task ID** | TASK-003 |
| **Title** | Add unit tests for `internal/config` (model, write, coerce) |
| **Status** | Planned |
| **Priority** | High |
| **Dependencies** | TASK-001 |
| **Complexity** | Medium (M) |
| **Branch** | `feature/task-003-config-tests` (from `master`) |

---

## Current State Analysis

### `internal/config/config.go`

- Contains data models: `K3OS`, `Wifi`, `Install`, `CloudConfig`, and `File`.
- Contains one method: `Permissions()` on `File`.
*(Note: The task checklist mentions a `Debug()` method on `CloudConfig`, but this does not exist in the current codebase. We will test the `Debug` bool field under `Install`, and the `Permissions()` method which is the actual method present.)*

### `internal/config/write.go`

- Contains `PrintInstall()`, `Write()`, `ToBytes()`, and a private `toYAMLKeys()` function.
- These handle YAML serialization and stripping the `Install` field for non-install serialization.

### `internal/config/coerce.go` and `internal/config/rename.go`

- Handle type coercion (`NewToMap`, `NewToSlice`, `NewToBool`) and field renaming (`FuzzyNames`).
- These rely on the `rancher/mapper` package to mutate parsed config values.

### Testing Guidelines Alignment

- **Category**: Pure Logic Tests (no OS dependencies).
- **Target Coverage**: ≥80% for these specific files (overall `internal/config` might be lower until `read.go` is tested in TASK-004).
- **Tools**: `testing` (stdlib), `testify/assert`, `testify/require`.
- **Fixtures**: We may need to create `testdata/` for some serialization outputs to assert against known good YAML strings.

---

## Pre-Implementation Checklist

- [ ] Confirm on `master` branch: `git checkout master`
- [ ] Create feature branch: `git checkout -b feature/task-003-config-tests`
- [ ] Ensure Docker is available for local testing on macOS/Windows.

---

## Implementation Steps

### Step 1: Tests for Data Models (`config_test.go`)

**What**: Test initialization, field access, and methods of the structs in `config.go`.
**TDD approach**:

1. Write test for `CloudConfig` struct initialization and nested fields.
2. Write test for `File.Permissions()` method (should handle valid octal strings, empty strings, and invalid strings).

### Step 2: Tests for Serialization (`write_test.go`)

**What**: Test the YAML serialization functions in `write.go`.
**TDD approach**:

1. Write tests for `ToBytes()`: ensure `Install` field is omitted from the resulting YAML.
2. Write tests for `Write()`: verify it correctly writes `ToBytes()` output to an `io.Writer` (e.g. `bytes.Buffer`).
3. Write tests for `PrintInstall()`: verify it only serializes the `Install` fields and ignores others.
4. Write tests for `toYAMLKeys()`: check recursive key conversion from `camelCase` to `yaml_key`.

### Step 3: Tests for Coercion and Renaming (`coerce_test.go` and `rename_test.go`)

**What**: Test the type converters in `coerce.go` and `FuzzyNames` in `rename.go`.
**TDD approach**:

1. `NewToMap`: test conversion of `map[string]interface{}` to `map[string]string`.
2. `NewToSlice`: test conversion of a single `string` into a `[]string`.
3. `NewToBool`: test conversion of the string `"true"` to boolean `true`.
4. `FuzzyNames`: test that plural/singular variants (e.g., `pass` vs `passphrase`, `sshAuthorizedKey` vs `sshAuthorizedKeys`) resolve to the correct internal names according to `ModifySchema`.

### Step 4: Run all tests and check coverage

**What**: Ensure tests pass and coverage is ≥80% for the targeted logic files (`config.go`, `write.go`, `coerce.go`, `rename.go`).
**How**:

```bash
docker run --rm -v "$(pwd)":/app -w /app golang:1.21.9-bookworm go test -v ./internal/config/...
docker run --rm -v "$(pwd)":/app -w /app golang:1.21.9-bookworm go test -coverprofile=coverage.out ./internal/config/...
docker run --rm -v "$(pwd)":/app -w /app golang:1.21.9-bookworm go tool cover -func=coverage.out
```

### Step 5: Update project tracking documents

**What**: Update statuses to reflect completion of TASK-003.
**Changes**:

- Update `tasks/tasks.md`: Mark all checkboxes in TASK-003, change status to Done.
- Update `docs/status.md`: Move TASK-003 to Completed Features.
- Update `docs/log.md`: Log the work done and a brief retrospective.
- Create task summary at `tasks/TASK-003.md`.

---

## File Change Manifest

| File | Action | Description |
|------|--------|-------------|
| `internal/config/config_test.go` | **New** | Test suite for data models |
| `internal/config/write_test.go` | **New** | Test suite for serialization |
| `internal/config/coerce_test.go` | **New** | Test suite for coercion rules |
| `internal/config/rename_test.go` | **New** | Test suite for fuzzy name mapping |
| `internal/config/testdata/` | **New** | Test fixtures for YAML serialization |
| `docs/status.md` | Modified | Task completion status update |
| `docs/log.md` | Modified | Activity log entry |
| `tasks/tasks.md` | Modified | Checklist items marked done, status → Done |
| `tasks/TASK-003.md` | **New** | Completed task summary |

---

## Estimated Effort

| Step | Time |
|------|------|
| Step 1: Model tests | ~10 min |
| Step 2: Serialization tests | ~15 min |
| Step 3: Coercion tests | ~15 min |
| Step 4: Coverage verification | ~5 min |
| Step 5: Status update | ~5 min |
| **Total** | **~50 min** |
