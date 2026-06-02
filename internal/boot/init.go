//go:build linux

// Package boot implements the top-level init orchestrator for k3OS.
// It wires together the bootstrap, mode detection, mode execution, boot
// finalization, and final exec of /sbin/init, matching the flow of the
// original overlay/init shell script.
package boot

import (
	"context"
	"log/slog"
	"os"

	"github.com/petercb/k3os-bin/internal/iface"
)

// BootstrapRunner runs the bootstrap phase.
type BootstrapRunner interface {
	Run() error
}

// FinalizerRunner runs the boot finalization phase.
type FinalizerRunner interface {
	Run() error
}

// ModeHandler executes the logic for a detected boot mode.
type ModeHandler interface {
	Execute() error
}

// OrphanReaper reaps orphaned child processes when running as PID 1.
type OrphanReaper interface {
	Start(ctx context.Context)
	Wait()
}

// ModeDetectorFunc detects the current boot mode and returns it.
type ModeDetectorFunc func() (string, error)

// ModeRegistryFunc returns the handler for a given mode name.
type ModeRegistryFunc func(mode string) (ModeHandler, error)

// Init orchestrates the full k3OS boot sequence after entering the chroot.
// It mirrors the flow of the original overlay/init shell script:
//  1. Run bootstrap (mounts /proc, /etc, sets up users)
//  2. Check cmdline for k3os.debug and enable debug logging
//  3. Redirect stdin/stdout/stderr to /dev/console
//  4. Detect boot mode
//  5. Look up and execute the mode-specific handler
//  6. Run boot finalization
//  7. Exec /sbin/init (OpenRC)
type Init struct {
	Bootstrap       BootstrapRunner
	Reaper          OrphanReaper
	ModeDetector    ModeDetectorFunc
	ModeRegistry    ModeRegistryFunc
	Finalizer       FinalizerRunner
	ExecFunc        func(path string, args []string, env []string) error
	Cmdline         iface.CmdlineParser
	RescueFunc      func() error
	ConsoleRedirect func() error
	ModeSetterFunc  func(mode string)
}

// Run executes the full init sequence. On any phase failure it drops to the
// rescue shell. This method does not return under normal operation because
// ExecFunc replaces the process with /sbin/init.
func (i *Init) Run() {
	// Install a structured text handler immediately for consistent log
	// formatting throughout the init sequence. The level starts at Info;
	// setupDebug() lowers it to Debug after bootstrap mounts /proc.
	logLevel := &slog.LevelVar{}
	logLevel.Set(slog.LevelInfo)
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: logLevel,
	})))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if i.Reaper != nil {
		i.Reaper.Start(ctx)
		defer i.Reaper.Wait()
	}

	slog.Info("init: running bootstrap")
	if err := i.Bootstrap.Run(); err != nil {
		slog.Error("init: bootstrap failed", "error", err)
		i.rescue()
		return
	}

	// Enable debug logging after bootstrap has mounted /proc, which makes
	// /proc/cmdline available for reading the k3os.debug flag.
	i.setupDebug(logLevel)

	// Redirect stdin/stdout/stderr to /dev/console, matching the shell's
	// exec >/dev/console </dev/console 2>&1 after bootstrap.
	if i.ConsoleRedirect != nil {
		if err := i.ConsoleRedirect(); err != nil {
			slog.Warn("init: console redirect failed", "error", err)
		}
	}

	slog.Info("init: detecting boot mode")
	mode, err := i.ModeDetector()
	if err != nil {
		slog.Error("init: mode detection failed", "error", err)
		i.rescue()
		return
	}
	slog.Info("init: detected mode", "mode", mode)

	// Explicitly propagate detected mode to the finalizer.
	if i.ModeSetterFunc != nil {
		i.ModeSetterFunc(mode)
	}

	handler, err := i.ModeRegistry(mode)
	if err != nil {
		slog.Error("init: mode registry lookup failed", "error", err, "mode", mode)
		i.rescue()
		return
	}

	slog.Info("init: executing mode handler", "mode", mode)
	if err := handler.Execute(); err != nil {
		slog.Error("init: mode handler failed", "error", err, "mode", mode)
		i.rescue()
		return
	}

	slog.Info("init: running boot finalization")
	if err := i.Finalizer.Run(); err != nil {
		slog.Error("init: boot finalization failed", "error", err)
		i.rescue()
		return
	}

	slog.Info("init: exec /sbin/init")
	if err := i.ExecFunc("/sbin/init", os.Args, os.Environ()); err != nil {
		slog.Error("init: exec /sbin/init failed", "error", err)
		i.rescue()
		return
	}
}

// setupDebug checks the kernel cmdline for k3os.debug and enables debug-level
// logging if found. It lowers the shared LevelVar rather than replacing the
// handler, preserving consistent log formatting.
func (i *Init) setupDebug(level *slog.LevelVar) {
	if i.Cmdline != nil && i.Cmdline.Contains("k3os.debug") {
		level.Set(slog.LevelDebug)
		slog.Debug("init: debug mode enabled via k3os.debug cmdline")
	}
}

// rescue logs an error message and calls the rescue function to drop to a
// shell, matching the original shell script's rescue() function.
func (i *Init) rescue() {
	slog.Error("init: something went wrong, run with cmdline k3os.debug for more logging")
	slog.Error("init: dropping to shell")
	if err := i.RescueFunc(); err != nil {
		slog.Error("init: rescue shell failed", "error", err)
	}
}
