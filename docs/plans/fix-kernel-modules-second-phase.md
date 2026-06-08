# Plan: Fix Kernel Modules Not Loading in Second Phase

## Status: Investigating

## Problem Statement

After the full disk-mode boot sequence (pivot_root → exec → enterchroot →
second-phase init), `/lib/modules` and `/lib/firmware` are empty. The
`SetupKernel()` function in the second-phase bootstrap doesn't mount the
squashfs because `Stat(kernelPath)` returns file-not-found.

The kernel squashfs IS present on the K3OS_STATE disk at:
`/k3os/system/kernel/7.0.0-14-k3os/kernel.squashfs`

## Root Cause Analysis

The disk boot sequence involves multiple root changes:

1. **Initramfs root** → first-phase init, bootstrap, disk handler
2. **pivot_root to K3OS_STATE** → disk handler calls PivotAndExec
3. **exec /sbin/init** → new process starts, runs `enterchroot`
4. **enterchroot**: mounts squashfs data overlay from `/dev/loop1` to
   `k3os/data/usr`, then pivots AGAIN to `/.base`

After step 4, the root filesystem is the **squashfs overlay under /.base**,
NOT the raw K3OS_STATE disk. The path `/k3os/system/kernel/<ver>/kernel.squashfs`
is relative to the K3OS_STATE disk which is now at `/.base` or similar —
NOT at the current root.

### Evidence from serial log

```
level=DEBUG msg="running enter-root" args=[]
level=DEBUG msg="using root" root=/proc/self/exe device=/dev/loop1
level=DEBUG msg="mounting squashfs" device=/dev/loop1 dst=k3os/data/usr
level=DEBUG msg="pivoting to ..base"
level=INFO  msg="init: running bootstrap"
```

The enterchroot pivot makes the root filesystem layout different from what
`system.RootPath()` (= `/k3os/system/...`) expects.

## Proposed Investigation

1. **Add debug logging** to `SetupKernel()` to print the exact path being
   checked and what `Stat()` returns.

2. **Trace the root filesystem** after enterchroot's pivot. What is `/`?
   What is at `/.base`? Is the K3OS_STATE disk still accessible?

3. **Check if `/.base/k3os/system/kernel/<ver>/kernel.squashfs`** is the
   correct path after enterchroot's pivot.

## Proposed Fix Options

### Option A: Check multiple paths

Try both `/k3os/system/kernel/...` and `/.base/k3os/system/kernel/...`:

```go
func (b *Bootstrapper) SetupKernel() error {
    paths := []string{
        system.RootPath("kernel", b.KernelVersion, "kernel.squashfs"),
        filepath.Join("/.base", system.RootPath("kernel", b.KernelVersion, "kernel.squashfs")),
    }
    var kernelPath string
    for _, p := range paths {
        if _, err := b.FS.Stat(p); err == nil {
            kernelPath = p
            break
        }
    }
    if kernelPath == "" {
        return nil // squashfs not found anywhere
    }
    ...
}
```

### Option B: Mount kernel squashfs BEFORE enterchroot pivot

Move the kernel squashfs mount to the disk handler (before pivot_root),
where the K3OS_STATE disk is definitely mounted and accessible.

### Option C: Pass the squashfs path through the exec

Have the disk handler write the resolved kernel squashfs path to a well-known
location (e.g., `/run/k3os/kernel-squashfs-path`) before exec, and have
SetupKernel() read it from there.

### Option D: Bind-mount /k3os into the chroot

During enterchroot, bind-mount `/k3os` from the underlying disk so it's
accessible at the same path after the pivot.

## Recommended Approach

Start with **debug logging** (print the exact path and stat result) to confirm
the hypothesis. Then implement **Option A** (try multiple paths) as the
simplest fix that doesn't require changes to the enterchroot flow.

## Files to Investigate

| File | Purpose |
|------|---------|
| `internal/boot/bootstrap/bootstrap.go` | SetupKernel — add debug logging |
| `internal/enterchroot/enter.go` | Understand what pivot changes the root to |
| `main.go` | Understand how enterchroot is invoked |
