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

# Create /etc/ssh directory (local mode handler expects it for SSH key persistence)
mkdir -p "${WORK_DIR}/etc/ssh"

# Create /dev/console placeholder (QEMU provides the real device)
touch "${WORK_DIR}/dev/console"

# Install kernel modules if the tarball is available (downloaded by
# download-kernel.sh). This allows modaliases() to load drivers like
# virtio_blk and ext4 during early boot, which is essential for
# devpopulate to discover block devices.
MODULES_TAR="${CACHE_DIR}/k3os-modules-amd64.tar.gz"
if [[ -f "${MODULES_TAR}" ]]; then
    echo "    [modules] Extracting kernel modules..."
    # List top-level entries for debugging, then extract.
    TAR_ROOT=$(tar -tzf "${MODULES_TAR}" 2>/dev/null | head -1 | cut -d'/' -f1)
    echo "    [modules] Tarball root: ${TAR_ROOT:-<empty>}"
    tar -xzf "${MODULES_TAR}" -C "${WORK_DIR}" 2>/dev/null || true
    # If tarball extracts to a subdirectory that doesn't match /lib/modules,
    # move it into place.
    if [[ -d "${WORK_DIR}/${TAR_ROOT}" && "${TAR_ROOT}" != "lib" && -d "${WORK_DIR}/${TAR_ROOT}/lib/modules" ]]; then
        cp -a "${WORK_DIR}/${TAR_ROOT}/lib/modules"/* "${WORK_DIR}/lib/modules/" 2>/dev/null || true
        rm -rf "${WORK_DIR:?}/${TAR_ROOT}"
    fi
    MODULE_COUNT=$(find "${WORK_DIR}/lib/modules" -name '*.ko' -o -name '*.ko.*' 2>/dev/null | wc -l)
    echo "    [modules] Installed: ${MODULE_COUNT} module files"
    if [[ "${MODULE_COUNT}" -eq 0 ]]; then
        echo "    [modules] WARNING: No modules found. Tarball contents:"
        tar -tzf "${MODULES_TAR}" 2>/dev/null | head -20
    fi
else
    echo "    [modules] No modules tarball found at ${MODULES_TAR}, skipping."
    echo "              (Run 'make qemu-download-kernel' to fetch kernel assets.)"
fi

# NOTE: busybox and /usr/sbin/mdev are no longer needed in the initramfs.
# The k3os binary now handles /dev population and /dev/disk/by-label symlink
# creation natively via the internal devpopulate package (pure Go replacement
# for "mdev -s"). This eliminates the external busybox dependency.

# Repack the initramfs
echo "    [pack] Creating cpio archive..."
(cd "${WORK_DIR}" && find . -print0 | cpio --null -o -H newc --quiet 2>/dev/null | gzip -9) > "${OUTPUT}"

echo "    [done] Test initramfs: ${OUTPUT} ($(du -h "${OUTPUT}" | cut -f1))"
echo "==> Build complete."
