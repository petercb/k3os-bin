//go:build linux

package finalize

import (
	"fmt"
	"log/slog"
)

// SetupManifests creates the k3s manifests directory and copies manifests
// from the system share, excluding example files.
func (f *Finalizer) SetupManifests() error {
	slog.Debug("finalize: setting up manifests")

	if err := f.FS.MkdirAll("/var/lib/rancher/k3s/server/manifests", 0o755); err != nil {
		return fmt.Errorf("mkdir manifests: %w", err)
	}

	if err := f.Cmd.Run("rsync", "-a", "--exclude=*.example",
		"/usr/share/rancher/k3s/server/manifests/",
		"/var/lib/rancher/k3s/server/manifests/"); err != nil {
		return fmt.Errorf("rsync manifests: %w", err)
	}

	return nil
}
