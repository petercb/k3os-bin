# Specification: Porting k3OS Init Scripts to Go

## 1. Overview and Motivation

The k3OS boot sequence is currently implemented as a set of Bash scripts under
`overlay/init` and `overlay/libexec/k3os/`. This specification describes the
plan to port those scripts into Go code within the `k3os-bin` repository,
producing a single statically-linked binary that handles the entire early
userspace initialization.

### Why port shell scripts to Go?

| Concern | Shell | Go |
|---------|-------|----|
| **Testability** | Difficult to unit-test; requires real filesystem/mounts | Interface-driven DI enables full unit/integration tests |
| **Single binary** | Requires bash + coreutils at runtime | Statically compiled, no runtime deps |
| **Type safety** | Untyped strings, silent failures | Compile-time type checking, explicit error handling |
| **Error handling** | `set -e` is fragile, errors easily lost | Every error is a value that must be handled |
| **Maintainability** | Implicit globals, sourcing side effects | Explicit dependency graph, package boundaries |
| **Performance** | Fork/exec for every utility call | Direct syscalls, no process spawning |
| **Debugging** | Limited; `set -x` only | Structured logging (`log/slog`), profiling, tracing |

## 2. Current Shell-Based Boot Flow

The boot sequence starts when the kernel hands control to `/init` (PID 1).
Below is the ordered execution flow:

```
/init (overlay/init)
 |
 |-- source /usr/libexec/k3os/functions   (utility functions, env setup)
 |-- source /usr/lib/os-release           (version metadata)
 |
 |-- ${SCRIPTS}/bootstrap                 (phase 1: early bootstrap)
 |    |-- setup_etc       : mount tmpfs on /etc, copy /usr/etc/*
 |    |-- setup_modules   : bind-mount kernel modules + firmware
 |    |-- setup_users     : create rancher user/group, set shell
 |    |-- k3os rc         : run rc (cloud-config) appliers
 |    |-- setup_dirs      : mkdir /run/k3os
 |    |-- setup_kernel    : mount kernel squashfs, bind modules/firmware
 |    |-- setup_config    : run `k3os config --initrd` (unless local mode)
 |
 |-- redirect console: exec >/dev/console </dev/console 2>&1
 |-- reinit_debug
 |
 |-- ${SCRIPTS}/mode                      (phase 2: mode detection)
 |    |-- parse /proc/cmdline for k3os.mode=, rescue, k3os.fallback_mode=
 |    |-- probe blkid for K3OS_STATE partition
 |    |-- detect local mode via filesystem type check
 |    |-- wait up to 30s for mode resolution
 |    |-- write mode to /run/k3os/mode
 |
 |-- source ${SCRIPTS}/mode-${K3OS_MODE}  (phase 3: mode-specific setup)
 |    |-- mode-disk:    mount state, grow partition, setup k3os/k3s,
 |    |                 pivot_root, exec /sbin/init
 |    |-- mode-local:   setup_ssh, setup_rancher_node
 |    |-- mode-live:    source live (setup_base, setup_kernel, setup_passwd, setup_motd)
 |    |-- mode-install: source live (same as mode-live)
 |    |-- mode-shell:   source live, then exec bash (rescue)
 |
 |-- source ${SCRIPTS}/boot               (phase 4: boot finalization)
 |    |-- setup_mounts      : bind /.base/boot, /k3os/system; unmount /.base
 |    |-- grow_live          : grow partition if growpart marker exists
 |    |-- setup_hostname     : generate or read hostname
 |    |-- setup_hosts        : generate /etc/hosts
 |    |-- setup_root         : create /root with 0700
 |    |-- setup_ttys         : configure getty on tty1-6 + serial consoles
 |    |-- setup_sudoers      : write sudoers config
 |    |-- setup_services     : symlink init.d scripts into runlevels
 |    |-- setup_config       : run `k3os config --boot`
 |    |-- setup_manifests    : rsync k3s server manifests
 |    |-- setup_state_dirs   : create /var/lib/nfs, kubernetes dirs
 |    |-- cleanup            : remove /run/k3os temp state, re-write mode
 |
 |-- exec /sbin/init                      (hand off to OpenRC)
```

