#!/usr/bin/env bash
# Creates a QEMU disk image with a K3OS_STATE-labeled ext4 partition for
# integration testing the disk boot mode. The partition is populated with
# the minimal filesystem structure that the disk handler expects to find
# after mounting, and that the second-phase binary needs after pivot_root.
#
# Expects:
#   ./k3os (built binary in project root)
#
# Produces:
#   integration/qemu/.cache/test-state-disk.qcow2

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
CACHE_DIR="${SCRIPT_DIR}/.cache"

BINARY="${PROJECT_ROOT}/k3os"
RAW_IMAGE="${CACHE_DIR}/test-state-disk.raw"
OUTPUT="${CACHE_DIR}/test-state-disk.qcow2"
DISK_SIZE_MB=128

# Validate inputs
if [[ ! -f "${BINARY}" ]]; then
    echo "ERROR: k3os binary not found at ${BINARY}. Run 'make build' first."
    exit 1
fi

mkdir -p "${CACHE_DIR}"

echo "==> Building K3OS_STATE disk image..."

# Create a raw disk image
echo "    [create] Raw image: ${RAW_IMAGE} (${DISK_SIZE_MB}MB)"
dd if=/dev/zero of="${RAW_IMAGE}" bs=1M count="${DISK_SIZE_MB}" status=none

# Create a single ext4 partition with label K3OS_STATE
echo "    [format] Creating ext4 filesystem with label K3OS_STATE..."
mkfs.ext4 -q -L K3OS_STATE "${RAW_IMAGE}"

# Mount and populate
MOUNT_DIR=$(mktemp -d)
trap 'sudo umount "${MOUNT_DIR}" 2>/dev/null || true; rm -rf "${MOUNT_DIR}"; rm -f "${RAW_IMAGE}"' EXIT

echo "    [mount] Mounting image at ${MOUNT_DIR}..."
sudo mount -o loop "${RAW_IMAGE}" "${MOUNT_DIR}"

# Populate the K3OS_STATE filesystem with what the disk handler creates/expects
# and what the second-phase binary needs after pivot_root + exec.
echo "    [populate] Installing filesystem contents..."

# The disk handler's SetupK3OS copies the binary from /.base into these paths.
# We pre-populate them so SetupK3OS is a no-op (the source /.base won't have
# the binary in this test since we only mount the initramfs there).
sudo mkdir -p "${MOUNT_DIR}/k3os/system/k3os/current"
sudo cp "${BINARY}" "${MOUNT_DIR}/k3os/system/k3os/current/k3os"
sudo chmod 755 "${MOUNT_DIR}/k3os/system/k3os/current/k3os"

# SetupInit creates /sbin/init -> ../k3os/system/k3os/current/k3os
sudo mkdir -p "${MOUNT_DIR}/sbin"
sudo ln -sf ../k3os/system/k3os/current/k3os "${MOUNT_DIR}/sbin/init"

# After pivot_root the old root is at /.root. The second-phase binary
# checks for /.base as the post-chroot sentinel. Since we pivot into
# K3OS_STATE, we create /.base here so the second run detects post-chroot.
sudo mkdir -p "${MOUNT_DIR}/.base"

# Minimal /etc for the second-phase bootstrap/finalization
sudo mkdir -p "${MOUNT_DIR}/etc"
sudo mkdir -p "${MOUNT_DIR}/usr/etc"

cat << 'EOF' | sudo tee "${MOUNT_DIR}/etc/passwd" > /dev/null
root:x:0:0:root:/root:/bin/bash
rancher:x:1000:1000:rancher:/home/rancher:/bin/bash
EOF

cat << 'EOF' | sudo tee "${MOUNT_DIR}/etc/shadow" > /dev/null
root:*:19000:0:99999:7:::
rancher:*:19000:0:99999:7:::
EOF

cat << 'EOF' | sudo tee "${MOUNT_DIR}/etc/group" > /dev/null
root:x:0:
rancher:x:1000:
EOF

sudo cp "${MOUNT_DIR}/etc/passwd" "${MOUNT_DIR}/usr/etc/passwd"
sudo cp "${MOUNT_DIR}/etc/shadow" "${MOUNT_DIR}/usr/etc/shadow"
sudo cp "${MOUNT_DIR}/etc/group" "${MOUNT_DIR}/usr/etc/group"

# Core directories the second-phase binary expects
sudo mkdir -p "${MOUNT_DIR}"/{proc,sys,dev,run,tmp,var,lib,bin}
sudo mkdir -p "${MOUNT_DIR}/run/k3os"

# Create /dev/console placeholder
sudo touch "${MOUNT_DIR}/dev/console"

# Minimal /bin/sh fallback - use busybox from the host initramfs build
# (the disk image builder runs after build-initramfs, so we can reference
# the cached busybox). If not available, use a no-op stub.
BUSYBOX_SRC="${CACHE_DIR}/busybox"
if [[ ! -f "${BUSYBOX_SRC}" ]]; then
    # Try system busybox
    for candidate in /bin/busybox /usr/bin/busybox; do
        if [[ -x "${candidate}" ]]; then
            BUSYBOX_SRC="${candidate}"
            break
        fi
    done
fi
if [[ -f "${BUSYBOX_SRC}" ]]; then
    sudo cp "${BUSYBOX_SRC}" "${MOUNT_DIR}/bin/busybox"
    sudo chmod 755 "${MOUNT_DIR}/bin/busybox"
    sudo ln -sf busybox "${MOUNT_DIR}/bin/sh"
else
    cat << 'SHELL' | sudo tee "${MOUNT_DIR}/bin/sh" > /dev/null
#!/sbin/init
exit 1
SHELL
    sudo chmod 755 "${MOUNT_DIR}/bin/sh"
fi

# Unmount
echo "    [unmount] Unmounting image..."
sudo umount "${MOUNT_DIR}"

# Convert to qcow2 for efficiency
echo "    [convert] Converting to qcow2..."
qemu-img convert -f raw -O qcow2 "${RAW_IMAGE}" "${OUTPUT}"

# Clean up raw image (trap will also fire but that's fine)
rm -f "${RAW_IMAGE}"
rm -rf "${MOUNT_DIR}"

# Disable the trap since we cleaned up manually
trap - EXIT

echo "    [done] Disk image: ${OUTPUT} ($(du -h "${OUTPUT}" | cut -f1))"
echo "==> Disk image build complete."
