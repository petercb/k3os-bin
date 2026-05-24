# Unit Testing Guidelines — k3os-bin

## Overview

This document defines the testing strategy, conventions, and tooling for the `k3os-bin` project. The goal is to achieve ≥60% unit test coverage across `internal/` packages while maintaining a fast, reliable test suite that can run in CI and on developer machines (including macOS).

---

## Testing Framework

### Primary Tools

| Tool | Purpose | Package |
|------|---------|---------|
| `testing` (stdlib) | Test runner and benchmarks | Built-in |
| `testify/assert` | Rich assertions with readable failure messages | `github.com/stretchr/testify/assert` |
| `testify/require` | Assertions that stop test execution on failure | `github.com/stretchr/testify/require` |
| `testify/mock` | Interface-based mocking | `github.com/stretchr/testify/mock` |
| `testify/suite` | Test suites with setup/teardown (optional) | `github.com/stretchr/testify/suite` |

### When to Use `assert` vs `require`

- Use `require` when a failure makes subsequent assertions meaningless (e.g., checking `err == nil` before using the result)
- Use `assert` when you want to collect multiple failures in a single test run

```go
func TestReadConfig(t *testing.T) {
    cfg, err := config.ReadConfig()
    require.NoError(t, err)           // Stop if this fails
    assert.Equal(t, "expected", cfg.Hostname)  // Collect multiple
    assert.NotEmpty(t, cfg.K3OS.Modules)
}
```

---

## Test Commands

### Platform notes

This project targets Linux only. Several packages (for example `internal/mount`) use Linux-specific APIs, so the full test suite must run on Linux.

On Linux hosts, run tests directly (no `GOOS` prefix required). CircleCI uses the same commands.

On **macOS and Windows**, setting `GOOS=linux` cross-compiles test binaries for Linux but **does not run them** on the host (`exec format error`). Use Docker (below) or a Linux VM to execute tests locally.

### Running Tests (Linux)

```bash
# Run all tests
go test ./...

# Run all tests with race detection (CI mode)
go test -race -covermode=atomic -failfast ./...

# Run tests for a specific package
go test ./internal/config/...

# Run a specific test function
go test -run TestReadConfig ./internal/config/...

# Run tests with verbose output
go test -v ./...

# Run tests with coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Running Tests in Docker (macOS / Windows)

Use the Go image version that matches `go` in `go.mod` (currently **1.24**). Mount the repository at `/app` and run tests inside the container:

```bash
# From the repository root on the host
docker run --rm -v "$(pwd)":/app -w /app golang:1.24-bookworm \
  go test -race -covermode=atomic -failfast ./...
```

Package-scoped runs:

```bash
docker run --rm -v "$(pwd)":/app -w /app golang:1.24-bookworm \
  go test -v ./internal/config/...

docker run --rm -v "$(pwd)":/app -w /app golang:1.24-bookworm \
  go test -v ./internal/version/...
```

Build verification (matches CI):

```bash
docker run --rm -v "$(pwd)":/app -w /app golang:1.24-bookworm \
  sh -c 'CGO_ENABLED=0 go build -o /dev/null .'
```

On macOS, `$(pwd)` resolves correctly for Docker Desktop volume mounts. On Windows, use the path format required by your Docker setup (for example `%cd%` in PowerShell with appropriate path conversion).

Packages without Linux-only dependencies (for example `internal/version`) can be tested on the host without Docker, but prefer Docker for parity with CI before pushing.

### Linting

On Linux, or inside the same Docker image:

```bash
# Run linter with auto-fix
golangci-lint run --fix --new-from-rev=HEAD ./...

