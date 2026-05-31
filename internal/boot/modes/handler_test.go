//go:build linux

package modes

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestDeps() *Deps {
	return &Deps{
		FS:            &MockFileSystem{},
		Cmd:           &MockCommandRunner{},
		Mounter:       &MockMounter{},
		Proc:          &MockProcessExecutor{},
		KernelVersion: "5.15.0",
		VersionID:     "v0.21.5-k3s2r1",
		SleepFunc:     func(time.Duration) {},
	}
}

func TestRegistry_Get_KnownModes(t *testing.T) {
	t.Parallel()

	deps := newTestDeps()
	reg := NewRegistry(deps)

	modes := []string{"disk", "local", "live", "install", "shell"}
	for _, mode := range modes {
		handler, err := reg.Get(mode)
		require.NoError(t, err, "mode %q should be registered", mode)
		assert.NotNil(t, handler, "handler for mode %q should not be nil", mode)
	}
}

func TestRegistry_Get_UnknownMode(t *testing.T) {
	t.Parallel()

	deps := newTestDeps()
	reg := NewRegistry(deps)

	handler, err := reg.Get("unknown")
	require.Error(t, err)
	assert.Nil(t, handler)
	assert.Contains(t, err.Error(), "unknown mode")
}

func TestRegistry_Get_EmptyMode(t *testing.T) {
	t.Parallel()

	deps := newTestDeps()
	reg := NewRegistry(deps)

	handler, err := reg.Get("")
	require.Error(t, err)
	assert.Nil(t, handler)
}
