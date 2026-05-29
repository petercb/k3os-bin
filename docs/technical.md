# Technical Specification — k3os-bin

## Overview

The `k3os-bin` project is a Go application that produces a single, statically-linked binary (`k3os`) serving multiple roles in the K3OS Linux distribution. This document defines the engineering patterns, conventions, and technical decisions governing the codebase.

---

## Technology Stack

| Component | Technology | Version |
|-----------|-----------|---------|
| Language | Go | 1.24 |
| CLI Framework | `urfave/cli` | v1.22.9 (target: v3) |
| YAML | `gopkg.in/yaml.v3` | v3.0.1 |
| Config Decode | `go-viper/mapstructure/v2` | v2.5.0 |
| Config Merge | `dario.cat/mergo` | v1.0.2 |
| Logging | `log/slog` (stdlib) | - |
| Module Loading | `pault.ag/go/modprobe` | v0.1.2 |
| Container Reexec | `moby/moby/pkg/reexec` | v20.10.17 (target: `moby/sys/reexec`) |
| File Copy | `otiai10/copy` | v1.7.0 |
| Loop Devices | `internal/loopdev` (golang.org/x/sys/unix ioctls) | - |
| Glob Matching | `path` (stdlib) | - |
| Testing | `stretchr/testify` | v1.11.1 |
| Linting | `golangci-lint` | v2 |
| Build/Release | GoReleaser | v2 |
| CI/CD | CircleCI | v2.1 |

---

## Design Principles

### SOLID Principles

- **Single Responsibility**: Each `internal/` package owns exactly one concern (e.g., `mount` only mounts, `hostname` only sets hostnames). The `cc` package orchestrates appliers but delegates to domain packages.
- **Open/Closed**: The applier pattern in `cc/apply.go` is open for extension (add new `applier` functions) without modifying existing applier logic.
- **Liskov Substitution**: Introduce interfaces for OS-dependent operations so mocks can substitute real implementations in tests.
- **Interface Segregation**: Keep interfaces small and focused (e.g., `Mounter`, `ModuleLoader`, `FileWriter`).
- **Dependency Inversion**: High-level modules (CLI commands, appliers) should depend on abstractions (interfaces), not concrete OS calls.

### DRY (Don't Repeat Yourself)

- Common path construction is centralized in `internal/system` (`RootPath`, `DataPath`, `LocalPath`, `StatePath`).
- Config reading/merging is centralized in `internal/config`.
- The applier pattern in `cc/apply.go` provides a uniform error-aggregation mechanism.

### KISS (Keep It Simple, Stupid)

- The binary's multi-personality behavior is achieved through a simple `reexec.Register` + `reexec.Init()` pattern rather than complex plugin systems.
- Each CLI subcommand is a self-contained function returning a `cli.Command`.

---

## Directory Structure

```text
k3os-bin/
├── main.go                          # Entry point, reexec registration, CLI bootstrap
├── go.mod / go.sum                  # Go module definition
├── docs/                            # Project documentation
│   ├── PRD.md                       # Product requirements
│   ├── technical.md                 # This file
│   ├── architecture.mermaid         # Architecture diagram
│   └── unit_testing_guideline.md    # Testing conventions
├── tasks/                           # Implementation plans
│   └── tasks.md                     # Task tracking
├── tests/                           # Integration / end-to-end tests (future)
├── internal/                        # All internal packages (not exported)
│   ├── cc/                          # Cloud-config applier orchestration
│   │   ├── apply.go                 # Phase-based applier chains
│   │   └── funcs.go                 # Individual applier functions
│   ├── cli/                         # CLI command definitions
│   │   ├── app/app.go               # Root CLI app setup
│   │   ├── config/config.go         # `config` subcommand
│   │   ├── install/install.go       # `install` subcommand
│   │   ├── rc/rc.go                 # `rc` subcommand (run control)
│   │   └── upgrade/upgrade.go       # `upgrade` subcommand
│   ├── cliinstall/                  # Interactive install wizard
│   │   ├── ask.go                   # User prompts
│   │   └── install.go               # Install execution
│   ├── command/command.go           # Shell command execution wrapper
│   ├── config/                      # Configuration model and I/O
│   │   ├── config.go                # Data model (CloudConfig, K3OS, Install, etc.)
│   │   ├── read.go                  # Config reading, merging, cmdline parsing
│   │   ├── read_cc.go               # Cloud-config data source reading
│   │   ├── read_test.go             # Existing test
│   │   ├── write.go                 # Config serialization
│   │   ├── coerce.go                # Type coercion mappers
│   │   └── rename.go                # Field name mapping
│   ├── enterchroot/                 # Chroot/pivot-root for boot
│   │   ├── enter.go                 # Main enter logic, squashfs mount
│   │   └── ensureloop.go            # Loop device setup
│   ├── hostname/hostname.go         # Hostname management
│   ├── modalias/modalias.go         # Module alias resolution
│   ├── mode/mode.go                 # Boot mode detection (live/local)
│   ├── module/module.go             # Kernel module loading
│   ├── mount/                       # Mount abstraction
│   │   ├── mount.go                 # Mount/ForceMount/Mounted
│   │   ├── flags.go                 # Non-Linux flag stubs
│   │   └── flags_linux.go           # Linux mount flag parsing
│   ├── questions/questions.go       # Interactive user prompts
│   ├── ssh/ssh.go                   # SSH authorized key management
│   ├── sysctl/sysctl.go             # Sysctl application
│   ├── system/                      # System path constants and component management
│   │   ├── system.go                # RootPath, DataPath, LocalPath, StatePath
│   │   └── component.go             # Component version/copy for upgrades
│   ├── transferroot/transferroot.go # Root filesystem relocation from ramfs
│   ├── util/                        # Shared utilities
│   │   ├── util.go                  # File helpers, symlink resolution
│   │   ├── decode.go                # Base64/gzip decoding
│   │   └── prompt.go                # Password prompt
│   ├── version/version.go           # Version variable (set via ldflags)
│   └── writefile/writefile.go       # File writing with permissions/encoding
├── .circleci/config.yml             # CI/CD pipeline
├── .golangci.yaml                   # Linter configuration
├── .goreleaser.yaml                 # Release configuration
└── .pre-commit-config.yaml          # Pre-commit hooks
```