# Run linter without auto-fix (CI mode)
golangci-lint run ./...
```

Example via Docker on macOS / Windows:

```bash
docker run --rm -v "$(pwd)":/app -w /app golangci/golangci-lint:latest-alpine golangci-lint run --fix --new-from-rev=HEAD
```

---

## Test File Conventions

### Location

All test files live alongside the code they test, following Go conventions:

```text
internal/config/
├── config.go
├── read.go
├── read_test.go      # Tests for config reading
├── write.go
├── write_test.go     # Tests for config writing
├── coerce.go
└── coerce_test.go    # Tests for type coercion
```

### Naming

| Convention | Example |
|-----------|---------|
| Test file | `*_test.go` (same package) |
| Test function | `Test<Function>` or `Test<Function>_<scenario>` |
| Table-driven sub-test | `t.Run("<scenario>", ...)` |
| Test helper | `testHelper<Name>(t *testing.T, ...)` with `t.Helper()` |
| Mock type | `Mock<Interface>` |
| Fixture file | `testdata/<name>.yaml` |

### Example Test Structure

```go
package config

import (
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestReadConfig_MergesMultipleSources(t *testing.T) {
    // Arrange
    // ...setup test fixtures...

    // Act
    cfg, err := ReadConfig()

    // Assert
    require.NoError(t, err)
    assert.Equal(t, "expected-hostname", cfg.Hostname)
}
```

---

## Table-Driven Tests

Use table-driven tests for functions with multiple input/output scenarios:

```go
func TestToYAMLKey(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected string
    }{
        {
            name:     "camelCase to yaml-key",
            input:    "sshAuthorizedKeys",
            expected: "ssh_authorized_keys",
        },
        {
            name:     "already lowercase",
            input:    "hostname",
            expected: "hostname",
        },
        // Add edge cases here
    }

    for _, tc := range tests {
        t.Run(tc.name, func(t *testing.T) {
            result := convert.ToYAMLKey(tc.input)
            assert.Equal(t, tc.expected, result)
        })
    }
}
```

---

## Mocking Strategy

### When to Mock

Many packages in this project interact directly with the OS (mount syscalls, file I/O, `/proc` reads, `modprobe`). To test these packages without requiring root or a Linux environment:

1. **Define interfaces** for OS-dependent operations
2. **Create mock implementations** using `testify/mock`
3. **Inject dependencies** via function parameters or struct fields

### Interface Pattern

```go
// internal/mount/mount.go
type Mounter interface {
    Mount(device, target, mType, options string) error
    ForceMount(device, target, mType, options string) error
    Mounted(target string) (bool, error)
}

// Real implementation
type LinuxMounter struct{}

func (m *LinuxMounter) Mount(device, target, mType, options string) error {
    // ... actual syscall implementation ...
}
```

### Mock Pattern

```go
// internal/mount/mock_mounter_test.go
package mount

import "github.com/stretchr/testify/mock"

type MockMounter struct {
    mock.Mock
}

func (m *MockMounter) Mount(device, target, mType, options string) error {
    args := m.Called(device, target, mType, options)
    return args.Error(0)
}

