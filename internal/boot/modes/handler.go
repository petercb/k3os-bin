//go:build linux

// Package modes implements the boot mode handlers for k3OS.
// Each mode (disk, local, live, install, shell) has a handler that performs
// the necessary setup for that boot mode.
package modes

import (
	"fmt"
	"time"

	"github.com/petercb/k3os-bin/internal/iface"
)

// ModeHandler is implemented by each boot mode to perform its setup logic.
type ModeHandler interface {
	Execute() error
}

// ProcessExecutor abstracts syscalls that replace the current process or
// pivot the root filesystem. This allows testing without actually calling
// pivot_root or exec.
type ProcessExecutor interface {
	PivotRoot(newRoot, putOld string) error
	Exec(path string, args []string, env []string) error
}

// Deps holds shared dependencies injected into all mode handlers.
type Deps struct {
	FS              iface.FileSystem
	Cmd             iface.CommandRunner
	Mounter         iface.Mounter
	BlockProber     iface.BlockProber
	PartitionGrower iface.PartitionGrower
	Proc            ProcessExecutor
	CopyDir         func(src, dst string) error
	KernelVersion   string
	VersionID       string
	SleepFunc       func(time.Duration)
}

// Registry maps mode names to their handler constructors. Call Get() to
// obtain the handler for a given mode name.
type Registry struct {
	deps     *Deps
	handlers map[string]func(*Deps) ModeHandler
}

// NewRegistry creates a new mode registry with all known mode handlers.
func NewRegistry(deps *Deps) *Registry {
	r := &Registry{
		deps: deps,
		handlers: map[string]func(*Deps) ModeHandler{
			"disk":    func(d *Deps) ModeHandler { return NewDiskHandler(d) },
			"local":   func(d *Deps) ModeHandler { return NewLocalHandler(d) },
			"live":    func(d *Deps) ModeHandler { return NewLiveHandler(d) },
			"install": func(d *Deps) ModeHandler { return NewInstallHandler(d) },
			"shell":   func(d *Deps) ModeHandler { return NewShellHandler(d) },
		},
	}
	return r
}

// Get returns the ModeHandler for the given mode name, or an error if unknown.
func (r *Registry) Get(mode string) (ModeHandler, error) {
	ctor, ok := r.handlers[mode]
	if !ok {
		return nil, fmt.Errorf("unknown mode: %q", mode)
	}
	return ctor(r.deps), nil
}