---

## Build System

### Build Command

```bash
# Development build
GOOS=linux go build -o k3os .

# Production build (matches GoReleaser)
GOOS=linux CGO_ENABLED=0 go build -ldflags "-s -w -extldflags -static -X github.com/petercb/k3os-bin/internal/version.Version=$(VERSION)" -tags static_build -o k3os .
```

### GoReleaser Configuration

- Produces static binaries for `linux/amd64` and `linux/arm64`
- Version injected via `-ldflags -X`
- Archives as `.tar.gz`
- Releases as drafts with semantic versioning

### CI/CD Pipeline (CircleCI)

```text
Workflow: continuous
  ├── test (all branches)
  │   ├── checkout
  │   ├── go mod download
  │   ├── go test -race -covermode=atomic -failfast
  │   └── go build + smoke run
  └── release (master only, requires test)
      ├── checkout
      └── go-semantic-release --hooks goreleaser
```

---

## Coding Conventions

### Go Standards

- **Formatting**: `gofumpt` + `goimports` (enforced by golangci-lint)
- **Error handling**: Prefer `fmt.Errorf("context: %w", err)` over `errors.Wrap` (modernization target)
- **Naming**: Follow Go conventions (exported = PascalCase, unexported = camelCase)
- **Package naming**: Short, single-word, lowercase (e.g., `mount`, `config`, `ssh`)
- **Constants**: Use `const` blocks; avoid magic numbers
- **Function length**: Target < 30 lines of code per function

### Logging

- Use `log/slog` (Go stdlib) for structured logging throughout
- Debug-level for detailed operational info
- Error-level for unrecoverable failures (followed by `os.Exit(1)` where fatal)
- Debug logging is enabled via the `--debug` flag in the CLI app

### Error Handling Patterns

```go
// Preferred (modern Go)
if err != nil {
    return fmt.Errorf("failed to mount %s: %w", target, err)
}

// Deprecated (to be replaced)
if err != nil {
    return errors.Wrapf(err, "failed to mount %s", target)
}
```

---

## Linting Configuration

Enabled linters (golangci-lint v2):

| Linter | Purpose |
|--------|---------|
| `bodyclose` | Ensure HTTP response bodies are closed |
| `gocognit` | Cognitive complexity limits |
| `gocritic` | Style and performance suggestions |
| `govet` (+ shadow) | Correctness checks including variable shadowing |
| `misspell` | Typo detection |
| `revive` | Comprehensive Go linting |
| `unconvert` | Remove unnecessary type conversions |
| `unparam` | Detect unused function parameters |
| `unused` | Detect unused code |

Formatters: `gofumpt`, `goimports`

Run with:

```bash
golangci-lint run --fix ./...
```

On non-Linux development hosts, run tests and lint inside a Linux environment; see `docs/unit_testing_guideline.md` (Docker section).

