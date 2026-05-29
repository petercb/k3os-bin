//go:build e2e

package e2e

import (
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func k3osBin() string {
	if bin := os.Getenv("K3OS_BIN"); bin != "" {
		return bin
	}
	return "k3os"
}

func TestVersion(t *testing.T) {
	t.Parallel()

	cmd := exec.Command(k3osBin(), "--version")
	out, err := cmd.CombinedOutput()

	require.NoError(t, err, "expected exit 0, got: %s", string(out))
	assert.Contains(t, string(out), "version")
}

func TestHelp(t *testing.T) {
	t.Parallel()

	cmd := exec.Command(k3osBin(), "--help")
	out, err := cmd.CombinedOutput()

	require.NoError(t, err, "expected exit 0, got: %s", string(out))

	output := string(out)
	assert.Contains(t, output, "config")
	assert.Contains(t, output, "rc")
	assert.Contains(t, output, "install")
	assert.Contains(t, output, "upgrade")
}

func TestConfigHelp(t *testing.T) {
	t.Parallel()

	cmd := exec.Command(k3osBin(), "config", "--help")
	out, err := cmd.CombinedOutput()

	require.NoError(t, err, "expected exit 0, got: %s", string(out))

	output := string(out)
	assert.Contains(t, output, "--initrd")
	assert.Contains(t, output, "--boot")
	assert.Contains(t, output, "--install")
	assert.Contains(t, output, "--dump")
	assert.Contains(t, output, "--dump-json")
}

func TestUpgradeHelp(t *testing.T) {
	t.Parallel()

	cmd := exec.Command(k3osBin(), "upgrade", "--help")
	out, err := cmd.CombinedOutput()

	require.NoError(t, err, "expected exit 0, got: %s", string(out))
	assert.Contains(t, string(out), "upgrade")
}

func TestConfigDump(t *testing.T) {
	t.Parallel()

	if os.Getuid() != 0 {
		t.Skip("skipping: requires root")
	}

	cmd := exec.Command(k3osBin(), "config", "--dump")
	out, err := cmd.CombinedOutput()

	require.NoError(t, err, "expected exit 0, got: %s", string(out))
	assert.Contains(t, string(out), "e2e-test-host")
}

func TestInvalidCommand(t *testing.T) {
	t.Parallel()

	cmd := exec.Command(k3osBin(), "badcmd")
	out, err := cmd.CombinedOutput()

	require.Error(t, err, "expected non-zero exit, output: %s", string(out))
}

func TestConfigRequiresRoot(t *testing.T) {
	t.Parallel()

	if os.Getuid() == 0 {
		t.Skip("skipping: test requires non-root user")
	}

	cmd := exec.Command(k3osBin(), "config", "--dump")
	out, err := cmd.CombinedOutput()

	require.Error(t, err, "expected non-zero exit")
	assert.Contains(t, string(out), "root")
}
