//go:build linux

package finalize

import (
	"fmt"
	"log/slog"
	"os"
)

// SetupHostname reads the hostname from /var/lib/rancher/k3os/hostname or
// generates a random one, then writes it to /etc/hostname.
func (f *Finalizer) SetupHostname() error {
	slog.Debug("finalize: setting up hostname")

	// If /etc/hostname already exists, nothing to do.
	if _, err := f.FS.Stat("/etc/hostname"); err == nil {
		return nil
	}

	// Try reading persisted hostname.
	if data, err := f.FS.ReadFile("/var/lib/rancher/k3os/hostname"); err == nil {
		hostname := string(data)
		if err := f.FS.WriteFile("/etc/hostname", []byte(hostname), 0o644); err != nil {
			return fmt.Errorf("write /etc/hostname: %w", err)
		}
		return nil
	}

	// Generate random hostname.
	randVal, err := f.RandFunc()
	if err != nil {
		return fmt.Errorf("generate random value: %w", err)
	}
	hostname := fmt.Sprintf("k3os-%d", randVal)

	if err := f.FS.MkdirAll("/var/lib/rancher/k3os", 0o755); err != nil {
		return fmt.Errorf("mkdir hostname dir: %w", err)
	}
	if err := f.FS.WriteFile("/var/lib/rancher/k3os/hostname", []byte(hostname+"\n"), 0o644); err != nil {
		return fmt.Errorf("write persisted hostname: %w", err)
	}
	if err := f.FS.WriteFile("/etc/hostname", []byte(hostname+"\n"), 0o644); err != nil {
		return fmt.Errorf("write /etc/hostname: %w", err)
	}

	return nil
}

// SetupHosts generates /etc/hosts if it does not already exist.
func (f *Finalizer) SetupHosts() error {
	slog.Debug("finalize: setting up hosts")

	if _, err := f.FS.Stat("/etc/hosts"); err == nil {
		return nil
	}

	data, err := f.FS.ReadFile("/etc/hostname")
	if err != nil {
		return fmt.Errorf("read /etc/hostname: %w", err)
	}

	hostname := trimNewline(string(data))

	hosts := fmt.Sprintf(`127.0.0.1	localhost localhost.localdomain
127.0.1.1	%s %s.localdomain

::1     ip6-localhost ip6-loopback
fe00::0 ip6-localnet
ff00::0 ip6-mcastprefix
ff02::1 ip6-allnodes
ff02::2 ip6-allrouters
`, hostname, hostname)

	if err := f.FS.WriteFile("/etc/hosts", []byte(hosts), 0o644); err != nil {
		return fmt.Errorf("write /etc/hosts: %w", err)
	}

	return nil
}

// SetupRoot creates /root with mode 0700 if it does not exist.
func (f *Finalizer) SetupRoot() error {
	slog.Debug("finalize: setting up /root")

	if _, err := f.FS.Stat("/root"); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("stat /root: %w", err)
		}
		if err := f.FS.MkdirAll("/root", 0o700); err != nil {
			return fmt.Errorf("mkdir /root: %w", err)
		}
		if err := f.FS.Chmod("/root", 0o700); err != nil {
			return fmt.Errorf("chmod /root: %w", err)
		}
	}

	return nil
}

// trimNewline removes trailing newline characters from a string.
func trimNewline(s string) string {
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	return s
}
