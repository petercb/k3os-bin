# Execution Plan: TASK-001 — Add testify dependency and test infrastructure

## Task Summary

| Field | Value |
|-------|-------|
| **Task ID** | TASK-001 |
| **Title** | Add testify dependency and test infrastructure |
| **Status** | Planned |
| **Priority** | High |
| **Dependencies** | None |
| **Complexity** | Small (S) |
| **Branch** | `feature/task-001-testify-infrastructure` (from `master`) |

---

## Current State Analysis

### go.mod

- **Go version**: 1.21.9
- **`gotest.tools/v3`** is already listed as an `// indirect` dependency (v3.3.0) — pulled in transitively by `github.com/moby/moby`
- **`testify` is NOT present** in go.mod

### Existing Test: [read_test.go](file:///Users/pburns/git/k3os-bin/internal/config/read_test.go)

- 2 test functions: `TestDataSource`, `TestAuthorizedKeys`
- Uses **only stdlib `testing`** — no `gotest.tools/v3` assertions are actually used in test code
- Uses `t.Fatal` / `t.Fatalf` for assertions (old-style, verbose)
- `TestAuthorizedKeys` has a **suspicious bug**: checks `len(cc.SSHAuthorizedKeys) != 1` with message "expected 2" — this may be an existing logic issue or intentional (merge replaces rather than appends)

### golangci-lint: [.golangci.yaml](file:///Users/pburns/git/k3os-bin/.golangci.yaml)

- v2 config format
- 9 linters enabled, 2 formatters (`gofumpt`, `goimports`)
- No testify-specific configuration present (none needed for basic usage)

### Version package: [version.go](file:///Users/pburns/git/k3os-bin/internal/version/version.go)

- Extremely simple: `var Version = "HEAD"` (4 lines total)
- Ideal candidate for a smoke test to validate testify works

### Branching

- Project uses `master`-only model (Decision History in status.md)
- Feature branch will be `feature/task-001-testify-infrastructure` from `master`

---

## Pre-Implementation Checklist

- [ ] Confirm on `master` branch: `git checkout master`
- [ ] Create feature branch: `git checkout -b feature/task-001-testify-infrastructure`
- [ ] Confirm test command works: `GOOS=linux go test ./...`
- [ ] Confirm linter works: `GOOS=linux golangci-lint run ./...`

---

## Implementation Steps

### Step 1: Add `github.com/stretchr/testify` to `go.mod`

**What**: Add testify as a direct test dependency.

**How**:

```bash
cd /Users/pburns/git/k3os-bin
go get github.com/stretchr/testify@latest
go mod tidy
```

**Verification**:

```bash
grep "stretchr/testify" go.mod  # Should show as a direct require
```

**Files changed**: `go.mod`, `go.sum`

> [!NOTE]
> `go get` will add testify as a direct dependency. Running `go mod tidy` will clean up the dependency graph. testify v1.10.x is the current latest.

---

### Step 2: Verify existing test (`internal/config/read_test.go`) still passes

**What**: Confirm the existing `TestDataSource` and `TestAuthorizedKeys` tests still pass after adding the testify dependency.

**How**:

```bash
GOOS=linux go test -v ./internal/config/...
```

**Expected**: Both tests pass. No code changes needed.

**Files changed**: None

> [!WARNING]
> `TestAuthorizedKeys` asserts `len(cc.SSHAuthorizedKeys) != 1` with the message "expected 2". This seems like a pre-existing bug (the message says "expected 2" but the check asserts "not 1"). This should be flagged but **not fixed** in this task — it's existing behavior. We may want to address it when migrating to testify assertions in the next step.

---

### Step 3: Migrate existing test to use `testify/assert` and `testify/require`

**What**: Rewrite `internal/config/read_test.go` to use testify assertions instead of raw `t.Fatal`/`t.Fatalf`.

**TDD approach**: This is a migration of existing tests, not new functionality. The tests should produce the same pass/fail results before and after.

**Before** (current):

```go
if err != nil {
    t.Fatal(err)
}
if len(cc.K3OS.DataSources) != 1 {
    t.Fatal("no datasources")
}
```

**After** (testify):

```go
require.NoError(t, err)
require.Len(t, cc.K3OS.DataSources, 1, "expected exactly one datasource")
assert.Equal(t, "foo", cc.K3OS.DataSources[0])
```

**Files changed**: `internal/config/read_test.go`

**Verification**:

```bash
GOOS=linux go test -v ./internal/config/...
```

> [!IMPORTANT]
> **Decision needed**: The `TestAuthorizedKeys` test checks `len(cc.SSHAuthorizedKeys) != 1` but the error message says "expected 2". During migration, should we:
>
> - **(A)** Preserve exact current behavior (assert len == 1)?
> - **(B)** Fix to match the message (assert len == 2)?
>
> Option A is safer and preserves existing semantics. We can investigate the merge behavior in TASK-004.

---

### Step 4: Create a smoke test in `internal/version/` to validate testify works

**What**: Write a simple test for the `version` package using testify to prove the dependency is correctly wired.

