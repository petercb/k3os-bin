# Design Document

## Overview

This design covers the removal of two standalone packages (`internal/module` and `internal/sysctl`) whose logic is already superseded by the interface-based `cc.Applier` pattern, and the addition of Linux-only integration tests for the real OS adapter implementations in `internal/iface/osimpl/`.

The work is purely subtractive (dead code removal) plus additive (new test files). No production logic changes are required — the `osimpl.LinuxModuleLoader` and `osimpl.LinuxSysctlApplier` implementations are already in use via `cc.NewDefaultApplier()`.

## Architecture

### Current State

```
internal/module/module.go       ← standalone LoadModules(cfg) — DEAD CODE
internal/sysctl/sysctl.go       ← standalone ConfigureSysctl(cfg) — DEAD CODE
internal/iface/osimpl/module.go ← LinuxModuleLoader (active, used by cc.Applier)
internal/iface/osimpl/sysctl.go ← LinuxSysctlApplier (active, used by cc.Applier)
```

### Target State

```
internal/module/                 ← REMOVED (directory deleted)
internal/sysctl/                 ← REMOVED (directory deleted)
internal/iface/osimpl/module.go          ← unchanged
internal/iface/osimpl/module_test.go     ← NEW: integration tests
internal/iface/osimpl/sysctl.go          ← unchanged
internal/iface/osimpl/sysctl_test.go     ← NEW: integration tests
```

### Dependency Graph (Post-Change)

```
cc.Applier
  ├── iface.ModuleLoader  → osimpl.LinuxModuleLoader
  │                            ├── reads /proc/modules
  │                            └── calls modprobe.Load()
  └── iface.SysctlApplier → osimpl.LinuxSysctlApplier
                                 └── writes /proc/sys/<path>
```

No callers import `internal/module` or `internal/sysctl` (confirmed via grep). The removal is safe.

## Components and Interfaces

### 1. File Removal

| File | Action | Rationale |
|------|--------|-----------|
| `internal/module/module.go` | Delete | Logic duplicated in `osimpl.LinuxModuleLoader` + `cc.ApplyModules` |
| `internal/sysctl/sysctl.go` | Delete | Logic duplicated in `osimpl.LinuxSysctlApplier` + `cc.ApplySysctls` |

### 2. Integration Test: `internal/iface/osimpl/module_test.go`

```go
//go:build linux

package osimpl_test

import (
	"strings"
	"testing"

	"github.com/petercb/k3os-bin/internal/iface/osimpl"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLinuxModuleLoader_LoadedModules_ReturnsNonEmpty(t *testing.T) {
	loader := osimpl.LinuxModuleLoader{}
	modules, err := loader.LoadedModules()
	require.NoError(t, err)
	assert.NotEmpty(t, modules, "expected at least one loaded kernel module")
}

func TestLinuxModuleLoader_LoadedModules_NamesHaveNoWhitespace(t *testing.T) {
	loader := osimpl.LinuxModuleLoader{}
	modules, err := loader.LoadedModules()
	require.NoError(t, err)

	for name := range modules {
		assert.NotContains(t, name, " ", "module name should not contain spaces")
		assert.NotContains(t, name, "\t", "module name should not contain tabs")
		assert.Equal(t, name, strings.TrimSpace(name),
			"module name should have no leading/trailing whitespace")
	}
}

func TestLinuxModuleLoader_LoadedModules_ExtractsOnlyFirstField(t *testing.T) {
	loader := osimpl.LinuxModuleLoader{}
	modules, err := loader.LoadedModules()
	require.NoError(t, err)

	// /proc/modules lines: <name> <size> <refcount> <deps> <state> <offset>
	// Module names are identifiers: alphanumeric + underscore, no spaces
	for name := range modules {
		assert.Regexp(t, `^[a-zA-Z0-9_]+$`, name,
			"module name should only contain alphanumeric chars and underscores")
	}
}
```

### 3. Integration Test: `internal/iface/osimpl/sysctl_test.go`

```go
//go:build linux

package osimpl_test

import (
	"os"
	"testing"

	"github.com/petercb/k3os-bin/internal/iface/osimpl"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLinuxSysctlApplier_Set_WritesToCorrectPath(t *testing.T) {
	applier := osimpl.LinuxSysctlApplier{}

	// Read current value to restore after test (avoid side effects)
	current, err := os.ReadFile("/proc/sys/net/ipv4/ip_forward")
	require.NoError(t, err)

	// Write back same value — verifies dot-to-path conversion
	err = applier.Set("net.ipv4.ip_forward", string(current))
	assert.NoError(t, err)
}

func TestLinuxSysctlApplier_Set_DotConversion(t *testing.T) {
	applier := osimpl.LinuxSysctlApplier{}

	// kernel.hostname is a safe sysctl to read/write
	current, err := os.ReadFile("/proc/sys/kernel/hostname")
	require.NoError(t, err)

	// Write back same value — verifies multi-segment dot-to-path conversion
	err = applier.Set("kernel.hostname", string(current))
	assert.NoError(t, err)
}

func TestLinuxSysctlApplier_Set_NonExistentPath_ReturnsError(t *testing.T) {
	applier := osimpl.LinuxSysctlApplier{}

	err := applier.Set("nonexistent.fake.key", "1")
	assert.Error(t, err, "expected error for non-existent sysctl path")
}
```

