//go:build linux

package osimpl_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/petercb/k3os-bin/internal/iface/osimpl"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newFakeSysctlApplier creates a LinuxSysctlApplier backed by a temp directory
// that mirrors the expected /proc/sys sub-path structure.
func newFakeSysctlApplier(t *testing.T, segments []string) (osimpl.LinuxSysctlApplier, string) {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(append([]string{root}, segments[:len(segments)-1]...)...)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	filePath := filepath.Join(root, filepath.Join(segments...))
	require.NoError(t, os.WriteFile(filePath, []byte("0\n"), 0o644))
	return osimpl.LinuxSysctlApplier{Root: root}, filePath
}

func TestLinuxSysctlApplier_Set_WritesToCorrectPath(t *testing.T) {
	applier, filePath := newFakeSysctlApplier(t, []string{"net", "ipv4", "ip_forward"})

	err := applier.Set("net.ipv4.ip_forward", "1\n")
	require.NoError(t, err)

	got, err := os.ReadFile(filePath)
	require.NoError(t, err)
	assert.Equal(t, "1\n", string(got))
}

func TestLinuxSysctlApplier_Set_DotConversion(t *testing.T) {
	applier, filePath := newFakeSysctlApplier(t, []string{"kernel", "hostname"})

	err := applier.Set("kernel.hostname", "testhost\n")
	require.NoError(t, err)

	got, err := os.ReadFile(filePath)
	require.NoError(t, err)
	assert.Equal(t, "testhost\n", string(got))
}

func TestLinuxSysctlApplier_Set_NonExistentPath_ReturnsError(t *testing.T) {
	root := t.TempDir()
	applier := osimpl.LinuxSysctlApplier{Root: root}

	err := applier.Set("nonexistent.fake.key", "1")
	assert.Error(t, err, "expected error for non-existent sysctl path")
}
