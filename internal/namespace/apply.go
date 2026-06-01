//go:build linux

package namespace // import "github.com/petercb/k3os-bin/internal/namespace"

import "log/slog"

// Apply iterates over all creators, calling Create() on each. Errors are
// logged but never stop iteration. Apply always returns nil.
func Apply(creators []Creator, logger *slog.Logger) error {
	if logger == nil {
		logger = slog.Default()
	}

	for _, c := range creators {
		if err := c.Create(); err != nil {
			logger.Warn("namespace: create failed", "item", c, "error", err)
		}
	}

	return nil
}
