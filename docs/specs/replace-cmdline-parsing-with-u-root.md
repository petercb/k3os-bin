# Specification: Replace Hand-Rolled Cmdline Parsing with u-root pkg/cmdline

## Overview

Replace the manual `/proc/cmdline` parsing in mode detection and debug setup
with the well-tested `github.com/u-root/u-root/pkg/cmdline` package.

## Motivation

The current mode detection code (`internal/mode/`) and debug initialization
(`internal/boot/init.go`) parse `/proc/cmdline` manually using
`strings.Fields()` and prefix matching:

```go
for _, field := range strings.Fields(cmdline) {
    if strings.HasPrefix(field, "k3os.mode=") {
        mode = strings.TrimPrefix(field, "k3os.mode=")
    }
}
```

This approach:
- Doesn't handle quoted values (e.g., `k3os.mode="disk"`)
- Doesn't handle parameters with spaces
- Is duplicated in multiple places
- Doesn't support the kernel's `var_name`/`var-name` equivalence

The u-root `pkg/cmdline` package correctly handles all kernel cmdline edge
cases and provides a clean map-based API.

## Current Parsing Locations

| Location | What it parses |
|----------|----------------|
| `internal/mode/detector.go` | `k3os.mode=`, `k3os.fallback_mode=`, `rescue` |
| `internal/boot/init.go` | `k3os.debug` |
| `internal/boot/finalize/ttys.go` | `console=ttyS0,115200` |
| `internal/enterchroot/enter.go` | `k3os.debug` (DebugCmdline) |

## Target Package

- **Import**: `github.com/u-root/u-root/pkg/cmdline`
- **License**: BSD-3-Clause
- **Key APIs**:
  ```go
  cmdline.NewCmdLine() *CmdLine       // Parse /proc/cmdline
  cmdline.Flag(name string) (string, bool)  // Get a flag value
  cmdline.ContainsFlag(name string) bool    // Check presence
  cmdline.FullCmdLine() string              // Raw string
  ```

## Design

### Wrapper interface for testability

```go
// CmdlineParser abstracts kernel command-line parsing.
type CmdlineParser interface {
    Flag(name string) (string, bool)
    Contains(name string) bool
    Raw() string
}
```

### Integration points

1. **Mode detection**: `parser.Flag("k3os.mode")` instead of manual prefix scan
2. **Debug check**: `parser.Contains("k3os.debug")` instead of field iteration
3. **Console parsing**: `parser.Flag("console")` for TTY setup
4. **Rescue detection**: `parser.Contains("rescue")`

### Fallback behavior

The `k3os.fallback_mode` parameter uses a dot separator. u-root's cmdline
package treats dots as valid flag name characters, so
`cmdline.Flag("k3os.fallback_mode")` works directly.

## Dependencies Added

- `github.com/u-root/u-root/pkg/cmdline`
- Transitive: `github.com/u-root/u-root/pkg/shlex` (shell-style lexing)

## Migration Steps

1. Add `CmdlineParser` interface to `internal/iface`
2. Create implementation wrapping u-root's `cmdline.NewCmdLine()`
3. Replace `CmdlineReader func() (string, error)` fields with `CmdlineParser`
4. Update mode detector to use `Flag()` / `Contains()` API
5. Update debug checks to use `Contains("k3os.debug")`
6. Update TTY setup to use `Flag("console")`
7. Remove manual parsing code

## Acceptance Criteria

- [ ] All `/proc/cmdline` parsing uses the `CmdlineParser` interface
- [ ] No manual `strings.Fields()` + prefix matching for cmdline parameters
- [ ] Quoted values are handled correctly
- [ ] Unit tests pass with mocked `CmdlineParser`
- [ ] `golangci-lint run ./...` passes
- [ ] Binary size increase is minimal (shlex is small)