### Key observations

1. **mode-disk** is unique: it performs `pivot_root` and re-execs, never
   reaching the boot phase within the same process.
2. **mode-live**, **mode-install**, and **mode-shell** all source the `live`
   helper; they differ only in what happens after.
3. The `functions` script is sourced globally and provides `pinfo`, `pfatal`,
   `perr`, `cleanup`, `reinit_debug`, and `setup_kernel`.
4. The bootstrap phase already invokes the Go binary (`k3os rc`,
   `k3os config --initrd`).

## 3. Target Go Architecture

### Package hierarchy

```
internal/boot/
    boot.go            -- top-level Run() orchestrator (equivalent to /init)
    bootstrap.go       -- phase 1: early bootstrap
    bootstrap_test.go
    mode.go            -- phase 2: mode detection (extends internal/mode)
    mode_test.go
    handler.go         -- ModeHandler interface
    handler_disk.go    -- mode-disk implementation
    handler_disk_test.go
    handler_local.go   -- mode-local implementation
    handler_local_test.go
    handler_live.go    -- mode-live / mode-install shared logic
    handler_live_test.go
    handler_shell.go   -- mode-shell implementation
    handler_shell_test.go
    finalize.go        -- phase 4: boot finalization
    finalize_test.go
    options.go         -- functional options for Run()
    log.go             -- structured logging helpers (pinfo/pfatal/perr equivalents)
```

### Design principles

- **Single Responsibility**: Each file handles one phase or mode.
- **Open/Closed**: New modes are added by implementing `ModeHandler`, not by
  modifying existing code.
- **Dependency Inversion**: All OS interactions go through interfaces defined
  in `internal/iface`.
- **Liskov Substitution**: Mock implementations satisfy the same contracts.
- **Interface Segregation**: Consumers depend only on the interfaces they use.

## 4. Shell-to-Go Mapping

### 4.1 functions (utility library)

| Shell function | Go equivalent | Package |
|---------------|---------------|---------|
| `pinfo` | `slog.Info` | `log/slog` (stdlib) |
| `perr` | `slog.Error` | `log/slog` (stdlib) |
| `pfatal` | `slog.Error` + `os.Exit(1)` | `log/slog` (stdlib) |
| `reinit_debug` | `boot.initDebug(cmdline []byte)` | `internal/boot` |
| `setup_kernel` | `boot.setupKernel(fs, mounter, loopAttacher)` | `internal/boot` |
| `cleanup` | `boot.cleanup(fs)` | `internal/boot` |
| `$SCRIPTS` / `$K3OS_SYSTEM` | `system.RootPath(...)` / `system.StatePath(...)` | `internal/system` |

### 4.2 bootstrap

| Shell function | Go equivalent | Signature |
|---------------|---------------|-----------|
| `setup_etc` | `Bootstrap.setupEtc` | `(fs FileSystem, mnt Mounter) error` |
| `setup_modules` | `Bootstrap.setupModules` | `(fs FileSystem, mnt Mounter) error` |
| `setup_users` | `Bootstrap.setupUsers` | `(fs FileSystem, cmd CommandRunner) error` |
| `setup_dirs` | `Bootstrap.setupDirs` | `(fs FileSystem) error` |
| `setup_config` | `Bootstrap.setupConfig` | `(cmd CommandRunner, mode string) error` |
| (k3os rc) | existing `internal/cli/rc` | already implemented |

### 4.3 mode detection

| Shell logic | Go equivalent | Notes |
|-------------|---------------|-------|
| Parse `/proc/cmdline` | `boot.parseCmdline(data []byte) ModeParams` | Returns struct with mode, fallback, debug |
| `blkid -L K3OS_STATE` | `boot.probeStatePartition(cmd CommandRunner) bool` | Wraps blkid call |
| Filesystem type check | `boot.isRootTmpfs(fs FileSystem) bool` | `stat -f` equivalent via syscall |
| Wait loop (30s) | `boot.detectMode(ctx, ...) (string, error)` | Context-based timeout |
| Write mode file | `mode.Set(fs, mode string) error` | New helper in `internal/mode` |

