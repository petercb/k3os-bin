# Specification: Replace blkid/lsblk Shell-outs with u-root pkg/mount/block

## Overview

Replace external `blkid` and `lsblk` command invocations with pure Go block
device discovery using `github.com/u-root/u-root/pkg/mount/block`.

## Motivation

The boot sequence currently shells out to `blkid` and `lsblk` in several
places to discover block devices by label or enumerate disks. These external
tool calls:

- Require `blkid` and `lsblk` to be present in the rootfs
- Add subprocess overhead during early boot
- Cannot be unit-tested without mocking the command runner
- Are fragile (output format may vary between versions)

The u-root `pkg/mount/block` package provides pure Go block device discovery
by reading `/sys/class/block/` and parsing partition tables directly.

## Current Shell-outs to Replace

| Location | Command | Purpose |
|----------|---------|---------|
| `internal/boot/modes/disk.go` | `blkid -L K3OS_STATE` | Find state partition device path |
| `internal/boot/modes/live.go` | `blkid -L K3OS` | Find ISO/USB device |
| `internal/boot/modes/live.go` | `lsblk -o NAME,TYPE -n` | Enumerate disk devices for USB probe |
| `internal/boot/finalize/grow.go` | `blkid -L K3OS_STATE` | Find state partition for resize |
| `internal/mode/mode.go` (detector) | `blockProbe("K3OS_STATE")` | Mode detection probe |

## Target Package

- **Import**: `github.com/u-root/u-root/pkg/mount/block`
- **License**: BSD-3-Clause (compatible with Apache-2.0)
- **Latest version**: v0.16.0 (Feb 2026)

## Key u-root APIs

```go
// Find device by filesystem label
block.Device(name string) (*BlockDev, error)

// Get all block devices
block.GetBlockDevices() (BlockDevices, error)

// BlockDev has Name, FSType, FsUUID fields
```

## Design

### New interface in `internal/iface`

```go
// BlockProber abstracts block device discovery.
type BlockProber interface {
    // FindByLabel returns the device path for a filesystem label.
    FindByLabel(label string) (string, error)
    // ListDisks returns all block devices of type "disk".
    ListDisks() ([]string, error)
}
```

### Implementation using u-root

```go
type URootBlockProber struct{}

func (p *URootBlockProber) FindByLabel(label string) (string, error) {
    devs, err := block.GetBlockDevices()
    if err != nil {
        return "", err
    }
    for _, dev := range devs {
        if dev.FsLabel == label {
            return "/dev/" + dev.Name, nil
        }
    }
    return "", fmt.Errorf("no device with label %q", label)
}
```

### Migration path

1. Add `BlockProber` interface to `internal/iface`
2. Create `internal/iface/osimpl` implementation using u-root
3. Add `BlockProber` field to `modes.Deps` and `finalize.Finalizer`
4. Replace `Cmd.RunOutput("blkid", ...)` and `Cmd.RunOutput("lsblk", ...)`
   calls with `BlockProber` method calls
5. Update tests to use mock `BlockProber`
6. Remove `blockProbe` helper from `main.go`

## Dependencies Added

- `github.com/u-root/u-root/pkg/mount/block`
- Transitive: `github.com/rekby/gpt` (GPT parsing)

## Acceptance Criteria

- [ ] No `blkid` or `lsblk` shell-outs remain in boot packages
- [ ] All block device lookups go through the `BlockProber` interface
- [ ] Unit tests pass with mocked `BlockProber`
- [ ] Integration works on ext4 partitions with K3OS_STATE/K3OS labels
- [ ] `golangci-lint run ./...` passes
- [ ] Binary size increase is documented and acceptable
