# Implementation Plan: Adopt u-root uinit.flags Pattern

## Summary

Create a reusable `internal/flagsource` package that implements the u-root
pattern of merging kernel cmdline args + flags file + environment into a single
args slice. This provides composable, testable building blocks for argument
resolution from multiple sources.

## Background

u-root's init reads `uroot.uinitargs` from the kernel cmdline (splits via
`shlex.Argv`), then reads `/etc/uinit.flags` (Go-quoted one-per-line via
`uflag.FileToArgv`), and merges both into a single args slice passed to uinit.

This "merge kernel cmdline + flags file + environment into args" pattern is
clean and reusable. Rather than importing u-root's specific implementation
(which is tightly coupled to their init flow), we adopt the pattern as a
standalone package with well-defined interfaces.

Key u-root primitives reused:

- `github.com/u-root/u-root/pkg/shlex` (already a transitive dep at v0.16.0):
  `shlex.Argv(s string) []string` provides shell-like argument splitting.
- The flags-file format from `uflag.FileToArgv`: one Go-quoted string per line,
  blank lines and `#` comments skipped, parsed via `strconv.Unquote`.

## Interface Design

```go
// Source provides command-line arguments from a single origin.
type Source interface {
    Args() ([]string, error)
}
```

### Concrete Implementations

**CmdlineSource** - Extracts a named flag from the kernel cmdline and splits
its value into arguments using shell-like parsing:

```go
type CmdlineSource struct {
    Parser   iface.CmdlineParser
    FlagName string
}
```

**FileSource** - Reads a flags file in the uflag format (Go-quoted strings,
one per line, blank lines and `#` comments skipped):

```go
type FileSource struct {
    Path string
}
```

**EnvSource** - Reads an environment variable and splits its value into
arguments using shell-like parsing:

```go
type EnvSource struct {
    Name string
}
```

### Merge Function

```go
func Merge(sources ...Source) ([]string, error)
```

Concatenates all source results in order. If any source returns an error, Merge
stops and propagates that error immediately.

## File Format (FileSource)

Each non-empty, non-comment line contains a single Go-quoted string:

```text
# Example flags file
"-v"
"--config=/etc/myapp.conf"
"--timeout=30s"
```

This matches the format used by u-root's `uflag.FileToArgv`. Lines are parsed
with `strconv.Unquote`. A missing file is not an error (the file is optional).

## Integration Opportunity

The config system's `readCmdline()` in `internal/config/read.go` currently uses
a regex-based parser instead of the existing u-root wrapper. The `flagsource`
package provides the building blocks to unify that approach: a `CmdlineSource`
can extract `k3os.config` from the kernel cmdline, a `FileSource` can read
additional config flags, and `Merge` combines them cleanly.

## Package Dependencies

```
internal/flagsource
    -> internal/iface          (CmdlineParser interface)
    -> github.com/u-root/u-root/pkg/shlex  (shell-like splitting)
    -> os                      (env var lookup, file reading)
    -> strconv                 (Go-quoted string parsing)
```

The package does NOT depend on `internal/config` (it is a lower-level
primitive that config may eventually consume).

## Feature Order

1. **FEAT-001**: Create the flagsource package with Source interface,
   CmdlineSource, FileSource, EnvSource implementations, Merge function,
   and comprehensive tests.
2. **FEAT-002**: Integrate flagsource into the config reading pipeline,
   replacing the regex-based cmdline parser.
