# Implementation Plan: Replace virt-what with Pure Go DMI Detection

## Overview

Replace the external `virt-what` shell-out in `main.go` with a pure Go
implementation that reads DMI/SMBIOS data from Linux sysfs to detect
virtualization type.

## Design Decision: DMI/SMBIOS sysfs over CPUID or device-tree

The `virt-what` tool uses multiple detection strategies (DMI, CPUID, cgroups,
filesystem markers). However, the consumer (`services.go`) only acts on three
hypervisor types: KVM/QEMU, Hyper-V, and VMware. All three reliably populate
DMI fields in `/sys/class/dmi/id/`, making sysfs reads sufficient.

Benefits of the DMI-only approach:
- Zero external dependencies (stdlib only)
- No architecture-specific code (CPUID is x86-only)
- No unsafe operations or assembly
- Covers all hypervisors the project actually acts on

## Interface

The detector is not defined as a formal interface in `internal/iface/` because
the existing `VirtDetector` field on `Finalizer` is already typed as
`func() ([]string, error)` - a function type, not an interface. The detector
simply provides a method matching that signature.

```go
type DMIDetector struct {
    BasePath string                          // default: "/sys/class/dmi/id/"
    ReadFile func(string) ([]byte, error)    // default: os.ReadFile
}

func (d *DMIDetector) Detect() ([]string, error)
```

## Detection Logic

Read three DMI files: `sys_vendor`, `product_name`, `board_vendor`.

Priority-ordered matching (first match wins):

| Condition | Return |
|-----------|--------|
| sys_vendor contains "QEMU" | `["kvm"]` |
| sys_vendor contains "Microsoft" AND product_name contains "Virtual Machine" | `["hyperv"]` |
| sys_vendor contains "VMware" | `["vmware"]` |
| sys_vendor or board_vendor contains "innotek" OR (Oracle + VirtualBox) | `["virtualbox"]` |
| None match | `nil` |

File read errors return nil, nil (non-fatal, matching existing behavior).

## Migration Points

| File | Before | After |
|------|--------|-------|
| `main.go` | `exec.Command("virt-what")` in `detectVirt()` | `virt.NewDMIDetector().Detect` |
| `main.go` | `import "os/exec"` | Removed |

## Testing Strategy

- Table-driven tests in `internal/virt/detect_test.go` with injected `ReadFile`
- Covers: all hypervisor types, no-match, empty files, read errors
- 100% statement coverage
- Non-Linux stub in `detect_unsupported.go` for cross-platform compilation

## Dependencies

No new external dependencies. Uses only Go standard library (`os`, `strings`).

## Known Limitations

1. KVM detection only matches sys_vendor "QEMU" - enterprise libvirt setups
   that override SMBIOS vendor strings may not be detected
2. VirtualBox is detected but has no service enablement case in services.go
   (intentional - k3OS does not ship VirtualBox guest additions)
