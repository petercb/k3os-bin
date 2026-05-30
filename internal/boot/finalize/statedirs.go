//go:build linux

package finalize

import (
	"fmt"
	"log/slog"
)

// SetupStateDirs creates required state directories.
func (f *Finalizer) SetupStateDirs() error {
	slog.Debug("finalize: setting up state dirs")

	dirs := []string{
		"/var/lib/nfs",
		"/var/lib/rancher/k3s/agent/libexec/kubernetes",
	}

	for _, dir := range dirs {
		if err := f.FS.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", dir, err)
		}
	}

	return nil
}
