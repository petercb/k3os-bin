//go:build linux

package rc

import (
	"testing"

	"github.com/petercb/k3os-bin/internal/namespace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRcNamespace_NotEmpty(t *testing.T) {
	t.Parallel()

	assert.NotEmpty(t, rcNamespace)
}

func TestRcNamespace_FirstEntry(t *testing.T) {
	t.Parallel()

	require.NotEmpty(t, rcNamespace)
	first := rcNamespace[0]
	assert.Contains(t, first.String(), "proc")
	assert.Contains(t, first.String(), "/proc")
}

func TestRcNamespace_LastEntry(t *testing.T) {
	t.Parallel()

	require.NotEmpty(t, rcNamespace)
	last := rcNamespace[len(rcNamespace)-1]
	s := last.String()
	assert.Contains(t, s, "/")
	assert.Contains(t, s, "mount")
}

func TestRcNamespace_ContainsCgroupMounts(t *testing.T) {
	t.Parallel()

	found := false
	for _, c := range rcNamespace {
		if _, ok := c.(namespace.CgroupMounts); ok {
			found = true
			break
		}
	}
	assert.True(t, found, "rcNamespace should contain a CgroupMounts entry")
}

func TestRcNamespace_ContainsDevConsole(t *testing.T) {
	t.Parallel()

	found := false
	for _, c := range rcNamespace {
		if d, ok := c.(namespace.Dev); ok {
			if d.Name == "/dev/console" {
				found = true
				break
			}
		}
	}
	assert.True(t, found, "rcNamespace should contain the /dev/console device entry")
}
