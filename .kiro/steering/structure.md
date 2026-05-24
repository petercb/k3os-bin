# Project Structure

## Top-Level Layout

```
k3os-bin/
├── main.go                  # Entry point: reexec registration + CLI bootstrap
├── go.mod / go.sum          # Go module definition
├── internal/                # All application code (unexported)
├── docs/                    # Project documentation (PRD, technical spec, etc.)
├── tasks/                   # Implementation task tracking
├── .circleci/config.yml     # CI/CD pipeline
├── .golangci.yaml           # Linter configuration
├── .goreleaser.yaml         # Release build configuration
└── .pre-commit-config.yaml  # Pre-commit hooks
```

## `internal/` Package Map

Each package owns exactly one concern. No package should reach into another's internals.

| Package | Responsibility |
|---------|---------------|
| `cli/app` | Root `urfave/cli` app setup, subcommand registration |
| `cli/config` | `k3os config` subcommand |
| `cli/install` | `k3os install` subcommand |
| `cli/rc` | `k3os rc` subcommand (run control) |
| `cli/upgrade` | `k3os upgrade` subcommand |
| `cliinstall` | Interactive install wizard (prompts + execution) |
| `cc` | Cloud-config applier orchestration (`apply.go` chains, `funcs.go` individual appliers) |
| `config` | Config data model, reading/merging, writing, type coercion, field renaming |
| `command` | Shell command execution wrapper |
| `enterchroot` | Chroot/pivot-root for boot sequence; loop device setup |
| `hostname` | Hostname get/set |
| `iface` | OS boundary interfaces (`FileSystem`, `CommandRunner`, `Mounter`, `ModuleLoader`, `SysctlApplier`, `HostnameSetter`) |
| `iface/osimpl` | Real OS implementations of `iface` interfaces |
| `modalias` | Kernel module alias resolution |
| `mode` | Boot mode detection (live vs. local) |
| `module` | Kernel module loading via modprobe |
| `mount` | Mount/unmount/check with Linux flag parsing |
| `questions` | Interactive user prompts |
| `ssh` | SSH authorized key management |
| `sysctl` | Sysctl application |
| `system` | Canonical path helpers (`RootPath`, `DataPath`, `LocalPath`, `StatePath`) and component version/symlink management |
| `transferroot` | Root filesystem relocation from ramfs/tmpfs |
| `util` | Shared helpers: file utilities, base64/gzip decode, password prompt |
| `version` | Version variable (injected via ldflags at build time) |
| `writefile` | File writing with encoding and permission support |

## Key Architectural Patterns

### Multi-Personality Binary

`main.go` uses `reexec.Register` to map `argv[0]` values to handlers. `reexec.Init()` runs the registered handler if matched, otherwise falls through to the CLI app. Symlinks to the binary enable different personalities.

### Applier Chain (`cc` package)

`cc/apply.go` defines an `applier func(*config.CloudConfig) error` type and composes phase-specific chains (`InitApply`, `BootApply`, `InstallApply`, `RunApply`). Errors are collected across all appliers rather than failing fast.

### Interface-Based OS Abstraction (`iface` package)

OS-dependent operations are defined as interfaces in `internal/iface`. Real implementations live in `internal/iface/osimpl`. This pattern enables `testify/mock` substitution in unit tests without requiring root or Linux.

### Config Merging

`internal/config` reads from multiple sources in priority order: system YAML → local YAML → `config.d/` directory → cloud-config data sources → kernel cmdline (`k3os.*` params). `rancher/mapper` handles type coercion and fuzzy field name matching.

### Component Version Symlinks

Upgrades in `internal/system/component.go` maintain `current` → `vX.Y.Z/` and `previous` → `vX.Y.Z/` symlinks under `/k3os/system/<component>/` for atomic swap and rollback.

## Coding Conventions

- **Formatting**: `gofumpt` + `goimports` (enforced by golangci-lint)
- **Error wrapping**: `fmt.Errorf("context: %w", err)` — avoid `github.com/pkg/errors`
- **Logging**: `logrus` throughout; avoid stdlib `log` except in early-boot code that predates logrus init
- **Package names**: short, single-word, lowercase
- **Function length**: target < 30 lines
- **Test files**: co-located with source (`*_test.go`), use `testify/assert` + `testify/require`; fixtures in `testdata/` subdirectories
- **Linux-only code**: guard with `//go:build linux` build tag
- **No exported packages**: everything lives under `internal/`
