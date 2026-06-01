//go:build linux

package namespace

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWrite_String(t *testing.T) {
	t.Parallel()

	w := Write{Path: "/sys/fs/cgroup/memory/memory.use_hierarchy", Content: "1"}
	assert.Equal(t, "write{/sys/fs/cgroup/memory/memory.use_hierarchy}", w.String())
}

func TestWrite_Create(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "testfile")
	w := Write{Path: path, Content: "hello world"}

	err := w.Create()
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "hello world", string(data))
}