**TDD approach**:

1. **Write failing test first**: Create `internal/version/version_test.go` with a test that uses `assert.Equal` and `require.NotEmpty`
2. **Run test**: Should pass immediately (testing existing behavior, not new code)
3. **Verify**: testify assertions work correctly in this package

**Test file**: `internal/version/version_test.go`

```go
package version

import (
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestVersion_DefaultValue(t *testing.T) {
    require.NotEmpty(t, Version, "Version should not be empty")
    assert.Equal(t, "HEAD", Version, "Default version should be HEAD when not set by ldflags")
}

func TestVersion_IsString(t *testing.T) {
    // Validates that Version is a settable string variable
    // (it gets overridden via -ldflags at build time)
    original := Version
    defer func() { Version = original }()

    Version = "v1.0.0-test"
    assert.Equal(t, "v1.0.0-test", Version)
}
```

**Verification**:

```bash
GOOS=linux go test -v ./internal/version/...
```

**Files changed**: `internal/version/version_test.go` (new file)

> [!TIP]
> This smoke test also validates that testify imports resolve correctly and the test runner works with the new dependency.

---

### Step 5: Verify golangci-lint configuration

**What**: Confirm golangci-lint runs clean with testify in the codebase. No configuration changes are expected to be needed — testify doesn't require any special lint rules.

**How**:

```bash
GOOS=linux golangci-lint run ./...
```

**Decision**: If lint issues arise from the testify migration or new test file, fix them. If not, this step is a verification pass.

**Files changed**: `.golangci.yaml` (only if lint issues arise)

> [!NOTE]
> The current `.golangci.yaml` already has `gofumpt` and `goimports` formatters enabled, which will auto-format the new test files correctly.

---

### Step 6: Verify CI pipeline passes

**What**: Confirm the full CI test and build commands succeed locally.

**How**:

```bash
# Full test suite (CI equivalent)
GOOS=linux go test -race -covermode=atomic -failfast ./...

# Build verification
GOOS=linux CGO_ENABLED=0 go build -o /dev/null .

# Lint verification
GOOS=linux golangci-lint run ./...
```

**Files changed**: None (verification only)

---

### Step 7: Update `docs/status.md`

**What**: Update project status to reflect TASK-001 completion.

**Changes**:

- Move TASK-001 from "Pending" to "Completed Features"
- Update "In Progress" section
- Add decision history entry for testify version
- Update "Next Steps"

**Files changed**: `docs/status.md`

---

## File Change Manifest

| File | Action | Description |
|------|--------|-------------|
| `go.mod` | Modified | Add `github.com/stretchr/testify` as direct dependency |
| `go.sum` | Modified | Updated dependency checksums |
| `internal/config/read_test.go` | Modified | Migrate to testify assert/require |
| `internal/version/version_test.go` | **New** | Smoke test validating testify works |
| `docs/status.md` | Modified | Task completion status update |
| `docs/log.md` | Modified | Activity log entry |
| `tasks/tasks.md` | Modified | Checklist items marked done, status → Done |

---

## Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| `gotest.tools/v3` conflicts with testify | Low | Low | `gotest.tools/v3` is indirect only; no code imports it directly. Coexistence is safe. |
| Existing `TestAuthorizedKeys` has a latent bug | Medium | Low | Preserve existing behavior during migration; investigate in TASK-004 |
| `GOOS=linux` required for tests | Known | None | Already documented; macOS dev machines use `GOOS=linux` prefix for `go test` |
| `go mod tidy` removes indirect deps | Low | Low | Verify `go.sum` diff is reasonable before committing |

---

## Estimated Effort

| Step | Time |
|------|------|
| Step 1: Add testify | ~2 min |
| Step 2: Verify existing tests | ~1 min |
| Step 3: Migrate read_test.go | ~10 min |
| Step 4: Create version smoke test | ~5 min |
| Step 5: Lint verification | ~2 min |
| Step 6: CI verification | ~3 min |
| Step 7: Status update | ~5 min |
| **Total** | **~28 min** |

---

## Acceptance Verification Commands

```bash
# 1. testify is in go.mod as a direct dependency
grep 'stretchr/testify' go.mod | grep -v indirect

# 2. All tests pass
GOOS=linux go test ./...

# 3. At least one test uses testify
grep -r 'testify/assert\|testify/require' internal/

# 4. CI equivalent passes
GOOS=linux go test -race -covermode=atomic -failfast ./...
GOOS=linux golangci-lint run ./...
```

---

## Open Questions for Review

1. **`TestAuthorizedKeys` behavior**: The existing test asserts `len == 1` but the error message says "expected 2". Should we preserve exact behavior (A) or fix the message/assertion (B) during migration?

2. **`gotest.tools/v3` cleanup**: Currently an indirect dependency via `moby/moby`. Should we explicitly exclude it or leave it as-is? (It causes no harm as indirect.)

3. **Ready to proceed?** If this plan is approved, I'll start execution with the TDD_ENFORCEMENT workflow — creating the feature branch from `master` and working through each step sequentially.
