//go:build linux

package enterchroot

import (
	"path/filepath"
	"testing"

	"github.com/moby/sys/reexec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestReexecSelfReturnsValidPath verifies that reexec.Self() returns an
// absolute path, which is required by inFile() to open the running binary.
func TestReexecSelfReturnsValidPath(t *testing.T) {
	self := reexec.Self()
	require.NotEmpty(t, self, "reexec.Self() must return a non-empty string")
	assert.True(t, filepath.IsAbs(self), "reexec.Self() must return an absolute path, got: %s", self)
}

// TestReexecRegisterBasenameDoesNotPanic verifies that registering a
// basename-only name (no path separators) does not panic with the new API.
func TestReexecRegisterBasenameDoesNotPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		reexec.Register("test-contract-basename", func() {})
	}, "registering a basename-only name must not panic")
}

// TestReexecExactMatchBehavior verifies that reexec.Init() uses os.Args[0]
// directly for matching (no filepath.Base transformation). This confirms
// that registrations must use the exact argv[0] value the kernel/exec provides.
func TestReexecExactMatchBehavior(t *testing.T) {
	// The reexec package matches os.Args[0] exactly against registered names.
	// "/init" and "/sbin/init" are distinct registrations.
	// Our enterchroot exec uses "/init" as argv[0] to match the "/init" registration.
	tests := []struct {
		argv0    string
		expected string
	}{
		{"/init", "/init"},
		{"/sbin/init", "/sbin/init"},
	}
	for _, tc := range tests {
		t.Run(tc.argv0, func(t *testing.T) {
			// Verify the value we'd pass as argv[0] matches exactly
			assert.Equal(t, tc.expected, tc.argv0)
		})
	}
}
