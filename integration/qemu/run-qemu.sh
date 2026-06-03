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

TIMEOUT="${QEMU_TIMEOUT:-540}"

# Validate inputs
if [[ ! -f "${KERNEL}" ]]; then
    echo "ERROR: Kernel not found at ${KERNEL}. Run 'make qemu-download-kernel' first."
    exit 1
fi

if [[ ! -f "${INITRD}" ]]; then
    echo "ERROR: Test initramfs not found at ${INITRD}. Run 'make qemu-build-initramfs' first."
    exit 1
fi

# Try to enable KVM if not already available
if [[ ! -w /dev/kvm ]]; then
    echo "==> /dev/kvm not writable, attempting to load KVM modules..."
    sudo modprobe kvm 2>/dev/null || true
    # Try both Intel and AMD; one will fail silently
    sudo modprobe kvm-intel 2>/dev/null || sudo modprobe kvm-amd 2>/dev/null || true
    # Give udev a moment to create the device node
    sleep 1
fi

# Determine acceleration strategy
ACCEL_OPTS=""
if [[ -w /dev/kvm ]]; then
    echo "==> KVM available, enabling hardware acceleration."
    ACCEL_OPTS="-enable-kvm -cpu host"
else
    echo "==> WARNING: KVM not available. TCG emulation will be used (very slow)."
    echo "==> Consider running on a host with nested virtualization support."
    ACCEL_OPTS="-accel tcg,thread=multi -cpu max -smp 2"
fi

# Without KVM, full kernel boot under TCG emulation is impractically slow
# (>10 minutes on typical CI VMs). Skip the test unless explicitly required.
if [[ -z "${ACCEL_OPTS##*tcg*}" ]]; then
    if [[ "${REQUIRE_KVM:-false}" == "true" ]]; then
        echo "ERROR: KVM required (REQUIRE_KVM=true) but not available."
        exit 1
    fi
    echo ""
    echo "========================================"
    echo "  QEMU INTEGRATION TEST SKIPPED"
    echo "========================================"
    echo ""
    echo "  KVM hardware acceleration is not available."
    echo "  Full kernel boot under TCG emulation is too slow for CI."
    echo ""
    echo "  To run this test, use a host with KVM support:"
    echo "    - Linux with nested virtualization enabled"
    echo "    - Self-hosted CI runner with /dev/kvm access"
    echo "    - Local development machine"
    echo ""
    echo "  Set REQUIRE_KVM=true to make this a hard failure."
    echo "========================================"
    exit 0
fi

echo "==> Booting k3os in QEMU (timeout: ${TIMEOUT}s)..."
echo "    Kernel:  ${KERNEL}"
echo "    Initrd:  ${INITRD}"
echo "    Output:  ${SERIAL_LOG}"
echo ""

# Run QEMU and capture serial output
set +e
timeout "${TIMEOUT}" qemu-system-x86_64 \
    ${ACCEL_OPTS} \
    -kernel "${KERNEL}" \
    -initrd "${INITRD}" \
    -append "console=ttyS0 k3os.mode=live k3os.test_mode k3os.debug" \
    -nographic \
    -serial stdio \
    -m 512 \
    -no-reboot \
    2>&1 | tee "${SERIAL_LOG}"
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
    echo "==> ALL TESTS PASSED"
    exit 0
else
    echo "==> TESTS FAILED"
    exit 1
fi
