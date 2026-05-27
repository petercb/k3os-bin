package mode

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/petercb/k3os-bin/internal/system"
)

// writeModeFile creates the directory structure expected by Get() under root
// and writes content to the mode file.
func writeModeFile(t *testing.T, root, content string) {
	t.Helper()

	modePath := filepath.Join(root, system.StatePath("mode"))
	require.NoError(t, os.MkdirAll(filepath.Dir(modePath), 0o755))

	require.NoError(t, os.WriteFile(modePath, []byte(content), 0o644))
}

func TestGet_LiveMode(t *testing.T) {
	root := t.TempDir()
	writeModeFile(t, root, "live")

	got, err := Get(root)

	require.NoError(t, err)
	assert.Equal(t, "live", got)
}

func TestGet_LocalMode(t *testing.T) {
	root := t.TempDir()
	writeModeFile(t, root, "local")

	got, err := Get(root)

	require.NoError(t, err)
	assert.Equal(t, "local", got)
}

func TestGet_TrimsWhitespace(t *testing.T) {
	root := t.TempDir()
	writeModeFile(t, root, "  live  \n")

	got, err := Get(root)

	require.NoError(t, err)
	assert.Equal(t, "live", got)
}

func TestGet_MissingFile_ReturnsEmpty(t *testing.T) {
	root := t.TempDir()
	// No mode file written — directory exists but file does not.

	got, err := Get(root)

	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestGet_MultiplePrefix_JoinsCorrectly(t *testing.T) {
	base := t.TempDir()
	sub := "subdir"
	root := filepath.Join(base, sub)

	writeModeFile(t, root, "live")

	got, err := Get(base, sub)

	require.NoError(t, err)
	assert.Equal(t, "live", got)
}

func TestGet_EmptyPrefix_UsesAbsolutePath(t *testing.T) {
	// With no prefix, Get() reads from the real system state path.
	// On a non-k3os host the file won't exist, so we expect "" and no error.
	got, err := Get()

	require.NoError(t, err)
	assert.Empty(t, got, "expected empty mode on a non-k3os host")
}

func TestGet_PathIsDirectory_ReturnsError(t *testing.T) {
	root := t.TempDir()
	modePath := filepath.Join(root, system.StatePath("mode"))

	// Create the mode path as a directory instead of a file.
	// os.ReadFile on a directory returns an error that is not IsNotExist.
	require.NoError(t, os.MkdirAll(modePath, 0o755))

	got, err := Get(root)

	require.Error(t, err)
	assert.Empty(t, got)
}
