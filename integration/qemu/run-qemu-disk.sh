#!/usr/bin/env bash
# Boots k3os in QEMU with a K3OS_STATE disk attached and test mode enabled.
# Mode detection finds the labeled partition and enters disk boot mode.
# This exercises the full disk handler path: resolve device, mount, setup,
# pivot_root, and exec into the second phase.
#
# Expects: integration/qemu/.cache/k3os-vmlinuz-amd64.img
#          integration/qemu/.cache/test-initramfs.gz
#          integration/qemu/.cache/test-state-disk.qcow2
# Produces: integration/qemu/.cache/serial-output-disk.log
#           Exit code 0 if tests passed, 1 if failed, 2 if no results found.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CACHE_DIR="${SCRIPT_DIR}/.cache"

KERNEL="${CACHE_DIR}/k3os-vmlinuz-amd64.img"
INITRD="${CACHE_DIR}/test-initramfs.gz"
DISK="${CACHE_DIR}/test-state-disk.qcow2"
SERIAL_LOG="${CACHE_DIR}/serial-output-disk.log"

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
    echo "ERROR: State disk not found at ${DISK}. Run 'make qemu-disk-image' first."
    exit 1
fi

# Create a snapshot of the disk so we don't modify the original image.
# This lets tests run repeatedly without rebuilding the disk.
DISK_SNAPSHOT="${CACHE_DIR}/test-state-disk-snapshot.qcow2"
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

echo "==> Booting k3os in QEMU [DISK MODE] (timeout: ${TIMEOUT}s)..."
echo "    Kernel:  ${KERNEL}"
echo "    Initrd:  ${INITRD}"
echo "    Disk:    ${DISK}"
echo "    Output:  ${SERIAL_LOG}"
echo ""

# Run QEMU with the state disk attached.
# NOTE: No k3os.mode= on cmdline — mode detection probes /dev/disk/by-label
# and finds K3OS_STATE on the attached virtio disk, selecting "disk" mode.
set +e
timeout --foreground --signal=KILL "${TIMEOUT}" qemu-system-x86_64 \
    ${ACCEL_OPTS} \
    -smp "${SMP:-2}" \
    -m "${MEMORY:-2048}" \
    -rtc base=utc,clock=rt \
    -kernel "${KERNEL}" \
    -initrd "${INITRD}" \
    -append "console=ttyS0 loglevel=4 printk.devkmsg=on k3os.test_mode k3os.test_expected_mode=disk k3os.debug" \
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
echo "    DISK MODE TEST RESULTS"
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

if [[ "${PASSED}" == "true" ]]; then
    # Check for ERROR-level log entries that indicate runtime failures
    # not caught by the structured verifier. These are hard failures.
    ERROR_LINES=$(grep -c 'level=ERROR' "${SERIAL_LOG}" 2>/dev/null || true)
    if [[ "${ERROR_LINES}" -gt 0 ]]; then
        echo ""
        echo "FAILURE: ${ERROR_LINES} ERROR-level log entries found in serial output:"
        grep 'level=ERROR' "${SERIAL_LOG}" | head -10
        echo ""
        echo "==> DISK MODE: TESTS FAILED (structured checks passed but runtime errors detected)"
        exit 1
    else
        echo "==> DISK MODE: ALL TESTS PASSED"
    fi
    exit 0
else
    echo "==> DISK MODE: TESTS FAILED"
    echo ""
    echo "==> Serial output tail (debug):"
    tail -30 "${SERIAL_LOG}" 2>/dev/null || true
    exit 1
fi
