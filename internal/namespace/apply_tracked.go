//go:build linux

package namespace

import (
	"errors"
	"log/slog"

	"github.com/petercb/k3os-bin/internal/mount"
)

// Trackable is implemented by creators that produce tracked mount points.
type Trackable interface {
	AsMountPoint() *mount.Point
}

// ApplyTracked iterates creators like Apply, recording successful mounts into pool.
// If pool is nil, it behaves identically to Apply. Creators that return
// ErrSilentSkip are neither logged nor recorded -- this indicates a silent
// mount that was intentionally skipped.
func ApplyTracked(creators []Creator, pool *mount.Pool, logger *slog.Logger) error {
	if logger == nil {
		logger = slog.Default()
	}

	for _, c := range creators {
		if err := c.Create(); err != nil {
			if errors.Is(err, ErrSilentSkip) {
				continue
			}

			logger.Warn("namespace: create failed", "item", c, "error", err)

			continue
		}

		if pool != nil {
			if t, ok := c.(Trackable); ok {
				pool.Add(t.AsMountPoint())
			}
		}
	}

	return nil
}
