package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadFlagsFile(t *testing.T) {
	oldFlagsFile := flagsFile
	defer func() { flagsFile = oldFlagsFile }()

	t.Run("missing file returns nil", func(t *testing.T) {
		tempDir := t.TempDir()
		flagsFile = filepath.Join(tempDir, "nonexistent")

		data, err := readFlagsFile()
		require.NoError(t, err)
		assert.Nil(t, data)
	})

	t.Run("empty file returns nil", func(t *testing.T) {
		tempDir := t.TempDir()
		path := filepath.Join(tempDir, "config.flags")
		err := os.WriteFile(path, []byte(""), 0o644)
		require.NoError(t, err)
		flagsFile = path

		data, err := readFlagsFile()
		require.NoError(t, err)
		assert.Nil(t, data)
	})

	t.Run("comments and blank lines are skipped", func(t *testing.T) {
		tempDir := t.TempDir()
		path := filepath.Join(tempDir, "config.flags")
		content := `# This is a comment

# Another comment
k3os.hostname=flaghost
`
		err := os.WriteFile(path, []byte(content), 0o644)
		require.NoError(t, err)
		flagsFile = path

		data, err := readFlagsFile()
		require.NoError(t, err)
		require.NotNil(t, data)

		k3os, ok := data["k3os"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "flaghost", k3os["hostname"])
	})

	t.Run("plain key=value lines", func(t *testing.T) {
		tempDir := t.TempDir()
		path := filepath.Join(tempDir, "config.flags")
		content := `k3os.hostname=myhost
k3os.password=secret
k3os.install.silent=true
`
		err := os.WriteFile(path, []byte(content), 0o644)
		require.NoError(t, err)
		flagsFile = path

		data, err := readFlagsFile()
		require.NoError(t, err)
		require.NotNil(t, data)

		k3os, ok := data["k3os"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "myhost", k3os["hostname"])
		assert.Equal(t, "secret", k3os["password"])

		install, ok := k3os["install"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "true", install["silent"])
	})

	t.Run("go-quoted values are unquoted", func(t *testing.T) {
		tempDir := t.TempDir()
		path := filepath.Join(tempDir, "config.flags")
		content := `k3os.password="my secret password"
k3os.label="hello\tworld"
`
		err := os.WriteFile(path, []byte(content), 0o644)
		require.NoError(t, err)
		flagsFile = path

		data, err := readFlagsFile()
		require.NoError(t, err)
		require.NotNil(t, data)

		k3os, ok := data["k3os"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "my secret password", k3os["password"])
		assert.Equal(t, "hello\tworld", k3os["label"])
	})

	t.Run("repeated keys become list", func(t *testing.T) {
		tempDir := t.TempDir()
		path := filepath.Join(tempDir, "config.flags")
		content := `k3os.dns=8.8.8.8
k3os.dns=1.1.1.1
`
		err := os.WriteFile(path, []byte(content), 0o644)
		require.NoError(t, err)
		flagsFile = path

		data, err := readFlagsFile()
		require.NoError(t, err)
		require.NotNil(t, data)

		k3os, ok := data["k3os"].(map[string]interface{})
		require.True(t, ok)
		dns, ok := k3os["dns"].([]string)
		require.True(t, ok)
		assert.Equal(t, []string{"8.8.8.8", "1.1.1.1"}, dns)
	})

	t.Run("boolean flag without value", func(t *testing.T) {
		tempDir := t.TempDir()
		path := filepath.Join(tempDir, "config.flags")
		content := `k3os.debug
`
		err := os.WriteFile(path, []byte(content), 0o644)
		require.NoError(t, err)
		flagsFile = path

		data, err := readFlagsFile()
		require.NoError(t, err)
		require.NotNil(t, data)

		k3os, ok := data["k3os"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "true", k3os["debug"])
	})

	t.Run("dot notation produces nested maps", func(t *testing.T) {
		tempDir := t.TempDir()
		path := filepath.Join(tempDir, "config.flags")
		content := `k3os.install.device=/dev/sda
k3os.install.silent=true
`
		err := os.WriteFile(path, []byte(content), 0o644)
		require.NoError(t, err)
		flagsFile = path

		data, err := readFlagsFile()
		require.NoError(t, err)
		require.NotNil(t, data)

		k3os, ok := data["k3os"].(map[string]interface{})
		require.True(t, ok)
		install, ok := k3os["install"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "/dev/sda", install["device"])
		assert.Equal(t, "true", install["silent"])
	})
}

func TestFlagsFileOverridesCmdline(t *testing.T) {
	// Test that flags-file values override cmdline values through the merge pipeline.
	oldCmdlineFile := cmdlineFile
	oldFlagsFile := flagsFile

	defer func() {
		cmdlineFile = oldCmdlineFile
		flagsFile = oldFlagsFile
	}()

	tempDir := t.TempDir()

	// Set up cmdline with password and install.silent.
	cmdPath := filepath.Join(tempDir, "cmdline")
	err := os.WriteFile(cmdPath, []byte("k3os.password=from-cmdline k3os.install.silent=true"), 0o644)
	require.NoError(t, err)
	cmdlineFile = cmdPath

	// Set up flags file that overrides password but not install.silent.
	flagsPath := filepath.Join(tempDir, "config.flags")
	err = os.WriteFile(flagsPath, []byte("k3os.password=from-flags\n"), 0o644)
	require.NoError(t, err)
	flagsFile = flagsPath

	// Use readersToObject with just cmdline and flags readers to test merge order.
	cfg, err := readersToObject(readCmdline, readFlagsFile)
	require.NoError(t, err)

	// Flags file overrides cmdline for password.
	assert.Equal(t, "from-flags", cfg.K3OS.Password)
	// Cmdline value is preserved for install.silent (not overridden by flags file).
	assert.True(t, cfg.K3OS.Install.Silent)
}
