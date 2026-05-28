//go:build linux
// +build linux

package kernel

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetKernelVersion(t *testing.T) {
	version, err := GetKernelVersion()
	require.NoError(t, err)
	assert.NotEmpty(t, version)
	assert.Contains(t, version, ".", "kernel version should contain at least one dot: %s", version)
}
