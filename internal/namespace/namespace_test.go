//go:build linux

package namespace

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// String() methods
// ---------------------------------------------------------------------------

func TestDir_String(t *testing.T) {
	t.Parallel()

	d := Dir{Name: "/proc", Mode: 0o555}
	assert.Equal(t, "dir{/proc 0555}", d.String())
}

func TestMount_String(t *testing.T) {
	t.Parallel()

	m := Mount{Source: "proc", Target: "/proc", FSType: "proc"}
	assert.Equal(t, "mount{proc on /proc type proc}", m.String())
}

func TestDev_String(t *testing.T) {
	t.Parallel()

	d := Dev{Name: "/dev/console", Mode: 0o600, Major: 5, Minor: 1}
	assert.Equal(t, "dev{/dev/console 5:1}", d.String())
}

func TestSymlink_String(t *testing.T) {
	t.Parallel()

	s := Symlink{Target: "/proc/self/fd", NewPath: "/dev/fd"}
	assert.Equal(t, "symlink{/dev/fd -> /proc/self/fd}", s.String())
}

// ---------------------------------------------------------------------------
// Create() methods
// ---------------------------------------------------------------------------

func TestDir_Create(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	dir := filepath.Join(tmp, "sub", "deep")
	d := Dir{Name: dir, Mode: 0o750}

	err := d.Create()
	require.NoError(t, err)

	info, err := os.Stat(dir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
	assert.Equal(t, os.FileMode(0o750), info.Mode().Perm())
}

func TestSymlink_Create(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	target := filepath.Join(tmp, "target")
	require.NoError(t, os.WriteFile(target, []byte("hello"), 0o644))

	link := filepath.Join(tmp, "link")
	s := Symlink{Target: target, NewPath: link}

	err := s.Create()
	require.NoError(t, err)

	resolved, err := os.Readlink(link)
	require.NoError(t, err)
	assert.Equal(t, target, resolved)
}

func TestSymlink_Create_AlreadyExists(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	target := filepath.Join(tmp, "target")
	require.NoError(t, os.WriteFile(target, []byte("hello"), 0o644))

	link := filepath.Join(tmp, "link")
	require.NoError(t, os.Symlink(target, link))

	s := Symlink{Target: target, NewPath: link}

	err := s.Create()
	require.NoError(t, err)
}

// ---------------------------------------------------------------------------
// Apply()
// ---------------------------------------------------------------------------

type fakeCreator struct {
	name string
	err  error
}

func (f *fakeCreator) Create() error  { return f.err }
func (f *fakeCreator) String() string { return f.name }

func TestApply_Empty(t *testing.T) {
	t.Parallel()

	err := Apply(nil, nil)
	require.NoError(t, err)
}

func TestApply_AllCalled(t *testing.T) {
	t.Parallel()

	creators := []Creator{
		&fakeCreator{name: "ok1", err: nil},
		&fakeCreator{name: "fail1", err: errors.New("boom")},
		&fakeCreator{name: "ok2", err: nil},
		&fakeCreator{name: "fail2", err: errors.New("bang")},
	}

	err := Apply(creators, nil)
	require.NoError(t, err)
}

func TestApply_NilLogger(t *testing.T) {
	t.Parallel()

	creators := []Creator{
		&fakeCreator{name: "will-fail", err: errors.New("oops")},
	}

	require.NotPanics(t, func() {
		_ = Apply(creators, nil)
	})
}
