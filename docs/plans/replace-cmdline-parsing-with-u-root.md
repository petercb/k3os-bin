# Implementation Plan: Replace Cmdline Parsing with u-root

## Summary

Replace manual `/proc/cmdline` parsing (strings.Fields + prefix matching) with
the `github.com/u-root/u-root/pkg/cmdline` package, accessed through a
`CmdlineParser` interface for testability.

## Background

The u-root `pkg/cmdline` package is already a dependency (v0.16.0). It provides:

- `CmdLine` struct with exported fields: `Raw string`, `AsMap map[string]string`, `Err error`
- `(*CmdLine).Flag(name string) (string, bool)` - get a flag value
- `(*CmdLine).ContainsFlag(name string) bool` - check presence (key exists in map)
- `(*CmdLine).Consoles() []string` - parse multiple `console=` entries from Raw
- `parse(io.Reader) *CmdLine` - internal parsing (not exported, but struct is constructable)

Key insight: `Flag()` uses `strings.Replace(flag, "-", "_", -1)` for canonical
lookup, so `k3os.mode` and `k3os_mode` are NOT equivalent (dots are preserved).
The existing `k3os.mode=` parameters work correctly with `Flag("k3os.mode")`.

## Interface Design

```go
// CmdlineParser abstracts kernel command-line parsing for testability.
type CmdlineParser interface {
    Flag(name string) (string, bool)
    Contains(name string) bool
    Consoles() []string
    Raw() string
}
```

Note: `Consoles()` is added because the TTY setup needs ALL console= entries
(there can be multiple), and `Flag("console")` only returns the last one.

## Implementation wrapping

```go
// uRootCmdline wraps *cmdline.CmdLine to satisfy CmdlineParser.
type uRootCmdline struct {
    cl *cmdline.CmdLine
}

func (u *uRootCmdline) Flag(name string) (string, bool) { return u.cl.Flag(name) }
func (u *uRootCmdline) Contains(name string) bool       { return u.cl.ContainsFlag(name) }
func (u *uRootCmdline) Consoles() []string              { return u.cl.Consoles() }
func (u *uRootCmdline) Raw() string                     { return u.cl.Raw }
```

For testing, we create from a raw string:

```go
func NewFromString(raw string) CmdlineParser {
    // Construct CmdLine manually using exported fields
    cl := &cmdline.CmdLine{
        Raw:   raw,
        AsMap: parseToMap(raw),  // need to use the parse function
    }
    return &uRootCmdline{cl: cl}
}
```

Actually, since `parse()` is unexported, the best approach is to use
`strings.NewReader(raw)` and construct via the exported struct fields. But the
AsMap construction uses internal `parseToMap`. Instead we can use the `CmdLine`
struct directly by populating `AsMap` ourselves, or better: create the wrapper
that uses `strings.NewReader` + the exported `parse` path.

Wait - `parse()` is unexported. But the `CmdLine` struct fields ARE exported.
The simplest approach for testing: construct `CmdLine{Raw: raw, AsMap: map}` manually
using a helper that replicates the key=value parsing (simple split on `=`).

Best approach: provide a `NewCmdlineParser()` that calls `cmdline.NewCmdLine()`
for production, and a `NewCmdlineParserFromString(raw string)` for tests that
constructs the struct with proper AsMap parsing.

Actually, since tests just mock the interface, we only need the production
constructor `NewCmdlineParser()` and tests provide mock implementations directly.

## Changes by File

### 1. `internal/iface/iface.go` - Add CmdlineParser interface

```go
// CmdlineParser abstracts kernel command-line parsing.
type CmdlineParser interface {
    Flag(name string) (string, bool)
    Contains(name string) bool
    Consoles() []string
    Raw() string
}
```

### 2. New file: `internal/cmdline/cmdline.go` - Production implementation

Creates the concrete implementation wrapping u-root, plus a constructor.

### 3. `internal/mode/detect.go` - Replace CmdlineReader with CmdlineParser

- Change `CmdlineReader func() (string, error)` to `Cmdline iface.CmdlineParser`
- Replace `parseCmdline()` with direct `Flag()` / `Contains()` calls
- Remove the `parseCmdline` pure function and `CmdlineResult` struct

### 4. `internal/boot/init.go` - Replace CmdlineReader with CmdlineParser

- Change `CmdlineReader func() (string, error)` to `Cmdline iface.CmdlineParser`
- Replace `setupDebug()` logic with `i.Cmdline.Contains("k3os.debug")`

### 5. `internal/boot/finalize/finalize.go` + `ttys.go` - Replace CmdlineReader

- Change `CmdlineReader func() (string, error)` to `Cmdline iface.CmdlineParser`
- Replace `parseConsoleEntries(cmdline)` with `f.Cmdline.Consoles()` for TTY names
- Keep baudrate parsing from the raw value (use `Flag("console")` or parse raw)

Actually, u-root's `Consoles()` only returns TTY names (strips baudrate). The
existing code needs baudrate info too. Two options:
- Keep parsing raw line for `console=` with baudrate
- Use `Cmdline.Raw()` and parse console entries from it

Best approach: The `Consoles()` method on u-root only returns the first part
before `,`. We still need baudrate. So we'll use `Cmdline.Raw()` and keep
`parseConsoleEntries()` but feed it `Cmdline.Raw()` instead of
`CmdlineReader()`.

### 6. `internal/enterchroot/enter.go` - Replace isDebug cmdline parsing

- Add a `CmdlineParser` dependency (package-level var like `loopAttacher`)
- Replace the `isDebug()` manual parsing with `cmdlineParser.Contains(DebugCmdline)`

### 7. Update all test files

- Mock `CmdlineParser` interface in tests
- Remove tests for deleted `parseCmdline()` function (or keep as tests of the
  new interface behavior)

## Feature Order

1. **FEAT-001**: Add CmdlineParser interface + production implementation (TDD)
2. **FEAT-002**: Migrate all consumers to use CmdlineParser interface (TDD)

## Risks

- u-root `Consoles()` does not return baudrate info - mitigated by using Raw()
- `parseCmdline()` in mode/detect.go handles `rescue` keyword specially (maps to "shell") -
  this semantic must be preserved in the new code
- The `enterchroot` package uses a package-level var pattern for testability