---

## Workflow & Validation

### Git commits

Commit messages follow [Conventional Commits](https://www.conventionalcommits.org/):

```text
<type>(<optional scope>): <short description>

<optional body>
```

Common types:

| Type | Use for |
|------|---------|
| `feat` | New behavior or capability |
| `fix` | Bug fixes |
| `test` | Tests only (adding or updating tests, test tooling) |
| `docs` | Documentation only |
| `chore` | Tooling, dependencies, CI, housekeeping |
| `refactor` | Code changes that are not fixes or features |

Examples:

```text
test: add testify smoke tests for version package

docs: document Docker-based test runs on macOS
```

Match existing history in the repository (for example `feat:`, `fix:`, `doc:`). Prefer `docs:` for documentation-only changes; `doc:` appears in older commits.

Branching uses `master` only (no long-lived `develop` branch). Feature branches: `feature/<task-id>-<short-slug>`.

To maintain consistency and correctness across documentation, configuration, and CI/CD pipelines, the following validation commands must be executed after making changes:

### Markdown Files

After modifying any Markdown (`.md`) files, run:

```bash
markdownlint-cli2 "**/*.md" "#node_modules"
```

### YAML Files

After modifying any YAML (`.yaml` or `.yml`) files, format and lint them:

```bash
yamlfmt .
yamllint .
```

### CircleCI Configuration

After modifying `.circleci/config.yml` or related CI config files, validate the configuration:

```bash
circleci config validate
```

---

## Key Design Patterns

### Multi-Personality Binary (Reexec Pattern)

The binary registers handlers for different `argv[0]` values:

- `/init` → live boot initrd sequence
- `/sbin/init` → local boot initrd sequence
- `enter-root` → chroot entry
- `k3os` → CLI app with subcommands

This allows a single binary to serve multiple roles via symlinks.

### Applier Chain Pattern

Cloud-config application uses a chain-of-responsibility pattern:

```go
type applier func(cfg *config.CloudConfig) error

func runApplies(cfg *config.CloudConfig, appliers ...applier) error {
    // Collects all errors, returns as MultiError
}
```

Different boot phases compose different applier chains:

- `InitApply`: modules, sysctls, hostname, write-files, environment, initcmd
- `BootApply`: data sources + init appliers + DNS, WiFi, passwords, SSH, k3s, bootcmd
- `InstallApply`: k3s with restart
- `RunApply`: full set including runcmd

### Config Merging Strategy

Configuration is read from multiple sources and merged in priority order:

1. System config (`/k3os/system/config.yaml`)
2. Local config (`/var/lib/rancher/k3os/config.yaml`)
3. Config directory (`/var/lib/rancher/k3os/config.d/*.yaml`)
4. Cloud-config data sources
5. Kernel command line parameters (`k3os.*`)

The `go-viper/mapstructure/v2` library provides struct decoding with custom hooks for type coercion (string to bool, string to []string, map normalization) and fuzzy field name matching. `dario.cat/mergo` handles deep map merging with override semantics.

### Component Version Management

Upgrades use a symlink-based version scheme:

```text
/k3os/system/<component>/
  ├── current -> v1.2.3/
  ├── previous -> v1.2.2/
  ├── v1.2.3/
  └── v1.2.2/
```

`CopyComponent` atomically swaps `current` to the new version while preserving `previous` for rollback.

---

## Interfaces for Testability (To Be Introduced)

To enable unit testing of OS-dependent code, introduce interfaces:

```go
// internal/mount
type Mounter interface {
    Mount(device, target, mType, options string) error
    Mounted(target string) (bool, error)
}

// internal/module
type ModuleLoader interface {
    LoadModules(cfg *config.CloudConfig) error
}

// internal/sysctl
type SysctlApplier interface {
    ConfigureSysctl(cfg *config.CloudConfig) error
}
```

These enable `testify/mock` implementations for isolated unit testing.

---

## Security Considerations

- Most operations require root (`os.Getuid() == 0` checks)
- SSH keys are written with restrictive permissions (`0600` for keys, `0700` for `.ssh`)
- Password hashing uses `chpasswd` system command
- Static binary with no CGO eliminates native library attack surface
- Config files may contain sensitive data (WiFi passphrases, tokens) — handle with care

---

## Performance Considerations

- Binary is statically linked (~13MB) for fast load without dynamic linking
- Boot path is critical — minimize I/O and syscalls in `transferroot` and `enterchroot`
- Module loading checks `/proc/modules` first to avoid redundant `modprobe` calls
- Config merging happens once at boot, not on every access