func (m *MockMounter) Mounted(target string) (bool, error) {
    args := m.Called(target)
    return args.Bool(0), args.Error(1)
}
```

### Using Mocks in Tests

```go
func TestUpgradeComponent_MountsBeforeCopy(t *testing.T) {
    mockMounter := new(MockMounter)
    mockMounter.On("Mount", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
        Return(nil)

    // ... use mockMounter in the code under test ...

    mockMounter.AssertExpectations(t)
}
```

---

## Test Categories

### Pure Logic Tests (Priority: High)

These packages can be tested without mocking — they have no OS dependencies:

| Package | What to Test |
|---------|-------------|
| `config/coerce.go` | Type coercion mappers (string→bool, string→[]string) |
| `config/rename.go` | Field name mapping |
| `config/config.go` | Data model validation, `Debug()` function |
| `config/write.go` | YAML serialization, key conversion |
| `system/system.go` | Path construction (`RootPath`, `DataPath`, etc.) |
| `version/version.go` | Version variable |
| `mode/mode.go` | Mode detection (with temp files) |

### I/O-Dependent Tests (Priority: High, requires mocking)

| Package | What to Test | Mock Strategy |
|---------|-------------|---------------|
| `config/read.go` | Config reading, merging, cmdline parsing | Mock file readers, use `testdata/` fixtures |
| `cc/funcs.go` | Each applier function | Mock file writers, command execution |
| `module/module.go` | Module loading | Mock `/proc/modules` reader, mock modprobe |
| `sysctl/sysctl.go` | Sysctl application | Mock file writer |
| `hostname/hostname.go` | Hostname setting | Mock `os.WriteFile` |
| `ssh/ssh.go` | SSH key management | Mock file system operations |
| `writefile/writefile.go` | File writing with encoding | Use temp directories |

### System-Level Tests (Priority: Medium, Linux-only)

| Package | What to Test | Constraints |
|---------|-------------|-------------|
| `mount/` | Mount/unmount operations | Requires Linux + root |
| `enterchroot/` | Chroot entry and squashfs mount | Requires Linux + root |
| `transferroot/` | Root filesystem relocation | Requires Linux + root |
| `cli/rc/` | Run control (full boot sequence) | Requires Linux + root |

Use build tags for Linux-only tests:

```go
//go:build linux

package mount
```

---

## Test Data / Fixtures

Store test fixtures in `testdata/` directories within each package:

```text
internal/config/
├── testdata/
│   ├── basic_config.yaml
│   ├── merged_config.yaml
│   ├── cloud_config.yaml
│   └── cmdline.txt
├── config.go
├── read.go
└── read_test.go
```

Go's `go test` automatically excludes `testdata/` from compilation.

---

## Coverage Requirements

| Scope | Target | Notes |
|-------|--------|-------|
| Overall `internal/` | ≥60% | Measured by CI |
| Pure logic packages | ≥80% | `config/coerce`, `config/write`, `system/system` |
| Applier functions | ≥60% | `cc/funcs.go` |
| System-level packages | Best effort | `mount`, `enterchroot`, `transferroot` |

### Coverage Commands

Run on Linux or inside the Docker image from [Running Tests in Docker](#running-tests-in-docker-macos--windows):

```bash
# Generate coverage profile
go test -coverprofile=coverage.out ./internal/...

# View coverage summary
go tool cover -func=coverage.out

# Generate HTML report
go tool cover -html=coverage.out -o coverage.html
```

---

## CI Integration

Tests run in CircleCI on every push:

```yaml
- go/test:
    covermode: atomic
    failfast: true
    race: true
```

### CI Test Requirements

1. All tests must pass (`-failfast`)
2. Race detector enabled (`-race`)
3. Atomic coverage mode (`-covermode=atomic`)
4. Build must succeed after tests
5. Linter must pass with zero issues

---

## Test Anti-Patterns to Avoid

| Anti-Pattern | Instead Do |
|-------------|-----------|
| Testing private functions directly | Test via public API |
| Relying on global state | Use dependency injection |
| Hard-coding file paths | Use `t.TempDir()` or `testdata/` |
| Ignoring errors in tests | Always check errors with `require.NoError` |
| Large monolithic test functions | Break into focused sub-tests with `t.Run` |
| Testing the framework (yaml parsing) | Test your logic, not third-party libraries |
| Sleep-based synchronization | Use channels, waitgroups, or polling |
| Happy-path only tests | Include error cases, edge cases, empty inputs |

---

## Edge Cases to Always Consider

For every function under test, consider:

1. **Empty/nil inputs**: Empty slices, nil maps, zero values
2. **Missing files**: Config files that don't exist
3. **Permission errors**: Operations that require root
4. **Invalid data**: Malformed YAML, unexpected types
5. **Boundary conditions**: Empty `config.d/` directory, single vs. multiple sources
6. **Concurrency**: If the code could be called concurrently
7. **Platform differences**: Tests that would fail on macOS vs Linux
