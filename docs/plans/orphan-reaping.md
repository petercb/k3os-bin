# Orphan Process Reaping

## Why PID 1 Needs Orphan Reaping

When a process exits, its children are reparented to PID 1 (the init process).
If PID 1 does not call `wait()` on these orphaned children, they remain in a
zombie state indefinitely, consuming process table entries and potentially
exhausting system resources.

In k3OS, the init binary runs as PID 1 during the boot sequence. Any process
spawned during bootstrap, mode detection, or mode execution that exits before
its parent calls `wait()` becomes an orphan reparented to our init. Without
active reaping, these accumulate as zombies.

## Design

### Context-Based Lifecycle

The reaper uses a `context.Context` for lifecycle management:

- `Start(ctx context.Context)` spawns a background goroutine that polls for
  zombie children using `syscall.Wait4(-1, &status, WNOHANG, nil)`.
- The goroutine exits when `ctx.Done()` fires.
- `Wait()` blocks until the goroutine has fully stopped.

This design integrates cleanly with Go's standard cancellation patterns and
avoids custom channel-based stop protocols.

### Polling Strategy

The reaper uses `WNOHANG` (non-blocking wait) with a 100ms ticker interval.
This avoids busy-waiting while still reaping zombies promptly. The `reapAll()`
helper drains all available zombies in a single tick to handle bursts of child
exits efficiently.

## Integration Point

The reaper is integrated into `boot.Init.Run()`:

1. A cancellable context is created at the start of `Run()`.
2. If the `Reaper` field is non-nil, `Start(ctx)` is called immediately after
   log handler setup (before bootstrap).
3. A deferred `cancel()` ensures the context is cancelled when `Run()` returns.
4. A deferred `Wait()` ensures the goroutine exits cleanly before the process
   is replaced by `exec(/sbin/init)`.

The reaper is only activated when `os.Getpid() == 1`, ensuring it does not
interfere with normal CLI usage of the binary.

## Future Considerations

Currently, `Run()` ends with `exec(/sbin/init)` which replaces the process
entirely, making the deferred cleanup academic in the success path. However,
if the exec-to-OpenRC pattern is removed in the future and k3OS manages
services directly, the reaper would continue running for the lifetime of PID 1,
reaping all orphaned descendants indefinitely.
