package mode

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/petercb/k3os-bin/internal/iface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockCmdlineParser implements iface.CmdlineParser for testing.
type mockCmdlineParser struct {
	flags    map[string]string
	contains map[string]bool
	raw      string
	consoles []string
}

// Compile-time check.
var _ iface.CmdlineParser = (*mockCmdlineParser)(nil)

func (m *mockCmdlineParser) Flag(name string) (string, bool) {
	v, ok := m.flags[name]
	return v, ok
}

func (m *mockCmdlineParser) Contains(name string) bool {
	return m.contains[name]
}

func (m *mockCmdlineParser) Consoles() []string { return m.consoles }
func (m *mockCmdlineParser) Raw() string        { return m.raw }

// ---------------------------------------------------------------------------
// Detector tests
// ---------------------------------------------------------------------------

func newTestDetector() *Detector {
	return &Detector{
		Cmdline:       &mockCmdlineParser{},
		BlockProber:   func(_ string) (string, error) { return "", errors.New("not found") },
		StatfsChecker: func(_ string) (string, error) { return "tmpfs", nil },
		EnvReader:     func(_ string) string { return "" },
		FileWriter:    func(_ string, _ []byte, _ os.FileMode) error { return nil },
		MkdirAll:      func(_ string, _ os.FileMode) error { return nil },
		SleepFunc:     func(_ time.Duration) {},
		Timeout:       100 * time.Millisecond,
		SleepInterval: 10 * time.Millisecond,
	}
}

func TestDetect_CmdlineMode(t *testing.T) {
	t.Parallel()

	d := newTestDetector()
	d.Cmdline = &mockCmdlineParser{flags: map[string]string{"k3os.mode": "disk"}}

	mode, err := d.Detect(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "disk", mode)
}

func TestDetect_CmdlineRescue(t *testing.T) {
	t.Parallel()

	d := newTestDetector()
	d.Cmdline = &mockCmdlineParser{contains: map[string]bool{"rescue": true}}

	mode, err := d.Detect(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "shell", mode)
}

func TestDetect_RescueOverridesMode(t *testing.T) {
	t.Parallel()

	d := newTestDetector()
	d.Cmdline = &mockCmdlineParser{
		flags:    map[string]string{"k3os.mode": "disk"},
		contains: map[string]bool{"rescue": true},
	}

	mode, err := d.Detect(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "shell", mode)
}

func TestDetect_EnvVarOverridesBlkid(t *testing.T) {
	t.Parallel()

	d := newTestDetector()
	d.BlockProber = func(_ string) (string, error) { return "/dev/sda1", nil }
	d.EnvReader = func(key string) string {
		if key == "K3OS_MODE" {
			return "install"
		}
		return ""
	}

	mode, err := d.Detect(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "install", mode)
}

func TestDetect_BlkidFindsK3OSState(t *testing.T) {
	t.Parallel()

	d := newTestDetector()
	d.BlockProber = func(label string) (string, error) {
		if label == "K3OS_STATE" {
			return "/dev/sda1", nil
		}
		return "", errors.New("not found")
	}

	mode, err := d.Detect(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "disk", mode)
}

func TestDetect_NonTmpfsRoot(t *testing.T) {
	t.Parallel()

	d := newTestDetector()
	d.StatfsChecker = func(_ string) (string, error) { return "ext4", nil }

	mode, err := d.Detect(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "local", mode)
}

func TestDetect_FallbackMode(t *testing.T) {
	t.Parallel()

	d := newTestDetector()
	d.Cmdline = &mockCmdlineParser{flags: map[string]string{"k3os.fallback_mode": "live"}}

	mode, err := d.Detect(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "live", mode)
}

