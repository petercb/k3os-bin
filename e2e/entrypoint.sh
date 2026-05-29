#!/usr/bin/env bash
set -uo pipefail

exit_code=0

echo "=== Running E2E tests as root ==="
go test -tags e2e -v -count=1 ./e2e/...
root_rc=$?
if [ $root_rc -ne 0 ]; then
    echo "FAIL: root test pass exited with code $root_rc"
    exit_code=$root_rc
fi

echo ""
echo "=== Running E2E tests as non-root user ==="
su testuser -c "go test -tags e2e -v -count=1 -run TestConfigRequiresRoot ./e2e/..."
user_rc=$?
if [ $user_rc -ne 0 ]; then
    echo "FAIL: non-root test pass exited with code $user_rc"
    exit_code=$user_rc
fi

echo ""
if [ $exit_code -eq 0 ]; then
    echo "All E2E test passes succeeded."
else
    echo "One or more E2E test passes failed."
fi

exit $exit_code
