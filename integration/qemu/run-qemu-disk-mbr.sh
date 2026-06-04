#!/usr/bin/env bash
# Boots k3os in QEMU with an MBR-partitioned K3OS_STATE disk and test mode
# enabled. This exercises the MBR partition grow code path:
#   1. Mode detection finds K3OS_STATE label → disk mode
#   2. Disk handler mounts K3OS_STATE, finds growpart marker
#   3. PartitionGrower detects MBR table type, grows partition via rekby/mbr
#   4. e2fsck + resize2fs expand the filesystem
#   5. pivot_root into the expanded disk, exec second phase
#
# Expects: integration/qemu/.cache/k3os-vmlinuz-amd64.img
#          integration/qemu/.cache/test-initramfs.gz
#          integration/qemu/.cache/test-state-disk-mbr.qcow2
# Produces: integration/qemu/.cache/serial-output-disk-mbr.log
#           Exit code 0 if tests passed, 1 if failed, 2 if no results found.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CACHE_DIR="${SCRIPT_DIR}/.cache"

KERNEL="${CACHE_DIR}/k3os-vmlinuz-amd64.img"
INITRD="${CACHE_DIR}/test-initramfs.gz"
DISK="${CACHE_DIR}/test-state-disk-mbr.qcow2"
SERIAL_LOG="${CACHE_DIR}/serial-output-disk-mbr.log"

TIMEOUT="${QEMU_TIMEOUT:-600}"

# Validate inputs
if [[ ! -f "${KERNEL}" ]]; then
    echo "ERROR: Kernel not found at ${KERNEL}. Run 'make qemu-download-kernel' first."
    exit 1
fi

if [[ ! -f "${INITRD}" ]]; then
    echo "ERROR: Test initramfs not found at ${INITRD}. Run 'make qemu-build-initramfs' first."
    exit 1
fi

if [[ ! -f "${DISK}" ]]; then
    echo "ERROR: MBR state disk not found at ${DISK}. Run 'make qemu-disk-image-mbr' first."
    exit 1
fi

# Create a snapshot of the disk so we don't modify the original image.
DISK_SNAPSHOT="${CACHE_DIR}/test-state-disk-mbr-snapshot.qcow2"
qemu-img create -f qcow2 -b "$(basename "${DISK}")" -F qcow2 "${DISK_SNAPSHOT}" >/dev/null 2>&1

# Detect KVM availability
ACCEL_OPTS=""
if [[ -w /dev/kvm ]] 2>/dev/null; then
    echo "==> KVM available, enabling hardware acceleration."
    ACCEL_OPTS="-accel kvm -cpu host"
else
    echo "==> KVM not available, using TCG multi-thread emulation."
    ACCEL_OPTS="-accel tcg,thread=multi -cpu max"
fi

echo "==> Booting k3os in QEMU [DISK MODE - MBR GROW] (timeout: ${TIMEOUT}s)..."
echo "    Kernel:  ${KERNEL}"
echo "    Initrd:  ${INITRD}"
echo "    Disk:    ${DISK} (MBR partitioned)"
echo "    Output:  ${SERIAL_LOG}"
echo ""

# Run QEMU with the MBR state disk attached.
# No k3os.mode= — mode detection probes /dev/disk/by-label for K3OS_STATE.
set +e
timeout --foreground --signal=KILL "${TIMEOUT}" qemu-system-x86_64 \
    ${ACCEL_OPTS} \
    -smp "${SMP:-2}" \
    -m "${MEMORY:-2048}" \
    -rtc base=utc,clock=rt \
    -kernel "${KERNEL}" \
    -initrd "${INITRD}" \
    -append "console=ttyS0 loglevel=4 printk.devkmsg=on k3os.test_mode k3os.test_expected_mode=local k3os.debug" \
    -drive "file=${DISK_SNAPSHOT},format=qcow2,if=virtio,id=state" \
    -nographic \
    -serial "file:${SERIAL_LOG}" \
    -no-reboot
QEMU_EXIT=$?
set -e

# Clean up snapshot
rm -f "${DISK_SNAPSHOT}"

echo ""
echo "==> QEMU exited with code ${QEMU_EXIT}"

