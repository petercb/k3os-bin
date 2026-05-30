//go:build linux

// Package boot implements the top-level init orchestrator for k3OS.
// It wires together the bootstrap, mode detection, mode execution, boot
// finalization, and final exec of /sbin/init, matching the flow of the
// original overlay/init shell script.
package boot

import (
	"log/slog"
	"os"
	"strings"
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

// ModeDetectorFunc detects the current boot mode and returns it.
type ModeDetectorFunc func() (string, error)

// ModeRegistryFunc returns the handler for a given mode name.
type ModeRegistryFunc func(mode string) (ModeHandler, error)

// Init orchestrates the full k3OS boot sequence after entering the chroot.
// It mirrors the flow of the original overlay/init shell script:
//  1. Check cmdline for k3os.debug and enable debug logging
//  2. Run bootstrap
//  3. Detect boot mode
//  4. Look up and execute the mode-specific handler
//  5. Run boot finalization
//  6. Exec /sbin/init (OpenRC)
type Init struct {
	Bootstrap     BootstrapRunner
	ModeDetector  ModeDetectorFunc
	ModeRegistry  ModeRegistryFunc
	Finalizer     FinalizerRunner
	ExecFunc      func(path string, args []string, env []string) error
	CmdlineReader func() (string, error)
	RescueFunc    func() error
}

// Run executes the full init sequence. On any phase failure it drops to the
// rescue shell. This method does not return under normal operation because
// ExecFunc replaces the process with /sbin/init.
func (i *Init) Run() {
	i.setupDebug()

	slog.Info("init: running bootstrap")
	if err := i.Bootstrap.Run(); err != nil {
		slog.Error("init: bootstrap failed", "error", err)
		i.rescue()
		return
	}

	slog.Info("init: detecting boot mode")
	mode, err := i.ModeDetector()
	if err != nil {
		slog.Error("init: mode detection failed", "error", err)
		i.rescue()
		return
	}
	slog.Info("init: detected mode", "mode", mode)

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

// setupDebug checks /proc/cmdline for k3os.debug and enables debug-level
// logging if found. Errors reading the cmdline are silently ignored.
func (i *Init) setupDebug() {
	cmdline, err := i.CmdlineReader()
	if err != nil {
		slog.Debug("init: could not read cmdline for debug check", "error", err)
		return
	}

	for _, field := range strings.Fields(cmdline) {
		if field == "k3os.debug" {
			slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
				Level: slog.LevelDebug,
			})))
			slog.Debug("init: debug mode enabled via k3os.debug cmdline")
			return
		}
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
