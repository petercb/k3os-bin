# Product Requirements Document — k3os-bin

## Product Vision

The `k3os-bin` project produces the core `k3os` binary used by the [K3OS](https://github.com/petercb/k3os) Linux distribution. This single, statically-linked binary serves as the Swiss-army knife of the OS: it is the init program, the configuration applier, the upgrade orchestrator, and the run-control manager. By consolidating these responsibilities into one binary with a multi-personality design (via `reexec`), K3OS achieves a minimal, container-optimized OS footprint that "boots to k3s so you don't have to."

The current phase focuses on **modernization**, **improved test coverage**, and **selective feature enhancements** to ensure the codebase remains maintainable, robust, and ready for future expansion (e.g., riscv64 support, potential shell-init incorporation).

---

## Goals & Success Criteria

| # | Goal | Success Criteria |
|---|------|-----------------|
| 1 | **Modernize dependencies** | Upgrade Go version (≥1.22), migrate `urfave/cli` v1→v3, replace deprecated packages |
| 2 | **Improve test coverage** | Achieve ≥60% unit test coverage across `internal/` packages, with interface-based mocking for OS-dependent code |
| 3 | **Establish development workflow** | CI pipeline runs tests, linting, and builds on every push; semantic release on `master` |
| 4 | **Maintain backward compatibility** | Existing `config.yaml` schemas continue to work without migration |
| 5 | **Prepare for future architectures** | Build infrastructure supports adding `linux/riscv64` without structural changes |
| 6 | **Documentation** | Complete PRD, technical spec, architecture diagram, and testing guidelines |

---

## User Personas / Stakeholders

| Persona | Description |
|---------|-------------|
| **Distribution Maintainer** (primary) | Builds and maintains the K3OS distribution; directly develops and tests this binary |
| **K3OS End User** | Operators who configure K3OS via `config.yaml` and interact with `k3os` CLI commands |
| **CI/CD System** | Automated pipelines that build, test, and release the binary |

---

## User Flow

```text
Boot Sequence (init mode):
┌──────────────┐     ┌───────────────┐     ┌──────────────┐     ┌─────────────┐
│  BIOS/UEFI   │────▶│  /init or     │────▶│ transferroot │────▶│ enterchroot │
│  loads kernel │     │  /sbin/init   │     │ Relocate()   │     │ Mount()     │
└──────────────┘     └───────────────┘     └──────────────┘     └─────────────┘
                                                                       │
                                                                       ▼
                                                               ┌─────────────┐
                                                               │ Shell init  │
                                                               │ (hand-off)  │
                                                               └─────────────┘

CLI Usage (user mode):
┌──────────────┐     ┌───────────────┐     ┌──────────────────────────┐
│  User runs   │────▶│  k3os <cmd>   │────▶│  rc | config | install  │
│  k3os binary │     │  or symlink   │     │  | upgrade               │
└──────────────┘     └───────────────┘     └──────────────────────────┘
```

### Detailed User Flows

1. **Boot (init)**: Kernel loads the binary as `/init` (live) or `/sbin/init` (local). The binary relocates the root filesystem from ramfs/tmpfs, remounts root as read-write, mounts the squashfs data image, sets up an overlay filesystem, then hands off to the shell init system.

2. **Run Control (`k3os rc`)**: Early-boot phase that mounts essential filesystems (proc, sys, dev, cgroups), populates device nodes and `/dev/disk/by-label` symlinks (pure Go), sets the hardware clock, configures loopback networking, sets the hostname, and writes resolv.conf.

3. **Configuration (`k3os config`)**: Reads and merges `config.yaml` from multiple sources (system, local, cloud-config data sources, kernel cmdline), then applies configuration in phases:
   - `--initrd`: modules, sysctls, hostname, write-files, environment, initcmd
   - `--boot`: data sources, modules, sysctls, hostname, DNS, WiFi, passwords, SSH keys, k3s, write-files, environment, bootcmd
   - `--install`: k3s with restart
   - `--dump` / `--dump-json`: output current merged configuration

4. **Installation (`k3os install`)**: Interactive wizard for installing K3OS to disk (hidden in "local" mode).

5. **Upgrade (`k3os upgrade`)**: Copies upgraded components (kernel, rootfs/k3s/k3os) from a source directory to the system root, managing version symlinks (`current`/`previous`), with optional remount, sync, and reboot.

---

## User Stories / Use Cases

| ID | Story |
|----|-------|
| US-1 | As a distribution maintainer, I want the binary to boot K3OS reliably from both live ISO and local disk installations |
| US-2 | As an operator, I want to define my system configuration in `config.yaml` and have it applied consistently at boot |
| US-3 | As an operator, I want to upgrade K3OS components (kernel, rootfs) in-place with automatic rollback support via `previous` symlinks |
| US-4 | As a distribution maintainer, I want comprehensive tests so I can refactor with confidence |
| US-5 | As a distribution maintainer, I want the CI pipeline to catch regressions before they reach `master` |
| US-6 | As a distribution maintainer, I want the codebase modernized to current Go standards and dependencies |
| US-7 | As an operator, I want the install wizard to guide me through disk installation interactively |

---

## Features & Requirements

### Core Features (Existing)

| Feature | Description | Status |
|---------|-------------|--------|
| **Multi-personality binary** | Single binary acts as init, CLI app, or enter-root based on `argv[0]` via `reexec` | ✅ Implemented |
| **Init / boot sequence** | `transferroot` → `enterchroot` → hand-off to shell init | ✅ Implemented |
| **Run control (`rc`)** | Mount essential filesystems, hotplug, clock, loopback, hostname, DNS | ✅ Implemented |
| **Cloud-config application** | Phase-based applier system (init, boot, install, run) for 12+ config sections | ✅ Implemented |
| **Config merging** | Multi-source config: system YAML, local YAML, config.d directory, kernel cmdline, cloud-config data sources | ✅ Implemented |
| **OS upgrade** | Component-based upgrade with version symlink management and optional reboot | ✅ Implemented |
| **Interactive installer** | CLI wizard for disk installation | ✅ Implemented |
| **Module loading** | Kernel module loading via `modprobe` and `modalias` matching | ✅ Implemented |

### Modernization Requirements

| Requirement | Priority |
|-------------|----------|
| Upgrade Go to ≥1.22 | High |
| Migrate `urfave/cli` v1 → v3 | Medium |
| Replace `github.com/pkg/errors` with `fmt.Errorf` + `%w` | Medium |
| Migrate `moby/moby/pkg/reexec` to `github.com/moby/sys/reexec` | Medium |
| Evaluate replacing `github.com/rancher/mapper` with standard alternatives | Done |

### Future Enhancements

| Requirement | Priority |
|-------------|----------|
| Add `linux/riscv64` to GoReleaser build matrix | Low |
| Integrate `whydeadcode` analysis (`https://github.com/aarzilli/whydeadcode`) | Low |
| Create Dependabot configuration for automated dependency updates | Low |

### Testing Requirements

| Requirement | Priority |
|-------------|----------|
| Add unit tests for `internal/config` (read, write, merge, coerce) | High |
| Add unit tests for `internal/cc` applier functions | High |
| Add unit tests for `internal/system` component management | High |
| Introduce interfaces for OS-dependent operations (mount, modprobe, file I/O) to enable mocking | High |
| Standardize on `testify` for assertions and mocking | High |
| Achieve ≥60% coverage across `internal/` packages | Medium |

---

## Out of Scope

- **Kernel build**: Occurs in a separate repository
- **Root filesystem assembly**: Occurs in a separate repository
- **Final K3OS distribution image build**: Occurs in a separate repository
- **Shell init system**: Currently external; may be incorporated in a future phase after modernization
- **K3s itself**: K3OS installs/configures k3s but does not build it

---

## Constraints & Assumptions

| Constraint | Detail |
|-----------|--------|
| **Static binary** | Must compile with `CGO_ENABLED=0` and `-extldflags -static` |
| **Linux only** | Targets `linux/amd64` and `linux/arm64`; `linux/riscv64` in the future |
| **Root required** | Most subcommands (`rc`, `config`, `install`, `upgrade`) require root privileges |
| **Config compatibility** | Existing `config.yaml` schema must remain backward-compatible |
| **Branching** | Single `master` branch; no `develop` branch |
| **CI/CD** | CircleCI with `go-semantic-release` and GoReleaser |
| **License** | Apache-2.0 |

---

## Acceptance Criteria

1. All existing functionality continues to work unchanged
2. CI pipeline passes: tests, linting, build for all target architectures
3. Test coverage increases from near-zero to ≥60%
4. Dependencies are modernized without breaking backward compatibility
5. Documentation (PRD, technical, architecture, testing guidelines) is complete and accurate

---

## Metrics / KPIs

| Metric | Target |
|--------|--------|
| Unit test coverage | ≥60% across `internal/` |
| CI pass rate | ≥95% on `master` |
| Build time | <5 minutes for full CI pipeline |
| Open lint issues | 0 on `master` |
