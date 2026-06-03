# QEMU Integration Tests

This directory contains scripts for running k3os integration tests in a QEMU
virtual machine. The tests boot a real kernel with a custom initramfs containing
the freshly-built k3os binary, exercise the init sequence in test mode, and
verify the results via structured JSON output on the serial console.

## Prerequisites

- `qemu-system-x86_64` (QEMU with x86_64 system emulation)
- `cpio` (for packing/unpacking initramfs archives)
- `gzip` (for compression)
- `curl` (for downloading kernel assets from GitHub)
- `jq` (optional, for pretty-printing test results)

On Ubuntu/Debian:

```bash
sudo apt-get install qemu-system-x86 cpio gzip curl jq
```

## Quick Start

From the project root:

```bash
make qemu-integration
```

This single command will:

1. Build the k3os binary (`make build`)
2. Download the k3os kernel assets from GitHub releases
3. Build a test initramfs with the freshly-built binary
4. Boot QEMU and run the integration tests
5. Parse and display results

## Individual Steps

```bash
# Download kernel assets only
make qemu-download-kernel

# Build test initramfs (requires k3os binary + kernel assets)
make qemu-build-initramfs

# Run QEMU tests (requires built initramfs)
make qemu-integration
```

Or run the scripts directly:

```bash
integration/qemu/download-kernel.sh
integration/qemu/build-initramfs.sh
integration/qemu/run-qemu.sh
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `KERNEL_VERSION` | v0.111.0 (Makefile) | k3os-kernel release tag. Defined in the Makefile; override via env var. |
| `QEMU_TIMEOUT` | 120 | Timeout in seconds before killing QEMU |
| `GITHUB_TOKEN` | (unset) | Optional GitHub token for API requests (avoids rate limits) |

## Architecture

```
                    +------------------+
                    | download-kernel  |
                    | (GitHub Release) |
                    +--------+---------+
                             |
                          vmlinuz
                             |
                             v
+----------+      +----------+---------+
| make build| ---> | build-initramfs   |
| (k3os)   |      | (inject binary)   |
+----------+      +----------+---------+
                             |
                    test-initramfs.gz
                             |
                             v
                    +--------+---------+
                    |   run-qemu       |
                    | (boot + capture) |
                    +--------+---------+
                             |
                    serial-output.log
                             |
                             v
                    +--------+---------+
                    |  parse results   |
                    | (JSON from log)  |
                    +------------------+
```

### Test Flow

1. QEMU boots with the k3os kernel and the test initramfs
2. The kernel mounts the initramfs and runs `/init` (the k3os binary)
3. The binary detects `/.base` exists (post-chroot sentinel) and runs `postChroot()`
4. `postChroot()` detects `k3os.test_mode` on the kernel cmdline
5. After bootstrap/finalize phases, the Verifier runs instead of exec /sbin/init
6. The Verifier checks system state and outputs JSON results with delimiters
7. The binary calls `reboot(LINUX_REBOOT_CMD_POWER_OFF)` to shut down QEMU
8. `run-qemu.sh` parses the serial output for test results

## Debugging

If tests fail or QEMU hangs:

1. Check the serial output log:
   ```bash
   cat integration/qemu/.cache/serial-output.log
   ```

2. Increase the timeout:
   ```bash
   QEMU_TIMEOUT=300 make qemu-integration
   ```

3. Run QEMU manually for interactive debugging:
   ```bash
   qemu-system-x86_64 \
     -kernel integration/qemu/.cache/k3os-vmlinuz-amd64.img \
     -initrd integration/qemu/.cache/test-initramfs.gz \
     -append "console=ttyS0 k3os.mode=live k3os.test_mode k3os.debug" \
     -nographic -serial stdio -m 512 -no-reboot
   ```

## File Layout

```
integration/qemu/
  download-kernel.sh   # Downloads kernel assets from GitHub
  build-initramfs.sh   # Builds test initramfs with k3os binary
  run-qemu.sh          # Runs QEMU and parses results
  README.md            # This file
  .cache/              # Downloaded/generated artifacts (git-ignored)
    k3os-vmlinuz-amd64.img
    test-initramfs.gz
    serial-output.log
```