### 4. Interfaces

No new interfaces are introduced. The existing interfaces are already defined in `internal/iface/iface.go`:

```go
// ModuleLoader abstracts kernel module loading.
type ModuleLoader interface {
	LoadedModules() (map[string]bool, error)
	LoadModule(name string, params string) error
}

// SysctlApplier abstracts sysctl configuration.
type SysctlApplier interface {
	Set(key string, value string) error
}
```

The implementations under test:

```go
// LinuxModuleLoader implements iface.ModuleLoader using /proc/modules and modprobe.
type LinuxModuleLoader struct{}

// LinuxSysctlApplier implements iface.SysctlApplier by writing to /proc/sys/.
type LinuxSysctlApplier struct{}
```

## Data Models

No new data models. The tests operate on:

- `/proc/modules` — kernel-provided pseudo-file with line format: `<name> <size> <refcount> <deps> <state> <offset>`
- `/proc/sys/` — kernel-provided pseudo-filesystem where sysctl keys map to file paths (dots become directory separators)

## Error Handling

| Scenario | Behavior | Test Coverage |
|----------|----------|---------------|
| `/proc/modules` unreadable | `LoadedModules()` returns `(nil, error)` | Covered by existing mock tests in `cc` package |
| `/proc/modules` scanner error | `LoadedModules()` returns `(partial, error)` | Covered by `sc.Err()` check in implementation |
| Non-existent sysctl path | `Set()` returns `os.WriteFile` error (ENOENT) | Integration test: `TestLinuxSysctlApplier_Set_NonExistentPath_ReturnsError` |
| Permission denied on sysctl write | `Set()` returns `os.WriteFile` error (EACCES) | Requires non-root execution (not tested — Docker runs as root) |

## Testing Strategy

### Local (macOS) via Docker

```bash
# Run integration tests
docker run --rm -v "$(pwd)":/app -w /app golang:1.21.9-bookworm \
  go test -v ./internal/iface/osimpl/...

# Run with coverage
docker run --rm -v "$(pwd)":/app -w /app golang:1.21.9-bookworm \
  go test -coverprofile=coverage.out ./internal/iface/osimpl/...

# View coverage
go tool cover -func=coverage.out
```

### CI (Linux)

Tests run natively as part of `go test ./...` since the `//go:build linux` tag matches the CI environment.

### Coverage Target

The `LinuxModuleLoader` has two methods:
- `LoadedModules()` — 12 statements, all exercised by the non-empty + name-format tests
- `LoadModule()` — 1 statement (delegates to `modprobe.Load`), not safe to test in integration (would load a real kernel module)

The `LinuxSysctlApplier` has one method:
- `Set()` — 4 statements, all exercised by the success + error path tests

Expected coverage: ~85% for `module.go` (all paths except `LoadModule` error), 100% for `sysctl.go`.

## Commit Plan

| Order | Type | Message | Content |
|-------|------|---------|---------|
| 1 | docs | `docs(spec): add TASK-008 module-sysctl-tests spec` | Spec files |
| 2 | refactor | `refactor(module,sysctl): remove standalone packages in favor of iface-based appliers` | Delete `internal/module/` and `internal/sysctl/` |
| 3 | test | `test(osimpl): add Linux-only integration tests for module and sysctl adapters` | Add `module_test.go` and `sysctl_test.go` |
| 4 | docs | `docs: update status for TASK-008 completion` | Update `docs/status.md` |

Each commit must pass `golangci-lint run ./...` independently.

## Correctness Properties

*A property is a characteristic or behavior that should hold true across all valid executions of a system — essentially, a formal statement about what the system should do. Properties serve as the bridge between human-readable specifications and machine-verifiable correctness guarantees.*

### Property 1: Module name parsing extracts only the first field

*For any* `/proc/modules` line in the format `<name> <size> <refcount> <deps> <state> <offset>`, the `LoadedModules()` parser SHALL extract only the first whitespace-delimited field as the module name key, with no trailing whitespace, numeric data, or parameter content.

**Validates: Requirements 4.3, 4.4**

### Property 2: Sysctl key dot-to-path conversion

*For any* sysctl key string containing dot separators (e.g., `a.b.c`), the `Set()` method SHALL construct the file path `/proc/sys/a/b/c` by splitting the key on dots and joining the segments as path components under `/proc/sys/`.

**Validates: Requirements 5.2, 5.3**
