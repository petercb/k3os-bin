//go:build linux

package modes

import (
	"fmt"
	"log/slog"
	"os"
)

// ShellHandler implements ModeHandler for the "shell" boot mode.
// It runs the live setup first, then execs bash.
type ShellHandler struct {
	deps *Deps
}

// NewShellHandler creates a new ShellHandler.
func NewShellHandler(deps *Deps) *ShellHandler {
	return &ShellHandler{deps: deps}
}

// Execute runs the live setup and then execs bash.
func (h *ShellHandler) Execute() error {
	slog.Info("shell: running live setup before dropping to shell")

	if err := NewLiveSetup(h.deps).Run(); err != nil {
		return fmt.Errorf("live setup: %w", err)
	}

	slog.Info("shell: dropping to bash")
	err := h.deps.Proc.Exec("/bin/bash", []string{"/bin/bash"}, os.Environ())
	if err != nil {
		return fmt.Errorf("exec bash: %w", err)
	}
	return &ErrExecCalled{Path: "/bin/bash", Args: []string{"/bin/bash"}, Env: os.Environ()}
}
