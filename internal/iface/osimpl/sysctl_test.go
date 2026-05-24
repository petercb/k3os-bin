//go:build linux

package osimpl_test

import (
	"os"
	"testing"

	"github.com/petercb/k3os-bin/internal/iface/osimpl"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLinuxSysctlApplier_Set_WritesToCorrectPath(t *testing.T) {
	applier := osimpl.LinuxSysctlApplier{}

	// Read current value to restore after test (avoid side effects)
	current, err := os.ReadFile("/proc/sys/net/ipv4/ip_forward")
	require.NoError(t, err)

	// Write back same value — verifies dot-to-path conversion
	err = applier.Set("net.ipv4.ip_forward", string(current))
	assert.NoError(t, err)
}

func TestLinuxSysctlApplier_Set_DotConversion(t *testing.T) {
	applier := osimpl.LinuxSysctlApplier{}

	// kernel.hostname is a safe sysctl to read/write
	current, err := os.ReadFile("/proc/sys/kernel/hostname")
	require.NoError(t, err)

	// Write back same value — verifies multi-segment dot-to-path conversion
	err = applier.Set("kernel.hostname", string(current))
	assert.NoError(t, err)
}

func TestLinuxSysctlApplier_Set_NonExistentPath_ReturnsError(t *testing.T) {
	applier := osimpl.LinuxSysctlApplier{}

	err := applier.Set("nonexistent.fake.key", "1")
	assert.Error(t, err, "expected error for non-existent sysctl path")
}
