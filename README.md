# k3os binary

![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/petercb/k3os-bin)
![GitHub release (latest SemVer)](https://img.shields.io/github/v/release/petercb/k3os-bin?label=release&sort=semver)
[![CircleCI](https://dl.circleci.com/status-badge/img/gh/petercb/k3os-bin/tree/master.svg?style=svg)](https://dl.circleci.com/status-badge/redirect/gh/petercb/k3os-bin/tree/master)

This is the k3os binary cli used in [k3OS](https://github.com/petercb/k3os)

## Init Subsystem

The init subsystem (`internal/boot/`) handles the entire early-userspace boot
sequence, from mounting filesystems and detecting the boot mode through to
handing off control to `/sbin/init`. This replaces the legacy shell scripts
with a testable, type-safe Go implementation.

See [docs/spec-port-init-scripts.md](docs/spec-port-init-scripts.md) for the
full specification.
