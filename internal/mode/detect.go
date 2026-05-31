package mode

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/petercb/k3os-bin/internal/system"
)

// ValidModes defines the set of recognized k3OS boot modes.
var ValidModes = map[string]bool{
	"disk":    true,
	"local":   true,
	"live":    true,
	"install": true,
	"shell":   true,
}

// CmdlineResult holds the parsed kernel command line parameters relevant to
// mode detection.
type CmdlineResult struct {
	Mode         string
	FallbackMode string
}

// parseCmdline is a pure function that parses kernel command line parameters
// for mode-related values.
func parseCmdline(cmdline string) CmdlineResult {
	var result CmdlineResult

	for _, param := range strings.Fields(cmdline) {
		switch {
		case param == "rescue":
			result.Mode = "shell"
		case strings.HasPrefix(param, "k3os.mode="):
			result.Mode = strings.TrimPrefix(param, "k3os.mode=")
		case strings.HasPrefix(param, "k3os.fallback_mode="):
			result.FallbackMode = strings.TrimPrefix(param, "k3os.fallback_mode=")
		}
	}

	return result
}

// Detector holds the injectable dependencies for boot mode detection.
type Detector struct {
	// CmdlineReader reads the kernel command line (typically /proc/cmdline).
	CmdlineReader func() (string, error)

	// BlockProber checks whether a block device with the given label exists.
	// Returns the device path if found, or an error if not.
	BlockProber func(label string) (string, error)

	// StatfsChecker returns the filesystem type name for the given path.
	StatfsChecker func(path string) (string, error)

	// EnvReader reads environment variables.
	EnvReader func(key string) string

	// FileWriter writes data to a file.
	FileWriter func(path string, data []byte, perm os.FileMode) error

	// MkdirAll creates directories.
	MkdirAll func(path string, perm os.FileMode) error

	// SleepFunc is the sleep implementation (injectable for testing).
	SleepFunc func(d time.Duration)

	// Timeout is the total time to wait for mode detection (default 30s).
	Timeout time.Duration

	// SleepInterval is the duration between retry attempts.
	SleepInterval time.Duration

	// StateDir overrides the directory where the mode file is written.
	// If empty, system.StatePath is used.
	StateDir string
}

// Detect determines the k3OS boot mode using the same logic as the original
// shell script. It retries in a loop until a mode is found or the timeout
// expires. The detected mode is written to the state file before returning.
func (d *Detector) Detect(ctx context.Context) (string, error) {
	cmdline, err := d.CmdlineReader()
	if err != nil {
		return "", fmt.Errorf("reading cmdline: %w", err)
	}

	parsed := parseCmdline(cmdline)

	// If the command line specifies a mode directly, use it immediately.
	if parsed.Mode != "" {
		return d.finalize(ctx, parsed.Mode)
	}

	deadline := time.Now().Add(d.Timeout)

	for {
		mode := d.detectOnce(parsed)
		if mode != "" {
			return d.finalize(ctx, mode)
		}

		if time.Now().After(deadline) {
			return "", fmt.Errorf("failed to determine boot mode (did you forget to set k3os.mode?)")
		}

		select {
		case <-ctx.Done():
			return "", fmt.Errorf("mode detection cancelled: %w", ctx.Err())
		default:
		}

		remaining := time.Until(deadline)
		slog.Debug("mode: waiting for mode detection", "remaining", remaining.Round(time.Second))
		d.SleepFunc(d.SleepInterval)
	}
}

// detectOnce runs a single pass through the probe logic, matching the shell
// script's loop body ordering.
func (d *Detector) detectOnce(parsed CmdlineResult) string {
	var mode string

	// Check blkid -L K3OS_STATE
	if dev, err := d.BlockProber("K3OS_STATE"); err == nil && dev != "" {
		mode = "disk"
	}

	// K3OS_MODE env var overrides
	if envMode := d.EnvReader("K3OS_MODE"); envMode != "" {
		mode = envMode
	}

	// Fallback mode from cmdline
	if mode == "" {
		mode = parsed.FallbackMode
	}

	// Non-tmpfs root filesystem means local mode
	if mode == "" {
		if fsType, err := d.StatfsChecker("/"); err == nil && fsType != "tmpfs" {
			mode = "local"
		}
	}

	return mode
}

// finalize validates the mode, writes it to the state file, and returns it.
func (d *Detector) finalize(_ context.Context, mode string) (string, error) {
	if !ValidModes[mode] {
		return "", fmt.Errorf("invalid mode %q: must be one of disk, local, live, install, shell", mode)
	}

	stateDir := d.StateDir
	if stateDir == "" {
		stateDir = system.StatePath()
	}

	if err := d.MkdirAll(stateDir, 0o755); err != nil {
		return "", fmt.Errorf("creating state dir: %w", err)
	}

	modePath := filepath.Join(stateDir, "mode")
	if err := d.FileWriter(modePath, []byte(mode), 0o644); err != nil {
		return "", fmt.Errorf("writing mode file: %w", err)
	}

	slog.Info("mode: detected boot mode", "mode", mode)
	return mode, nil
}

// Set writes the given mode to the state file under the provided prefix path.
// If prefix is empty, it uses the default system state path.
func Set(mode string, prefix ...string) error {
	var dir string
	if len(prefix) > 0 {
		dir = filepath.Join(prefix...)
	} else {
		dir = system.StatePath()
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating state dir: %w", err)
	}

	modePath := filepath.Join(dir, "mode")
	if err := os.WriteFile(modePath, []byte(mode), 0o644); err != nil {
		return fmt.Errorf("writing mode file: %w", err)
	}

	return nil
}