### 4.4 mode-disk

| Shell function | Go equivalent | Notes |
|---------------|---------------|-------|
| `grow` | `DiskHandler.growPartition` | Uses parted/e2fsck/resize2fs via CommandRunner |
| `setup_mounts` | `DiskHandler.setupMounts` | Mount K3OS_STATE, handle growpart |
| `setup_kernel_squashfs` | `DiskHandler.setupKernelSquashfs` | Copy squashfs from .base |
| `setup_k3os` | `DiskHandler.setupK3OS` | Copy/symlink k3os binary |
| `setup_init` | `DiskHandler.setupInit` | Symlink sbin/init |
| `setup_k3s` | `DiskHandler.setupK3s` | Symlink k3s current |
| `takeover` | `DiskHandler.takeover` | Factory reset/cleanup |
| `pivot_root` + exec | `DiskHandler.pivotAndExec` | syscall.PivotRoot + syscall.Exec |

### 4.5 mode-local

| Shell function | Go equivalent | Notes |
|---------------|---------------|-------|
| `setup_ssh` | `LocalHandler.setupSSH` | Persist/symlink /etc/ssh |
| `setup_rancher_node` | `LocalHandler.setupRancherNode` | Create /etc/rancher symlink |

### 4.6 live (shared by mode-live, mode-install, mode-shell)

| Shell function | Go equivalent | Notes |
|---------------|---------------|-------|
| `setup_base` | `LiveHandler.setupBase` | Mount K3OS iso or probe USB |
| `setup_passwd` | `LiveHandler.setupPasswd` | Remove rancher password |
| `setup_motd` | `LiveHandler.setupMotd` | Append install instructions |

### 4.7 boot (finalization)

| Shell function | Go equivalent | Notes |
|---------------|---------------|-------|
| `setup_mounts` | `Finalizer.setupMounts` | Bind boot, system; unmount .base |
| `grow_live` | `Finalizer.growLive` | Grow partition in local mode |
| `setup_hostname` | `Finalizer.setupHostname` | Generate or read hostname |
| `setup_hosts` | `Finalizer.setupHosts` | Write /etc/hosts |
| `setup_root` | `Finalizer.setupRoot` | Create /root 0700 |
| `setup_ttys` | `Finalizer.setupTTYs` | Configure inittab + securetty |
| `setup_sudoers` | `Finalizer.setupSudoers` | Write sudoers.d/sudo |
| `setup_services` | `Finalizer.setupServices` | Symlink runlevel scripts |
| `setup_config` | `Finalizer.setupConfig` | Run k3os config --boot |
| `setup_manifests` | `Finalizer.setupManifests` | Copy k3s manifests |
| `setup_state_dirs` | `Finalizer.setupStateDirs` | Create state directories |

## 5. Interface Definitions

All new interfaces follow the existing `internal/iface` pattern. Existing
interfaces (`FileSystem`, `CommandRunner`, `Mounter`, `LoopAttacher`) are
reused directly. New interfaces needed:

### 5.1 ModeHandler

```go
// ModeHandler handles mode-specific boot setup.
type ModeHandler interface {
    // Name returns the mode name (e.g., "disk", "local", "live").
    Name() string
    // Execute performs the mode-specific boot operations.
    // It receives the boot context containing all injected dependencies.
    Execute(ctx context.Context, deps *Dependencies) error
}
```

### 5.2 ModeDetector

```go
// ModeDetector determines the boot mode from system state.
type ModeDetector interface {
    // Detect returns the boot mode string or an error if detection fails.
    Detect(ctx context.Context) (string, error)
}
```

### 5.3 Bootstrapper

