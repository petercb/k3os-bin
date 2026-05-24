# Product: k3os-bin

`k3os-bin` produces the single statically-linked `k3os` binary used by the [K3OS](https://github.com/petercb/k3os) Linux distribution. It is a multi-personality binary that serves as:

- **Init process** (`/init`, `/sbin/init`) — relocates the root filesystem, mounts squashfs, and hands off to the shell init system
- **Run control** (`k3os rc`) — mounts essential filesystems, starts hotplug, sets hostname/clock/DNS at early boot
- **Config applier** (`k3os config`) — reads and merges `config.yaml` from multiple sources, applies configuration in phases (initrd, boot, install)
- **Upgrade orchestrator** (`k3os upgrade`) — copies new kernel/rootfs components and manages `current`/`previous` version symlinks
- **Interactive installer** (`k3os install`) — CLI wizard for disk installation

The binary targets `linux/amd64` and `linux/arm64`. It must be compiled with `CGO_ENABLED=0` and `-extldflags -static`. Most subcommands require root privileges.

**Out of scope**: kernel build, rootfs assembly, final K3OS image build, k3s itself — those live in separate repositories.
