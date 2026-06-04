#!/usr/bin/env bash
# Creates a QEMU disk image with an MBR (DOS) partition table containing a
# K3OS_STATE-labeled ext4 partition for integration testing the MBR partition
# grow code path. The partition is intentionally small (~30MB) within a larger
# disk (128MB) so the growpart logic can expand it on first boot.
#
# This mirrors the Raspberry Pi 4 image layout (MBR + ext4 K3OS_STATE).
#
# Expects:
#   ./k3os (built binary in project root)
#
# Produces:
#   integration/qemu/.cache/test-state-disk-mbr.qcow2

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
CACHE_DIR="${SCRIPT_DIR}/.cache"

BINARY="${PROJECT_ROOT}/k3os"
RAW_IMAGE="${CACHE_DIR}/test-state-disk-mbr.raw"
OUTPUT="${CACHE_DIR}/test-state-disk-mbr.qcow2"
DISK_SIZE_MB=128

# Partition layout (in 512-byte sectors):
#   Sector 0:      MBR
#   Sector 2048:   Partition 1 start (1MB alignment)
#   Sector 63488:  Partition 1 end (~30MB = 61440 sectors)
#   Sector 63488+: Free space for grow test (~97MB)
PART_START=2048
PART_SIZE=61440  # ~30MB in 512-byte sectors

# Validate inputs
if [[ ! -f "${BINARY}" ]]; then
    echo "ERROR: k3os binary not found at ${BINARY}. Run 'make build' first."
    exit 1
fi

mkdir -p "${CACHE_DIR}"

echo "==> Building K3OS_STATE MBR disk image..."

# Create a raw disk image
echo "    [create] Raw image: ${RAW_IMAGE} (${DISK_SIZE_MB}MB)"
dd if=/dev/zero of="${RAW_IMAGE}" bs=1M count="${DISK_SIZE_MB}" status=none

# Create MBR partition table with one Linux partition using sfdisk
echo "    [partition] Creating MBR with partition 1 (start=${PART_START}, size=${PART_SIZE} sectors)..."
echo "${PART_START} ${PART_SIZE} 83" | sfdisk --label dos --no-reread "${RAW_IMAGE}" >/dev/null 2>&1

# Format partition 1 as ext4 with K3OS_STATE label.
# We need to use a loop device with the correct offset.
PART_OFFSET=$((PART_START * 512))
PART_BYTES=$((PART_SIZE * 512))

LOOP_DEV=$(sudo losetup --find --show --offset "${PART_OFFSET}" --sizelimit "${PART_BYTES}" "${RAW_IMAGE}")
echo "    [format] Formatting partition as ext4 (label=K3OS_STATE) on ${LOOP_DEV}..."
sudo mkfs.ext4 -q -L K3OS_STATE "${LOOP_DEV}"

# Mount and populate
MOUNT_DIR=$(mktemp -d)
trap 'sudo umount "${MOUNT_DIR}" 2>/dev/null || true; sudo losetup -d "${LOOP_DEV}" 2>/dev/null || true; rm -rf "${MOUNT_DIR}"; rm -f "${RAW_IMAGE}"' EXIT

echo "    [mount] Mounting partition at ${MOUNT_DIR}..."
sudo mount "${LOOP_DEV}" "${MOUNT_DIR}"

# Populate the K3OS_STATE filesystem (same as build-disk-image.sh)
echo "    [populate] Installing filesystem contents..."

# k3os binary
sudo mkdir -p "${MOUNT_DIR}/k3os/system/k3os/current"
sudo cp "${BINARY}" "${MOUNT_DIR}/k3os/system/k3os/current/k3os"
sudo chmod 755 "${MOUNT_DIR}/k3os/system/k3os/current/k3os"

# /sbin/init symlink
sudo mkdir -p "${MOUNT_DIR}/sbin"
sudo ln -sf ../k3os/system/k3os/current/k3os "${MOUNT_DIR}/sbin/init"

# /.base sentinel (post-chroot detection)
sudo mkdir -p "${MOUNT_DIR}/.base"

# /etc and /usr/etc
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

# /etc/ssh and /usr/etc/ssh (for local mode SSH persistence)
sudo mkdir -p "${MOUNT_DIR}/usr/etc/ssh"
sudo mkdir -p "${MOUNT_DIR}/etc/ssh"

# Core directories
sudo mkdir -p "${MOUNT_DIR}"/{proc,sys,dev,run,tmp,var,lib,bin}
sudo mkdir -p "${MOUNT_DIR}/run/k3os"

# /dev/console placeholder
sudo touch "${MOUNT_DIR}/dev/console"

# Minimal /bin/sh stub
cat << 'SHELL' | sudo tee "${MOUNT_DIR}/bin/sh" > /dev/null
#!/sbin/init
exit 1
SHELL
sudo chmod 755 "${MOUNT_DIR}/bin/sh"

# GROWPART MARKER — This is the key difference from build-disk-image.sh.
# The marker triggers partition expansion on first boot.
# Format: "<device> <partition_number>"
# Using /dev/xxx as a dummy device forces the BlockProber fallback path
# (same as the real RPi4 image which uses "/dev/xxx 99").
sudo mkdir -p "${MOUNT_DIR}/k3os/system"
echo "/dev/xxx 1" | sudo tee "${MOUNT_DIR}/k3os/system/growpart" > /dev/null

# Unmount and detach
echo "    [unmount] Unmounting partition..."
sudo umount "${MOUNT_DIR}"
sudo losetup -d "${LOOP_DEV}"

# Convert to qcow2
echo "    [convert] Converting to qcow2..."
qemu-img convert -f raw -O qcow2 "${RAW_IMAGE}" "${OUTPUT}"

# Clean up
rm -f "${RAW_IMAGE}"
rm -rf "${MOUNT_DIR}"
trap - EXIT

echo "    [done] MBR disk image: ${OUTPUT} ($(du -h "${OUTPUT}" | cut -f1))"
echo "    [info] Partition 1: ${PART_SIZE} sectors ($(( PART_SIZE * 512 / 1024 / 1024 ))MB)"
echo "    [info] Free space: $(( (DISK_SIZE_MB * 1024 * 1024 / 512 - PART_START - PART_SIZE) * 512 / 1024 / 1024 ))MB available for grow"
echo "==> MBR disk image build complete."
