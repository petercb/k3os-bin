//go:build linux

package modes

import (
	"fmt"
	"log/slog"
)

// LocalHandler implements ModeHandler for the "local" boot mode.
type LocalHandler struct {
	deps *Deps
}

// NewLocalHandler creates a new LocalHandler with the given dependencies.
func NewLocalHandler(deps *Deps) *LocalHandler {
	return &LocalHandler{deps: deps}
}

// Execute runs the local mode boot sequence: setup SSH and rancher node.
func (h *LocalHandler) Execute() error {
	if err := h.SetupSSH(); err != nil {
		return fmt.Errorf("setup ssh: %w", err)
	}
	if err := h.SetupRancherNode(); err != nil {
		return fmt.Errorf("setup rancher node: %w", err)
	}
	return nil
}

// SetupSSH persists SSH keys to /var/lib/rancher/k3os/ssh and creates a
// symlink from /etc/ssh to the persistent location.
func (h *LocalHandler) SetupSSH() error {
	slog.Debug("local: setting up SSH persistence")

	persistDir := "/var/lib/rancher/k3os/ssh"
	etcSSH := "/etc/ssh"

	if _, err := h.deps.FS.Stat(persistDir); err != nil {
		// Persist directory does not exist, copy /etc/ssh there
		if err := h.deps.FS.MkdirAll("/var/lib/rancher/k3os", 0o755); err != nil {
			return fmt.Errorf("mkdir rancher dir: %w", err)
		}
		if err := h.deps.CopyDir(etcSSH, persistDir); err != nil {
			return fmt.Errorf("copy ssh keys: %w", err)
		}
	}

	// Remove /etc/ssh and symlink to persistent location
	if err := h.deps.FS.RemoveAll(etcSSH); err != nil {
		return fmt.Errorf("remove /etc/ssh: %w", err)
	}
	if err := h.deps.FS.Symlink(persistDir, etcSSH); err != nil {
		return fmt.Errorf("symlink ssh: %w", err)
	}
	return nil
}

// SetupRancherNode creates /etc/rancher and /var/lib/rancher/k3os/node
// directories, then symlinks /etc/rancher/node to the persistent location.
func (h *LocalHandler) SetupRancherNode() error {
	slog.Debug("local: setting up rancher node")

	etcRancher := "/etc/rancher"
	nodeDir := "/var/lib/rancher/k3os/node"
	nodeLink := "/etc/rancher/node"

	if err := h.deps.FS.MkdirAll(etcRancher, 0o755); err != nil {
		return fmt.Errorf("mkdir /etc/rancher: %w", err)
	}
	if err := h.deps.FS.MkdirAll(nodeDir, 0o755); err != nil {
		return fmt.Errorf("mkdir node dir: %w", err)
	}
	if err := h.deps.FS.Symlink(nodeDir, nodeLink); err != nil {
		return fmt.Errorf("symlink node: %w", err)
	}
	return nil
}
