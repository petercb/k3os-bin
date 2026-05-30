package mode

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// parseCmdline (pure function tests)
// ---------------------------------------------------------------------------

func TestParseCmdline_Empty(t *testing.T) {
	t.Parallel()
	result := parseCmdline("")
	assert.Empty(t, result.Mode)
	assert.Empty(t, result.FallbackMode)
}

func TestParseCmdline_Rescue(t *testing.T) {
	t.Parallel()
	result := parseCmdline("console=ttyS0 rescue quiet")
	assert.Equal(t, "shell", result.Mode)
}

func TestParseCmdline_K3OSMode(t *testing.T) {
	t.Parallel()
	result := parseCmdline("console=ttyS0 k3os.mode=disk quiet")
	assert.Equal(t, "disk", result.Mode)
}

func TestParseCmdline_FallbackMode(t *testing.T) {
	t.Parallel()
	result := parseCmdline("console=ttyS0 k3os.fallback_mode=live")
	assert.Empty(t, result.Mode)
	assert.Equal(t, "live", result.FallbackMode)
}

func TestParseCmdline_ModeOverridesFallback(t *testing.T) {
	t.Parallel()
	result := parseCmdline("k3os.mode=install k3os.fallback_mode=live")
	assert.Equal(t, "install", result.Mode)
	assert.Equal(t, "live", result.FallbackMode)
}

func TestParseCmdline_RescueOverridesMode(t *testing.T) {
	t.Parallel()
	// rescue appears after k3os.mode; last wins as in the shell for-loop
	result := parseCmdline("k3os.mode=disk rescue")
	assert.Equal(t, "shell", result.Mode)
}

func TestParseCmdline_TableDriven(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		cmdline      string
		wantMode     string
		wantFallback string
	}{
		{
			name:     "only rescue",
			cmdline:  "rescue",
			wantMode: "shell",
		},
		{
			name:     "mode=local",
			cmdline:  "k3os.mode=local",
			wantMode: "local",
		},
		{
			name:     "mode=live with other args",
			cmdline:  "console=tty1 k3os.mode=live loglevel=3",
			wantMode: "live",
		},
		{
			name:         "fallback only",
			cmdline:      "k3os.fallback_mode=install",
			wantFallback: "install",
		},
		{
			name:         "both mode and fallback",
			cmdline:      "k3os.mode=disk k3os.fallback_mode=live",
			wantMode:     "disk",
			wantFallback: "live",
		},
		{
			name:    "empty string",
			cmdline: "",
		},
		{
			name:    "unrelated params only",
			cmdline: "console=ttyS0 quiet splash",
		},
		{
			name:     "mode with equals in value ignored gracefully",
			cmdline:  "k3os.mode=shell",
			wantMode: "shell",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := parseCmdline(tc.cmdline)
			assert.Equal(t, tc.wantMode, result.Mode)
			assert.Equal(t, tc.wantFallback, result.FallbackMode)
		})
	}
}

// ---------------------------------------------------------------------------
// Detector tests
// ---------------------------------------------------------------------------

func newTestDetector() *Detector {
	return &Detector{
		CmdlineReader: func() (string, error) { return "", nil },
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
	d.CmdlineReader = func() (string, error) { return "k3os.mode=disk", nil }

	mode, err := d.Detect(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "disk", mode)
}

func TestDetect_CmdlineRescue(t *testing.T) {
	t.Parallel()

	d := newTestDetector()
	d.CmdlineReader = func() (string, error) { return "rescue", nil }

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
	d.CmdlineReader = func() (string, error) { return "k3os.fallback_mode=live", nil }

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
	d.CmdlineReader = func() (string, error) { return "k3os.mode=bogus", nil }

	_, err := d.Detect(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid mode")
}

func TestDetect_CmdlineReadError(t *testing.T) {
	t.Parallel()

	d := newTestDetector()
	d.CmdlineReader = func() (string, error) { return "", errors.New("read error") }

	_, err := d.Detect(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read error")
}

func TestDetect_WritesModefile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	d := newTestDetector()
	d.CmdlineReader = func() (string, error) { return "k3os.mode=disk", nil }
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

// ---------------------------------------------------------------------------
// Set function
// ---------------------------------------------------------------------------

func TestSet_WritesFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	modePath := filepath.Join(dir, "mode")

	err := Set("disk", dir)
	require.NoError(t, err)

	content, err := os.ReadFile(modePath)
	require.NoError(t, err)
	assert.Equal(t, "disk", string(content))
}

func TestSet_CreatesDirectory(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "sub", "dir")

	err := Set("live", dir)
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(dir, "mode"))
	require.NoError(t, err)
	assert.Equal(t, "live", string(content))
}
