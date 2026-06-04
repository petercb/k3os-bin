#!/usr/bin/env bash
# Builds a test initramfs for QEMU integration testing.
# The initramfs is minimal: it contains the k3os binary as /init,
# a /.base sentinel (so the binary detects it is post-chroot),
# and minimal filesystem scaffolding for the bootstrap phase.
#
# Expects:
#   ./k3os (built binary in project root)
#
# Produces:
#   integration/qemu/.cache/test-initramfs.gz

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
CACHE_DIR="${SCRIPT_DIR}/.cache"

BINARY="${PROJECT_ROOT}/k3os"
OUTPUT="${CACHE_DIR}/test-initramfs.gz"

# Validate inputs
if [[ ! -f "${BINARY}" ]]; then
    echo "ERROR: k3os binary not found at ${BINARY}. Run 'make build' first."
    exit 1
fi

echo "==> Building test initramfs..."

# Create temp directory for initramfs assembly
WORK_DIR=$(mktemp -d)
trap 'rm -rf "${WORK_DIR}"' EXIT

echo "    [work] Using temp directory: ${WORK_DIR}"

# Create the minimal filesystem structure
echo "    [setup] Creating minimal filesystem layout..."

# Core directories
mkdir -p "${WORK_DIR}"/{proc,sys,dev,run,tmp,etc,var,lib,bin,sbin}
mkdir -p "${WORK_DIR}/usr/etc"
mkdir -p "${WORK_DIR}/run/k3os"
mkdir -p "${WORK_DIR}/k3os/system/k3os/current"

# Create the /.base sentinel directory (triggers post-chroot detection)
mkdir -p "${WORK_DIR}/.base"

# Copy the k3os binary to the required locations
echo "    [copy] Installing k3os binary..."
cp "${BINARY}" "${WORK_DIR}/init"
chmod 755 "${WORK_DIR}/init"

cp "${BINARY}" "${WORK_DIR}/sbin/init"
chmod 755 "${WORK_DIR}/sbin/init"

cp "${BINARY}" "${WORK_DIR}/k3os/system/k3os/current/k3os"
chmod 755 "${WORK_DIR}/k3os/system/k3os/current/k3os"

# Create minimal /usr/etc/passwd (source for bootstrap CopyDir)
cat > "${WORK_DIR}/usr/etc/passwd" << 'EOF'
root:x:0:0:root:/root:/bin/bash
rancher:x:1000:1000:rancher:/home/rancher:/bin/bash
EOF

# Create minimal /usr/etc/shadow
cat > "${WORK_DIR}/usr/etc/shadow" << 'EOF'
root:*:19000:0:99999:7:::
rancher:*:19000:0:99999:7:::
EOF

# Create minimal /usr/etc/group
cat > "${WORK_DIR}/usr/etc/group" << 'EOF'
root:x:0:
rancher:x:1000:
EOF

# Create /etc/passwd as well (some checks look here directly)
cp "${WORK_DIR}/usr/etc/passwd" "${WORK_DIR}/etc/passwd"
cp "${WORK_DIR}/usr/etc/shadow" "${WORK_DIR}/etc/shadow"
cp "${WORK_DIR}/usr/etc/group" "${WORK_DIR}/etc/group"

# Create /bin/sh - use busybox if available, otherwise a no-op stub.
# A working shell is needed for /usr/sbin/mdev script execution.
BUSYBOX=""
for candidate in /bin/busybox /usr/bin/busybox; do
    if [[ -x "${candidate}" ]] && file "${candidate}" | grep -q "statically linked"; then
        BUSYBOX="${candidate}"
        break
    fi
done
# Try to find any busybox (even dynamically linked, copy it)
if [[ -z "${BUSYBOX}" ]]; then
    for candidate in /bin/busybox /usr/bin/busybox; do
        if [[ -x "${candidate}" ]]; then
            BUSYBOX="${candidate}"
            break
        fi
    done
fi
# As last resort, download a static busybox
if [[ -z "${BUSYBOX}" ]]; then
    echo "    [download] Fetching static busybox for initramfs..."
    BUSYBOX="${CACHE_DIR}/busybox"
    curl -fsSL -o "${BUSYBOX}" \
        "https://busybox.net/downloads/binaries/1.35.0-x86_64-linux-musl/busybox"
    chmod 755 "${BUSYBOX}"
fi

echo "    [shell] Using busybox: ${BUSYBOX}"
cp "${BUSYBOX}" "${WORK_DIR}/bin/busybox"
chmod 755 "${WORK_DIR}/bin/busybox"
# Cache busybox for other scripts (e.g., build-disk-image.sh)
cp "${BUSYBOX}" "${CACHE_DIR}/busybox" 2>/dev/null || true
ln -sf busybox "${WORK_DIR}/bin/sh"
ln -sf busybox "${WORK_DIR}/bin/dd"
ln -sf busybox "${WORK_DIR}/bin/tr"
ln -sf busybox "${WORK_DIR}/bin/ln"
ln -sf busybox "${WORK_DIR}/bin/mkdir"
ln -sf busybox "${WORK_DIR}/bin/basename"

# Create /usr/sbin/mdev stub that creates /dev/disk/by-label symlinks.
# The k3os rc phase calls "mdev -s" for device hotplug. In a real system
# udev would create /dev/disk/by-label/ symlinks, but in this minimal
# test initramfs we do it manually by reading ext4 superblock labels.
mkdir -p "${WORK_DIR}/usr/sbin"
cat > "${WORK_DIR}/usr/sbin/mdev" << 'MDEV'
#!/bin/sh
# Minimal mdev replacement for integration tests.
# Creates /dev/disk/by-label/ symlinks by reading ext4 volume labels
# from block device superblocks.
mkdir -p /dev/disk/by-label

# Scan all block devices in /sys/class/block
for dev in /sys/class/block/*; do
    name=$(basename "$dev")
    devpath="/dev/${name}"
    [ -b "$devpath" ] || continue

    # Read ext4 superblock label: superblock starts at byte 1024,
    # s_volume_name is at offset 120 within it = byte 1144, 16 bytes long.
    label=$(dd if="$devpath" bs=1 skip=1144 count=16 2>/dev/null | tr -d '\0')
    if [ -n "$label" ]; then
        ln -sf "../../${name}" "/dev/disk/by-label/${label}"
    fi
done
MDEV
chmod 755 "${WORK_DIR}/usr/sbin/mdev"

# Create /dev/console placeholder (QEMU provides the real device)
touch "${WORK_DIR}/dev/console"

# Repack the initramfs
echo "    [pack] Creating cpio archive..."
(cd "${WORK_DIR}" && find . -print0 | cpio --null -o -H newc --quiet 2>/dev/null | gzip -9) > "${OUTPUT}"

echo "    [done] Test initramfs: ${OUTPUT} ($(du -h "${OUTPUT}" | cut -f1))"
echo "==> Build complete."
