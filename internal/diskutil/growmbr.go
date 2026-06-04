//go:build linux

package diskutil

import (
	"fmt"
	"os/exec"
	"strings"
)

// growMBR grows an MBR partition to fill all available space using sfdisk.
// It runs: echo ", +" | sfdisk --no-reread -N <partNum> <device>
// which tells sfdisk to keep the start sector and expand the partition size
// to fill all remaining contiguous space on the device.
func growMBR(device string, partNum int) error {
	if partNum < 1 {
		return fmt.Errorf("invalid partition number %d: must be >= 1", partNum)
	}

	// Use sfdisk -N to resize a single partition.
	// Input ", +" means: keep start sector unchanged, grow size to max.
	partFlag := fmt.Sprintf("-N%d", partNum)
	cmd := exec.Command("sfdisk", "--no-reread", partFlag, device)
	cmd.Stdin = strings.NewReader(", +\n")

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("sfdisk resize partition %d on %s: %w (output: %s)",
			partNum, device, err, strings.TrimSpace(string(out)))
	}

	return nil
}
