//go:build linux

// Package finalize implements the boot finalization phase of the k3OS init sequence.
// It ports the shell script overlay/libexec/k3os/boot to Go.
package finalize

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/petercb/k3os-bin/internal/iface"
)

// Finalizer holds dependencies needed to execute the boot finalization phase.
type Finalizer struct {
	FS            iface.FileSystem
	Mounter       iface.Mounter
	Cmd           iface.CommandRunner
	Mode          string
	CmdlineReader func() (string, error)
	RandFunc      func() (uint32, error)
	VirtDetector  func() ([]string, error)
	SleepFunc     func(time.Duration)
}

// Run executes the full boot finalization sequence in order, stopping on first error.
func (f *Finalizer) Run() error {
	steps := []struct {
		name string
		fn   func() error
	}{
		{"SetupMounts", f.SetupMounts},
		{"GrowLive", f.GrowLive},
		{"SetupHostname", f.SetupHostname},
		{"SetupHosts", f.SetupHosts},
		{"SetupRoot", f.SetupRoot},
		{"SetupTTYs", f.SetupTTYs},
		{"SetupSudoers", f.SetupSudoers},
		{"SetupServices", f.SetupServices},
		{"SetupConfig", f.SetupConfig},
		{"SetupManifests", f.SetupManifests},
		{"SetupStateDirs", f.SetupStateDirs},
		{"Cleanup", f.Cleanup},
	}

	for _, step := range steps {
		slog.Debug("finalize: running step", "step", step.name)
		if err := step.fn(); err != nil {
			return fmt.Errorf("finalize %s: %w", step.name, err)
		}
	}

	return nil
}
