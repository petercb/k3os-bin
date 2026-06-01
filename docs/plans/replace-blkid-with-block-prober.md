# Implementation Plan: Replace blkid/lsblk with BlockProber

## Overview

Replace external `blkid` and `lsblk` command invocations with a pure Go
`BlockProber` interface that reads directly from Linux sysfs/devfs.

## Design Decision: sysfs/devfs over u-root

After examining the u-root `pkg/mount/block` package, it was found that:

- `BlockDev` struct only has `Name`, `FSType`, `FsUUID` - no filesystem label
- `GetBlockDevices()` walks `/sys/class/block/` but cannot filter by fs label
- `FilterPartLabel()` only matches GPT partition labels, not filesystem labels

Since our use case is finding devices by **filesystem label** (e.g., K3OS_STATE),
the simplest approach is:

1. **FindByLabel**: Read symlink at `/dev/disk/by-label/<label>` (managed by
   udev/devtmpfs, always available on Linux)
2. **ListDisks**: Read `/sys/block/` directory entries (only lists whole-disk
   devices, not partitions)

This is simpler, has zero external dependencies, and matches exactly what
`blkid -L` and `lsblk` do internally.

## Interface

```go
// BlockProber abstracts block device discovery.
type BlockProber interface {
    // FindByLabel returns the device path for a filesystem label.
    FindByLabel(label string) (string, error)
    // ListDisks returns device names of all block devices of type "disk".
    ListDisks() ([]string, error)
}
```

## Implementation

```go
// SysfsBlockProber implements BlockProber using Linux sysfs/devfs.
type SysfsBlockProber struct{}

func (SysfsBlockProber) FindByLabel(label string) (string, error) {
    path := filepath.Join("/dev/disk/by-label", label)
    target, err := os.Readlink(path)
    if err != nil {
        return "", fmt.Errorf("no device with label %q: %w", label, err)
    }
    if !filepath.IsAbs(target) {
        target = filepath.Join("/dev/disk/by-label", target)
    }
    return filepath.Clean(target), nil
}

func (SysfsBlockProber) ListDisks() ([]string, error) {
    entries, err := os.ReadDir("/sys/block")
    if err != nil {
        return nil, err
    }
    var disks []string
    for _, e := range entries {
        name := e.Name()
        // Skip virtual devices
        if strings.HasPrefix(name, "loop") ||
            strings.HasPrefix(name, "ram") ||
            strings.HasPrefix(name, "dm-") {
            continue
        }
        disks = append(disks, name)
    }
    return disks, nil
}
```

## Migration Points

| File | Before | After |
|------|--------|-------|
| `modes/disk.go` | `Cmd.RunOutput("blkid", "-L", "K3OS_STATE")` | `BlockProber.FindByLabel("K3OS_STATE")` |
| `modes/live.go` | `Cmd.RunOutput("blkid", "-L", "K3OS")` | `BlockProber.FindByLabel("K3OS")` |
| `modes/live.go` | `Cmd.RunOutput("lsblk", "-o", "NAME,TYPE", "-n")` | `BlockProber.ListDisks()` |
| `finalize/grow.go` | `Cmd.RunOutput("blkid", "-L", "K3OS_STATE")` | `BlockProber.FindByLabel("K3OS_STATE")` |
| `main.go` | `blockProbe(label)` helper | `osimpl.SysfsBlockProber{}.FindByLabel(label)` |

## Testing Strategy

- Add `MockBlockProber` to existing mock_test.go files
- Update test expectations from `Cmd.RunOutput("blkid", ...)` to
  `BlockProber.FindByLabel(...)`
- Unit test the osimpl implementation's path construction logic

## Dependencies

No new external dependencies needed. The implementation uses only the
Go standard library (`os`, `path/filepath`, `strings`).
