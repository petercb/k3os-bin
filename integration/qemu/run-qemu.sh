#!/usr/bin/env bash
# Boots k3os in QEMU with test mode and parses results.
# Expects: integration/qemu/.cache/k3os-vmlinuz-amd64.img
#          integration/qemu/.cache/test-initramfs.gz
# Produces: integration/qemu/.cache/serial-output.log
#           Exit code 0 if tests passed, 1 if failed, 2 if no results found.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CACHE_DIR="${SCRIPT_DIR}/.cache"

KERNEL="${CACHE_DIR}/k3os-vmlinuz-amd64.img"
INITRD="${CACHE_DIR}/test-initramfs.gz"
SERIAL_LOG="${CACHE_DIR}/serial-output.log"

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

# Detect KVM availability (available on bare-metal, not in Docker executors)
ACCEL_OPTS=""
if [[ -w /dev/kvm ]] 2>/dev/null; then
    echo "==> KVM available, enabling hardware acceleration."
    ACCEL_OPTS="-accel kvm -cpu host"
else
    echo "==> KVM not available, using TCG multi-thread emulation."
    ACCEL_OPTS="-accel tcg,thread=multi -cpu max"
fi

echo "==> Booting k3os in QEMU (timeout: ${TIMEOUT}s)..."
echo "    Kernel:  ${KERNEL}"
echo "    Initrd:  ${INITRD}"
echo "    Output:  ${SERIAL_LOG}"
echo ""

# Run QEMU and capture serial output.
# Uses -serial file: to guarantee output is captured even if QEMU is killed.
# Options aligned with petercb/k3os scripts/run-qemu for proven CI behavior.
set +e
timeout --foreground --signal=KILL "${TIMEOUT}" qemu-system-x86_64 \
    ${ACCEL_OPTS} \
    -smp "${SMP:-2}" \
    -m "${MEMORY:-2048}" \
    -rtc base=utc,clock=rt \
    -kernel "${KERNEL}" \
    -initrd "${INITRD}" \
    -append "console=ttyS0 loglevel=4 printk.devkmsg=on k3os.mode=live k3os.test_mode k3os.debug" \
    -nographic \
    -serial "file:${SERIAL_LOG}" \
    -no-reboot
QEMU_EXIT=$?
set -e

echo ""
echo "==> QEMU exited with code ${QEMU_EXIT}"

# Parse results from serial output
echo "==> Parsing test results..."

if ! grep -qF -- "---TEST_RESULTS_START---" "${SERIAL_LOG}"; then
    echo "ERROR: No test results found in serial output."
    echo "       QEMU may have timed out, crashed, or the init binary did not reach test mode."
    echo "       Check ${SERIAL_LOG} for details."
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
echo "        TEST RESULTS"
echo "========================================"
echo ""

# Pretty-print if jq is available, otherwise raw output.
# When jq is available, also use it for robust pass/fail extraction.
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
    # Even if structured tests passed, check for ERROR-level log entries
    # that indicate runtime failures not caught by the verifier.
    ERROR_LINES=$(grep -c 'level=ERROR' "${SERIAL_LOG}" 2>/dev/null || true)
    if [[ "${ERROR_LINES}" -gt 0 ]]; then
        echo ""
        echo "WARNING: ${ERROR_LINES} ERROR-level log entries found in serial output:"
        grep 'level=ERROR' "${SERIAL_LOG}" | head -10
        echo ""
        echo "==> TESTS PASSED (with warnings — review errors above)"
    else
        echo "==> ALL TESTS PASSED"
    fi
    exit 0
else
    echo "==> TESTS FAILED"
    echo ""
    echo "==> Serial output errors:"
    grep 'level=ERROR' "${SERIAL_LOG}" 2>/dev/null | head -10 || true
    exit 1
fi
