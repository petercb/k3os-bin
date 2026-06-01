//go:build linux

package finalize

import (
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// GrowLive grows the root partition if mode is "local" and a growpart marker
// exists at /k3os/system/growpart.
func (f *Finalizer) GrowLive() error {
	if f.Mode != "local" {
		return nil
	}

	data, err := f.FS.ReadFile("/k3os/system/growpart")
	if err != nil {
		// File does not exist, nothing to do.
		return nil
	}

	slog.Debug("finalize: growing live partition")

	parts := strings.Fields(strings.TrimSpace(string(data)))
	if len(parts) < 2 {
		return fmt.Errorf("invalid growpart content: %q", string(data))
	}

	dev := parts[0]
	num := parts[1]
	part := dev + num

	// Check if the partition device exists directly; if not, detect via BlockProber.
	if _, err := f.FS.Stat(part); err != nil {
		// Detect partition by label.
		output, err := f.BlockProber.FindByLabel("K3OS_STATE")
		if err != nil {
			return fmt.Errorf("find K3OS_STATE by label: %w", err)
		}
		part = output
		// Derive dev and num from partition path.
		dev, num = splitPartition(part)
	}

	slog.Debug("finalize: growing partition", "dev", dev, "num", num, "part", part)

	if err := f.Cmd.Run("parted", dev, "resizepart", num, "yes", "100%"); err != nil {
		return fmt.Errorf("parted resizepart: %w", err)
	}
	if err := f.Cmd.Run("partprobe", dev); err != nil {
		return fmt.Errorf("partprobe: %w", err)
	}
	if f.SleepFunc != nil {
		f.SleepFunc(2 * time.Second)
	}
	if err := f.Cmd.Run("resize2fs", part); err != nil {
		return fmt.Errorf("resize2fs: %w", err)
	}

	if err := f.FS.Remove("/k3os/system/growpart"); err != nil {
		return fmt.Errorf("remove growpart marker: %w", err)
	}

	return nil
}

// splitPartition splits a partition path like /dev/sda2 or /dev/nvme0n1p2
// into device and partition number.
func splitPartition(part string) (string, string) {
	// Find the last group of digits.
	i := len(part) - 1
	for i >= 0 && part[i] >= '0' && part[i] <= '9' {
		i--
	}
	num := part[i+1:]
	dev := part[:i+1]
	// Remove trailing 'p' for nvme-style devices (e.g., /dev/nvme0n1p2 -> /dev/nvme0n1).
	if len(dev) > 0 && dev[len(dev)-1] == 'p' && i > 0 && dev[i-1] >= '0' && dev[i-1] <= '9' {
		dev = dev[:len(dev)-1]
	}
	return dev, num
}