```go
// Bootstrapper performs early-boot system initialization.
type Bootstrapper interface {
    // Run executes all bootstrap steps in order.
    Run(ctx context.Context) error
}
```

### 5.4 Finalizer

```go
// Finalizer performs post-mode boot finalization.
type Finalizer interface {
    // Run executes all finalization steps before handing off to /sbin/init.
    Run(ctx context.Context) error
}
```

### 5.5 Dependencies (struct, not interface)

```go
// Dependencies holds all injected OS-level abstractions.
type Dependencies struct {
    FS           iface.FileSystem
    Cmd          iface.CommandRunner
    Mounter      iface.Mounter
    LoopAttacher iface.LoopAttacher
    Logger       *slog.Logger
}
```

### 5.6 BlockDeviceProber (new interface for iface package)

```go
// BlockDeviceProber abstracts block device queries (blkid, lsblk).
type BlockDeviceProber interface {
    // LookupByLabel finds a device path by filesystem label.
    LookupByLabel(label string) (string, error)
    // ListDisks returns block device names of type "disk".
    ListDisks() ([]string, error)
}
```

### 5.7 PartitionResizer (new interface for iface package)

```go
// PartitionResizer abstracts partition grow/resize operations.
type PartitionResizer interface {
    // Grow expands the partition and resizes the filesystem.
    Grow(device string, partNum int, partition string) error
}
```

### 5.8 ProcessExecutor (new interface for iface package)

```go
// ProcessExecutor abstracts process replacement (exec) and pivot_root.
type ProcessExecutor interface {
    // PivotRoot changes the root filesystem.
    PivotRoot(newRoot, putOld string) error
    // Exec replaces the current process image.
    Exec(path string, args []string, env []string) error
}
```

## 6. TDD Strategy

### Red-Green-Refactor cycle

Every function is developed using strict TDD:

1. **Red**: Write a failing test that specifies the expected behavior.
2. **Green**: Write the minimal implementation to pass the test.
3. **Refactor**: Clean up while keeping tests green.

### Test categories

| Category | Scope | Dependencies | Speed |
|----------|-------|-------------|-------|
| Unit | Single function | All mocked via interfaces | < 1ms |
| Integration | Single phase | Mocked OS, real logic | < 10ms |
| E2E | Full boot sequence | Docker container with real FS | ~30s |

### Mocking strategy

- Use `testify/mock` for generating mock implementations of interfaces.
- Each test file creates mock instances and injects them via the
  `Dependencies` struct.
- Table-driven tests cover edge cases (missing files, permission errors,
  timeout scenarios).

### Example test structure

```go
func TestDetectMode_FromCmdline(t *testing.T) {
    tests := []struct {
        name     string
        cmdline  string
        wantMode string
    }{
        {"explicit mode", "k3os.mode=disk quiet", "disk"},
        {"rescue keyword", "rescue quiet", "shell"},
        {"fallback mode", "k3os.fallback_mode=live", "live"},
        {"no mode set", "quiet", ""},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            params := parseCmdline([]byte(tt.cmdline))
            assert.Equal(t, tt.wantMode, params.Mode)
        })
    }
}
```

## 7. SOLID Principle Application

### Single Responsibility Principle (SRP)

Each Go file/struct owns exactly one concern:
- `bootstrap.go` handles early init (mount etc, create users)
- `mode.go` handles mode detection only
- `handler_disk.go` handles disk-mode only
- `finalize.go` handles post-mode boot setup

### Open/Closed Principle (OCP)

The mode system is open for extension:
- Adding a new mode requires only implementing `ModeHandler` and registering
  it in a mode registry map.
- No existing code needs modification.

```go
var handlers = map[string]ModeHandler{
    "disk":    &DiskHandler{},
    "local":   &LocalHandler{},
    "live":    &LiveHandler{},
    "install": &InstallHandler{},
    "shell":   &ShellHandler{},
}
```

### Liskov Substitution Principle (LSP)

All mock implementations satisfy the same behavioral contracts as real
implementations. Tests verify the contract, not the implementation.

