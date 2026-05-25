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

// TestReexecBasenameMappingForInit verifies that filepath.Base resolves both
// "/init" and "/sbin/init" to "init", confirming a single "init" registration
// covers both boot modes.
func TestReexecBasenameMappingForInit(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"/init", "init"},
		{"/sbin/init", "init"},
	}
	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			assert.Equal(t, tc.expected, filepath.Base(tc.path))
		})
	}
}
