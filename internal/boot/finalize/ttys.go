//go:build linux

package finalize

import (
	"fmt"
	"log/slog"
	"strings"
)

// SetupTTYs configures TTY entries in /etc/inittab and /etc/securetty.
// It adds entries for tty1-6 (if devices exist) and parses /proc/cmdline
// for console= entries to add serial consoles.
func (f *Finalizer) SetupTTYs() error {
	slog.Debug("finalize: setting up TTYs")

	var inittab strings.Builder
	var securetty strings.Builder

	// Track which TTYs have been added to avoid duplicates.
	seen := make(map[string]bool)

	// Add standard TTYs (1-6).
	for i := 1; i <= 6; i++ {
		tty := fmt.Sprintf("tty%d", i)
		devPath := fmt.Sprintf("/dev/%s", tty)
		if _, err := f.FS.Stat(devPath); err == nil {
			fmt.Fprintf(&inittab, "%s::respawn:/sbin/getty 38400 %s\n", tty, tty)
			securetty.WriteString(tty + "\n")
			seen[tty] = true
		}
	}

	// Parse cmdline for serial consoles.
	serialEntries := parseConsoleEntries(f.Cmdline.Raw())
	for _, entry := range serialEntries {
		if seen[entry.tty] {
			continue
		}
		devPath := fmt.Sprintf("/dev/%s", entry.tty)
		if _, err := f.FS.Stat(devPath); err == nil {
			fmt.Fprintf(&inittab, "%s::respawn:/sbin/getty -L %s %s vt100\n", entry.tty, entry.baudrate, entry.tty)
			securetty.WriteString(entry.tty + "\n")
			seen[entry.tty] = true
		}
	}

	// Append to /etc/inittab.
	if inittab.Len() > 0 {
		existing, _ := f.FS.ReadFile("/etc/inittab")
		content := string(existing) + inittab.String()
		if err := f.FS.WriteFile("/etc/inittab", []byte(content), 0o644); err != nil {
			return fmt.Errorf("write /etc/inittab: %w", err)
		}
	}

	// Append to /etc/securetty.
	if securetty.Len() > 0 {
		existing, _ := f.FS.ReadFile("/etc/securetty")
		content := string(existing) + securetty.String()
		if err := f.FS.WriteFile("/etc/securetty", []byte(content), 0o644); err != nil {
			return fmt.Errorf("write /etc/securetty: %w", err)
		}
	}

	return nil
}

// consoleEntry represents a parsed console= kernel parameter.
type consoleEntry struct {
	tty      string
	baudrate string
}

// parseConsoleEntries extracts console= entries from a kernel cmdline string.
// Format: console=ttyS0,115200n8 -> tty=ttyS0, baudrate=115200
func parseConsoleEntries(cmdline string) []consoleEntry {
	var entries []consoleEntry
	for _, field := range strings.Fields(cmdline) {
		if !strings.HasPrefix(field, "console=") {
			continue
		}
		spec := strings.TrimPrefix(field, "console=")
		parts := strings.SplitN(spec, ",", 2)
		tty := parts[0]
		baudrate := "9600"
		if len(parts) > 1 && parts[1] != "" {
			// Extract numeric baudrate (e.g., "115200n8" -> "115200").
			baudrate = extractBaudrate(parts[1])
		}
		entries = append(entries, consoleEntry{tty: tty, baudrate: baudrate})
	}
	return entries
}

// extractBaudrate extracts the leading numeric portion of a baudrate spec.
func extractBaudrate(spec string) string {
	var digits strings.Builder
	for _, ch := range spec {
		if ch >= '0' && ch <= '9' {
			digits.WriteRune(ch)
		} else {
			break
		}
	}
	if digits.Len() == 0 {
		return "9600"
	}
	return digits.String()
}