### Interface Segregation Principle (ISP)

Consumers depend only on what they need:
- `setupModules` needs `Mounter` + `FileSystem`, not `CommandRunner`
- `setupUsers` needs `CommandRunner` + `FileSystem`, not `Mounter`
- Mode detection needs `FileSystem` + `BlockDeviceProber`, not `Mounter`

### Dependency Inversion Principle (DIP)

High-level orchestration (`Run()`) depends on abstractions (`ModeHandler`,
`Bootstrapper`, `Finalizer`), never on concrete implementations. Concrete
implementations are injected at the composition root (main.go / reexec
handler).

## 8. Phased Implementation Plan

### Phase 1: Foundation (FEAT-003 through FEAT-004)

- Create `internal/boot/` package structure
- Implement `parseCmdline` with full test coverage
- Implement `initDebug` (debug mode detection from cmdline/env/file)
- Define `Dependencies` struct and `ModeHandler` interface
- Add new interfaces to `internal/iface/`

### Phase 2: Mode Detection (FEAT-005)

- Implement `ModeDetector` with timeout and retry logic
- Probe for K3OS_STATE via `BlockDeviceProber`
- Detect local mode via filesystem type check
- Full table-driven tests for all detection paths

### Phase 3: Bootstrap (FEAT-006)

- Implement `Bootstrapper` with all setup functions
- Each sub-function tested independently
- Integrate with existing `internal/cli/rc` for cloud-config

### Phase 4: Mode Handlers (FEAT-007)

- Implement each `ModeHandler`:
  - `DiskHandler` (most complex: grow, pivot, exec)
  - `LocalHandler` (SSH + rancher node setup)
  - `LiveHandler` (shared base for live/install)
  - `ShellHandler` (live + exec bash)
  - `InstallHandler` (delegates to LiveHandler)

### Phase 5: Boot Finalization (FEAT-008)

- Implement `Finalizer` with all setup functions
- TTY configuration from cmdline parsing
- Service runlevel symlinks
- Hostname/hosts generation
- Cleanup and handoff to /sbin/init

### Phase 6: Integration and Wiring

- Wire `boot.Run()` as the top-level orchestrator
- Register as a reexec handler (following existing `enterchroot` pattern)
- E2E tests in Docker verifying full boot sequence

## 9. Error Handling Strategy

Following Go idioms and the existing codebase patterns:

- All functions return `error` as the last return value.
- Errors are wrapped with context using `fmt.Errorf("...: %w", err)`.
- Fatal errors (cannot continue boot) log with `slog.Error` and return,
  allowing the top-level `Run()` to decide recovery (rescue shell).
- Non-fatal errors (optional feature unavailable) log with `slog.Warn` and
  continue.
- The rescue shell pattern from `/init` is preserved: if any phase fails,
  drop to an interactive shell for debugging.

## 10. Build Tags

Linux-specific code (syscalls, mount operations, pivot_root) uses build tags:

```go
//go:build linux
```

This follows the existing pattern in `internal/enterchroot/enter.go` and
allows the package to compile (with stubs or no-ops) on non-Linux platforms
for development and testing convenience.

## 11. Logging

All logging uses `log/slog` (structured logging), consistent with the existing
codebase:

| Shell | Go | Level |
|-------|-----|-------|
| `pinfo "msg"` | `slog.Info("msg")` | INFO |
| `perr "msg"` | `slog.Error("msg")` | ERROR |
| `pfatal "msg"` | `slog.Error("msg"); return err` | ERROR (caller exits) |
| `set -x` (debug) | `slog.Debug("msg")` | DEBUG |

Debug logging is enabled when:
- `K3OS_DEBUG=true` environment variable is set
- `/run/k3os/debug` file exists
- `k3os.debug` appears in `/proc/cmdline`

## 12. Relationship to Existing Code

The new `internal/boot/` package integrates with, but does not replace,
existing packages:

| Existing package | Relationship |
|-----------------|--------------|
| `internal/enterchroot` | Remains as-is; handles squashfs mount + pivot into rootfs. The boot package runs *after* enterchroot completes. |
| `internal/mode` | Extended with `Set()` function; `Get()` continues to work as before. |
| `internal/iface` | New interfaces added alongside existing ones. |
| `internal/iface/osimpl` | Real implementations added for new interfaces. |
| `internal/cc` | Called by bootstrap phase (existing `k3os rc` functionality). |
| `internal/system` | Path constants reused throughout boot package. |
| `internal/config` | Called by bootstrap (`config --initrd`) and finalize (`config --boot`). |

## 13. Success Criteria

The port is considered complete when:

1. All shell script functionality is replicated in Go with equivalent behavior.
2. Every function has unit test coverage.
3. Table-driven tests cover edge cases and error paths.
4. E2E tests in Docker demonstrate a successful boot sequence.
5. The shell scripts (`overlay/init`, `overlay/libexec/k3os/*`) can be removed
   from the k3os repository, replaced by the Go binary.
6. `golangci-lint run ./...` passes with no issues.
7. The binary size remains reasonable (no unnecessary dependencies).

## 14. Implementation Notes

This section documents what was actually implemented during the port, any
deviations from the original plan, and how the final architecture differs from
the spec above.

### 14.1 Summary of What Was Implemented

All four major phases of the init sequence were ported to Go, plus a top-level
orchestrator that wires them together:

1. **Bootstrap** (`internal/boot/bootstrap/`): SetupEtc, SetupModules,
   SetupUsers, RunRC, SetupDirs, SetupKernel, SetupConfig - matching the
   original `overlay/libexec/k3os/bootstrap` shell script.

2. **Mode Detection** (`internal/mode/`): The `Detector` struct implements
   cmdline parsing, block device probing via blkid, root filesystem type
   detection via statfs, and a retry loop with configurable timeout -
   matching the original `overlay/libexec/k3os/mode` shell script.

3. **Mode Handlers** (`internal/boot/modes/`): Five handlers covering all
   boot modes:
   - `DiskHandler` - partition grow, mount state, setup k3os/k3s, pivot_root
   - `LocalHandler` - SSH persistence, rancher node setup
   - `LiveHandler` - base image mount, password removal, motd setup
   - `InstallHandler` - delegates to LiveHandler
   - `ShellHandler` - delegates to LiveHandler, then execs bash

4. **Boot Finalization** (`internal/boot/finalize/`): SetupMounts, GrowLive,
   SetupHostname, SetupHosts, SetupRoot, SetupTTYs, SetupSudoers,
   SetupServices, SetupConfig, SetupManifests, SetupStateDirs, and Cleanup -
   matching the original `overlay/libexec/k3os/boot` shell script.

5. **Orchestrator** (`internal/boot/init.go`): The `Init` struct runs all
   phases in sequence with error handling and rescue-shell fallback, matching
   the flow of the original `overlay/init` shell script.

### 14.2 Deviations From the Original Plan

1. **Mode detection lives in `internal/mode/`, not `internal/boot/`**: The
   existing `internal/mode` package already handled mode reading/writing. The
   `Detector` struct was added there (with a `Detect` method) rather than
   creating a separate detection file in `internal/boot/`. The orchestrator
   wraps it via a `ModeDetectorFunc` closure.

2. **Package structure differs from spec**: Instead of a flat set of files
   under `internal/boot/`, the implementation uses sub-packages:
   - `internal/boot/bootstrap/` (not `internal/boot/bootstrap.go`)
   - `internal/boot/modes/` (not `internal/boot/handler_*.go`)
   - `internal/boot/finalize/` (not `internal/boot/finalize.go`)
   - `internal/boot/init.go` is the orchestrator (not `internal/boot/boot.go`)

3. **Finalize includes SetupConfig and SetupRoot**: The original spec listed
   these but the implementation adds concrete handling for writing
   `/root` with 0700 permissions and invoking `k3os config --boot`.