# Parse results from serial output
echo "==> Parsing test results..."

if ! grep -qF -- "---TEST_RESULTS_START---" "${SERIAL_LOG}"; then
    echo "ERROR: No test results found in serial output."
    echo "       The disk handler likely failed before the verifier could run."
    echo "       Check ${SERIAL_LOG} for details."
    echo ""
    echo "==> Last 50 lines of serial output:"
    tail -50 "${SERIAL_LOG}" 2>/dev/null || true
    exit 2
fi

# Extract JSON between delimiters
RESULTS=$(sed -n '/---TEST_RESULTS_START---/,/---TEST_RESULTS_END---/{//!p;}' "${SERIAL_LOG}")

if [[ -z "${RESULTS}" ]]; then
    echo "ERROR: Test results delimiters found but no content between them."
    exit 2
fi

echo ""
echo "========================================"
echo "    DISK MODE (MBR GROW) TEST RESULTS"
echo "========================================"
echo ""

# Pretty-print if jq is available, otherwise raw output.
if command -v jq &>/dev/null; then
    echo "${RESULTS}" | jq .
    PASSED=$(echo "${RESULTS}" | jq -r '.passed')
else
    echo "${RESULTS}"
    PASSED=$(echo "${RESULTS}" | grep -o '"passed"[[:space:]]*:[[:space:]]*[a-z]*' | head -1 | grep -o '[a-z]*$')
fi

echo ""
echo "========================================"

# === MBR-specific validations ===
echo ""
echo "==> MBR grow-specific checks..."

# Check 1: MBR partition table was detected
if grep -q 'detected partition table.*dos' "${SERIAL_LOG}" 2>/dev/null; then
    echo "    [PASS] MBR (dos) partition table detected"
else
    echo "    [FAIL] MBR partition table detection not found in serial log"
    echo "           (expected 'detected partition table' with type 'dos')"
    PASSED="false"
fi

# Check 2: Partition grow did NOT fail
if grep -q 'partition grow failed' "${SERIAL_LOG}" 2>/dev/null; then
    echo "    [FAIL] Partition grow failed:"
    grep 'partition grow failed' "${SERIAL_LOG}" || true
    PASSED="false"
else
    echo "    [PASS] No 'partition grow failed' errors"
fi

# Check 3: ext4 filesystem was probed successfully
if grep -q 'probed filesystem.*fstype=ext4' "${SERIAL_LOG}" 2>/dev/null; then
    echo "    [PASS] Filesystem probed as ext4"
else
    echo "    [WARN] ext4 filesystem probe not confirmed in log"
fi

echo ""
echo "========================================"

if [[ "${PASSED}" == "true" ]]; then
    # Verify no critical mode handler errors.
    CRITICAL_ERRORS=$(grep 'level=ERROR' "${SERIAL_LOG}" 2>/dev/null | grep -c 'mode handler failed' || true)
    if [[ "${CRITICAL_ERRORS}" -gt 0 ]]; then
        echo ""
        echo "FAILURE: Mode handler errors found in serial output:"
        grep 'level=ERROR' "${SERIAL_LOG}" | grep 'mode handler' || true
        echo ""
        echo "==> DISK MODE (MBR GROW): TESTS FAILED"
        exit 1
    fi
    # Report non-critical errors as informational.
    OTHER_ERRORS=$(grep -c 'level=ERROR' "${SERIAL_LOG}" 2>/dev/null || true)
    if [[ "${OTHER_ERRORS}" -gt 0 ]]; then
        echo ""
        echo "INFO: ${OTHER_ERRORS} non-critical ERROR entries (expected in minimal test env):"
        grep 'level=ERROR' "${SERIAL_LOG}" 2>/dev/null | grep -m 5 "" || true
    fi
    echo ""
    echo "==> DISK MODE (MBR GROW): ALL TESTS PASSED"
    exit 0
else
    echo "==> DISK MODE (MBR GROW): TESTS FAILED"
    echo ""
    echo "==> Serial output tail (debug):"
    tail -30 "${SERIAL_LOG}" 2>/dev/null || true
    exit 1
fi