func TestDetect_TimeoutReturnsError(t *testing.T) {
	t.Parallel()

	d := newTestDetector()
	d.Timeout = 50 * time.Millisecond
	d.SleepInterval = 10 * time.Millisecond
	// All probes return nothing; no mode can be determined.

	_, err := d.Detect(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to determine boot mode")
}

func TestDetect_ContextCancellation(t *testing.T) {
	t.Parallel()

	d := newTestDetector()
	d.Timeout = 5 * time.Second
	d.SleepInterval = 10 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := d.Detect(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "context")
}

func TestDetect_InvalidModeReturnsError(t *testing.T) {
	t.Parallel()

	d := newTestDetector()
	d.Cmdline = &mockCmdlineParser{flags: map[string]string{"k3os.mode": "bogus"}}

	_, err := d.Detect(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid mode")
}

func TestDetect_EmptyParserFallsThrough(t *testing.T) {
	t.Parallel()

	// With an empty parser and no other signals, should timeout.
	d := newTestDetector()
	d.Timeout = 50 * time.Millisecond
	d.SleepInterval = 10 * time.Millisecond

	_, err := d.Detect(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to determine boot mode")
}

func TestDetect_WritesModefile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	d := newTestDetector()
	d.Cmdline = &mockCmdlineParser{flags: map[string]string{"k3os.mode": "disk"}}
	d.MkdirAll = os.MkdirAll
	d.FileWriter = os.WriteFile
	d.StateDir = dir

	mode, err := d.Detect(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "disk", mode)

	content, err := os.ReadFile(filepath.Join(dir, "mode"))
	require.NoError(t, err)
	assert.Equal(t, "disk", string(content))
}

func TestDetect_PriorityOrder(t *testing.T) {
	t.Parallel()

	// The shell script processes in order within the loop:
	// 1. blkid K3OS_STATE -> disk
	// 2. K3OS_MODE env var overrides
	// 3. fallback_mode from cmdline
	// 4. non-tmpfs root -> local
	// K3OS_MODE should override blkid result

	d := newTestDetector()
	d.BlockProber = func(_ string) (string, error) { return "/dev/sda1", nil }
	d.EnvReader = func(key string) string {
		if key == "K3OS_MODE" {
			return "live"
		}
		return ""
	}

	mode, err := d.Detect(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "live", mode)
}

func TestDetect_WaitLoopRetries(t *testing.T) {
	t.Parallel()

	callCount := 0
	d := newTestDetector()
	d.Timeout = 200 * time.Millisecond
	d.SleepInterval = 10 * time.Millisecond
	// BlockProber succeeds on third attempt
	d.BlockProber = func(_ string) (string, error) {
		callCount++
		if callCount >= 3 {
			return "/dev/sda1", nil
		}
		return "", errors.New("not found")
	}

	mode, err := d.Detect(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "disk", mode)
	assert.GreaterOrEqual(t, callCount, 3)
}

func TestDetect_CmdlineModeTableDriven(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		flags    map[string]string
		contains map[string]bool
		wantMode string
	}{
		{
			name:     "only rescue",
			contains: map[string]bool{"rescue": true},
			wantMode: "shell",
		},
		{
			name:     "mode=local",
			flags:    map[string]string{"k3os.mode": "local"},
			wantMode: "local",
		},
		{
			name:     "mode=live",
			flags:    map[string]string{"k3os.mode": "live"},
			wantMode: "live",
		},
		{
			name:     "mode=shell",
			flags:    map[string]string{"k3os.mode": "shell"},
			wantMode: "shell",
		},
		{
			name:     "rescue overrides mode",
			flags:    map[string]string{"k3os.mode": "disk"},
			contains: map[string]bool{"rescue": true},
			wantMode: "shell",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			d := newTestDetector()
			d.Cmdline = &mockCmdlineParser{
				flags:    tc.flags,
				contains: tc.contains,
			}

			mode, err := d.Detect(context.Background())
			require.NoError(t, err)
			assert.Equal(t, tc.wantMode, mode)
		})
	}
}

// ---------------------------------------------------------------------------
// ValidModes
// ---------------------------------------------------------------------------

func TestValidModes(t *testing.T) {
	t.Parallel()

	for _, m := range []string{"disk", "local", "live", "install", "shell"} {
		assert.True(t, ValidModes[m], "expected %q to be valid", m)
	}
	assert.False(t, ValidModes["bogus"])
	assert.False(t, ValidModes[""])
}
