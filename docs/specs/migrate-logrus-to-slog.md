# Migrate from logrus to log/slog

## Rationale

- **Standard library solution**: `log/slog` is part of the Go standard library (available
  since Go 1.21), eliminating the external `github.com/sirupsen/logrus` dependency.
- **Structured logging**: slog provides first-class structured key/value logging out of
  the box.
- **Go team supported**: maintained by the Go core team with long-term stability
  guarantees.
- **No external dependency**: reduces module footprint and supply-chain surface area.

## API Mapping

| logrus call | slog equivalent |
|---|---|
| `logrus.Debug(msg)` | `slog.Debug(msg)` |
| `logrus.Debugf(format, args...)` | `slog.Debug(msg, key, value, ...)` (use structured attrs instead of formatting) |
| `logrus.Info(msg)` | `slog.Info(msg)` |
| `logrus.Infof(format, args...)` | `slog.Info(msg, key, value, ...)` (use structured attrs instead of formatting) |
| `logrus.Warn(err)` | `slog.Warn(msg)` |
| `logrus.Error(err)` | `slog.Error(msg)` |
| `logrus.Errorf(format, args...)` | `slog.Error(msg, key, value, ...)` (use structured attrs instead of formatting) |
| `logrus.Fatal(err)` | `slog.Error(msg, ...); os.Exit(1)` |
| `logrus.Fatalf(format, args...)` | `slog.Error(msg, key, value, ...); os.Exit(1)` |
| `logrus.SetLevel(logrus.DebugLevel)` | `slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})))` |
| `logrus.GetLevel() >= logrus.DebugLevel` | Check environment variable or package-level debug flag |

### Notes on Fatal

`log/slog` intentionally omits a Fatal-level function. The recommended pattern is to
log at Error level and then call `os.Exit(1)` explicitly. This keeps the logging
library free of process-lifecycle side effects.

### Notes on Level Control

`slog.SetLogLoggerLevel` only controls the level threshold for messages routed through
the standard library `log` package bridge. It does NOT affect the level of slog's own
default handler. To change the minimum level for `slog.Debug`, `slog.Info`, etc., you
must replace the default handler by calling `slog.SetDefault` with a new handler that
has the desired `Level` set in its `HandlerOptions`.

### Notes on Formatted Messages

`logrus.Debugf`, `logrus.Infof`, `logrus.Errorf`, and `logrus.Fatalf` use
`fmt.Sprintf`-style formatting. The slog equivalent is to pass structured key/value
pairs instead. Where a formatted string embeds a single variable, extract it as an
attribute:

```go
// Before
logrus.Infof("downloading %s", url)

// After
slog.Info("downloading", "url", url)
```

## Files to Modify

| File | Reason |
|---|---|
| `main.go` | Entry point; imports logrus |
| `internal/cli/app/app.go` | Sets log level, uses debug flag |
| `internal/cc/funcs.go` | Uses logrus.Debugf, logrus.Errorf |
| `internal/cli/upgrade/upgrade.go` | Uses logrus.Infof, logrus.Errorf, logrus.Fatalf |
| `internal/enterchroot/enter.go` | Uses logrus.Debugf, logrus.Fatalf, logrus.Errorf |
| `internal/enterchroot/ensureloop.go` | Uses logrus.Debugf |
| `internal/system/component.go` | Uses logrus.Infof, logrus.Errorf |

## Testing Approach

- **TDD**: verify that all existing tests continue to pass since logging is transparent
  to callers.
- Debug-level calls produce no externally observable behavior change, so no new test
  assertions are needed for them.
- Integration-style tests that capture stdout/stderr should remain unaffected because
  slog defaults to the same stderr output.

## Post-Migration Cleanup

After all logrus references are removed:

1. Run `go mod tidy` to remove `github.com/sirupsen/logrus` from `go.mod` and
   `go.sum`.
2. Run `go build ./...` to confirm compilation.
3. Run `go test ./...` to confirm all tests pass.
4. Run `golangci-lint run ./...` to confirm zero lint issues.
