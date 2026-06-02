//go:build linux

package namespace

import (
	"errors"
	"testing"

	"github.com/petercb/k3os-bin/internal/mount"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeTrackableCreator is a Creator that also implements Trackable.
type fakeTrackableCreator struct {
	name   string
	err    error
	target string
}

func (f *fakeTrackableCreator) Create() error  { return f.err }
func (f *fakeTrackableCreator) String() string { return f.name }

func (f *fakeTrackableCreator) AsMountPoint() *mount.Point {
	return &mount.Point{Target: f.target}
}

func TestApplyTracked_RecordsMounts(t *testing.T) {
	t.Parallel()

	pool := mount.NewPool(func(_ string, _ int) error { return nil })
	creators := []Creator{
		&fakeTrackableCreator{name: "mount1", target: "/mnt/a"},
		&fakeTrackableCreator{name: "mount2", target: "/mnt/b"},
	}

	err := ApplyTracked(creators, pool, nil)
	require.NoError(t, err)
	assert.Equal(t, 2, pool.Len())

	entries := pool.Entries()
	assert.Equal(t, "/mnt/a", entries[0].Target)
	assert.Equal(t, "/mnt/b", entries[1].Target)
}

func TestApplyTracked_SkipsNonMounts(t *testing.T) {
	t.Parallel()

	pool := mount.NewPool(func(_ string, _ int) error { return nil })
	creators := []Creator{
		&fakeCreator{name: "dir1", err: nil},
		&fakeTrackableCreator{name: "mount1", target: "/mnt/a"},
		&fakeCreator{name: "symlink1", err: nil},
	}

	err := ApplyTracked(creators, pool, nil)
	require.NoError(t, err)
	assert.Equal(t, 1, pool.Len())

	entries := pool.Entries()
	assert.Equal(t, "/mnt/a", entries[0].Target)
}

func TestApplyTracked_FailedMountNotRecorded(t *testing.T) {
	t.Parallel()

	pool := mount.NewPool(func(_ string, _ int) error { return nil })
	creators := []Creator{
		&fakeTrackableCreator{name: "mount-ok", target: "/mnt/ok"},
		&fakeTrackableCreator{name: "mount-fail", target: "/mnt/fail", err: errors.New("mount failed")},
	}

	err := ApplyTracked(creators, pool, nil)
	require.NoError(t, err)
	assert.Equal(t, 1, pool.Len())

	entries := pool.Entries()
	assert.Equal(t, "/mnt/ok", entries[0].Target)
}

func TestApplyTracked_NilPoolBehavesLikeApply(t *testing.T) {
	t.Parallel()

	creators := []Creator{
		&fakeTrackableCreator{name: "mount1", target: "/mnt/a"},
		&fakeCreator{name: "dir1", err: nil},
		&fakeTrackableCreator{name: "mount2", target: "/mnt/b", err: errors.New("fail")},
	}

	// Should not panic with nil pool.
	require.NotPanics(t, func() {
		err := ApplyTracked(creators, nil, nil)
		require.NoError(t, err)
	})
}

func TestApplyTracked_SilentSkipNotRecorded(t *testing.T) {
	t.Parallel()

	pool := mount.NewPool(func(_ string, _ int) error { return nil })
	creators := []Creator{
		&fakeTrackableCreator{name: "mount-ok", target: "/mnt/ok"},
		&fakeTrackableCreator{name: "mount-silent", target: "/mnt/silent", err: ErrSilentSkip},
		&fakeTrackableCreator{name: "mount-ok2", target: "/mnt/ok2"},
	}

	err := ApplyTracked(creators, pool, nil)
	require.NoError(t, err)
	assert.Equal(t, 2, pool.Len())

	entries := pool.Entries()
	assert.Equal(t, "/mnt/ok", entries[0].Target)
	assert.Equal(t, "/mnt/ok2", entries[1].Target)
}