4. **No separate interfaces in `internal/iface/` for new abstractions**:
   Rather than adding `BlockDeviceProber`, `PartitionResizer`, and
   `ProcessExecutor` as formal interfaces in the shared `iface` package, each
   package defines its own dependency interfaces locally:
   - `modes.ProcessExecutor` in `internal/boot/modes/handler.go`
   - Mode detection dependencies are function fields on `mode.Detector`
   - Bootstrap/Finalize use `iface.FileSystem`, `iface.Mounter`, and
     `iface.CommandRunner` directly

5. **Functional dependency injection**: The `mode.Detector` uses function
   fields (e.g., `CmdlineReader`, `BlockProber`, `StatfsChecker`) rather than
   interface types. This makes testing simpler since test functions can be
   assigned directly without creating mock structs.

6. **Finalize has individual files per function**: Each finalization step is
   in its own file (hostname.go, ttys.go, services.go, etc.) for clarity,
   rather than one large finalize.go file.

### 14.3 Architecture

The entry point is `main.go`, which registers an "init" reexec handler.
When the binary is invoked as PID 1 (`/init`), it enters the `initrd()`
function. This function detects whether it is running pre-chroot or
post-chroot by checking for the `/.base` sentinel file:

- **Pre-chroot**: Calls `transferroot.Relocate()`, remounts root as rw, and
  enters the chroot via `enterchroot.Mount()`. This is unchanged from the
  existing code.
- **Post-chroot** (new): Calls `postChroot()` which wires up all real OS
  dependencies and invokes `boot.Init.Run()`. This replaces the original
  `/usr/init` shell script that was previously exec'd after entering the
  chroot.

### 14.4 Package Structure

```
main.go                              -- reexec registration, postChroot() wiring
internal/boot/
    init.go                          -- Init orchestrator struct and Run() method
    init_test.go                     -- orchestrator unit tests
internal/boot/bootstrap/
    bootstrap.go                     -- Bootstrapper struct, all setup functions
    bootstrap_test.go                -- table-driven unit tests
    mock_test.go                     -- testify mock definitions
internal/boot/modes/
    handler.go                       -- ModeHandler interface, Deps, Registry
    handler_test.go                  -- registry tests
    disk.go                          -- DiskHandler
    disk_test.go
    local.go                         -- LocalHandler
    local_test.go
    live.go                          -- LiveHandler (shared by live/install)
    live_test.go
    shell.go                         -- ShellHandler
    shell_test.go
    mock_test.go                     -- shared mock definitions
internal/boot/finalize/
    finalize.go                      -- Finalizer struct with Run() sequence
    finalize_test.go                 -- integration test for step ordering
    mounts.go, grow.go, hostname.go  -- individual step implementations
    ttys.go, sudoers.go, services.go
    config.go, manifests.go
    statedirs.go, cleanup.go
    mock_test.go                     -- shared mock definitions
internal/mode/
    mode.go                          -- Get/Set functions (pre-existing)
    detect.go                        -- Detector struct with Detect() method
    detect_test.go                   -- table-driven detection tests
```

### 14.5 Testing Approach

- **Interface-driven DI**: All OS interactions (filesystem, mounts, process
  execution, command running) are injected via interfaces or function fields.
  No test touches the real filesystem or invokes real system commands.
- **testify/mock**: Mock implementations are generated using
  `github.com/stretchr/testify/mock` and defined in `mock_test.go` files
  within each package.
- **Table-driven tests**: All test functions use the standard Go table-driven
  pattern with `t.Run()` subtests, covering success paths, error paths, and
  edge cases.
- **Independent testability**: Each function (e.g., `SetupHostname`,
  `SetupTTYs`) can be tested in isolation by constructing a `Finalizer` or
  `Bootstrapper` with only the mocks that function needs.
- **Orchestrator tested with full mock graph**: `init_test.go` verifies the
  end-to-end flow (bootstrap, detect, mode execute, finalize, exec) using
  mock runners that track call order and simulate failures.
