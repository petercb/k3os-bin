// Package hostname provides utilities for setting and managing the system hostname.
package hostname

import (
	"bufio"
	"strings"

	"github.com/petercb/k3os-bin/internal/config"
	"github.com/petercb/k3os-bin/internal/iface"
)

// SetHostname applies the configured hostname and syncs hostname files.
func SetHostname(c *config.CloudConfig, hs iface.HostnameSetter, fs iface.FileSystem) error {
	hostname := c.Hostname
	if hostname == "" {
		return nil
	}
	if err := hs.SetHostname(hostname); err != nil {
		return err
	}
	return syncHostname(fs)
}

func syncHostname(fs iface.FileSystem) error {
	hostname, err := fs.Hostname()
	if err != nil {
		return err
	}
	if hostname == "" {
		return nil
	}

	if writeErr := fs.WriteFile("/etc/hostname", []byte(hostname+"\n"), 0o644); writeErr != nil {
		return writeErr
	}

	hosts, err := fs.Open("/etc/hosts")
	if err != nil {
		return err
	}
	defer func() { _ = hosts.Close() }()

	lines := bufio.NewScanner(hosts)
	content := ""
	for lines.Scan() {
		line := strings.TrimSpace(lines.Text())
		fields := strings.Fields(line)
		if len(fields) > 0 && fields[0] == "127.0.1.1" {
			content += "127.0.1.1 " + hostname + "\n"
			continue
		}
		content += line + "\n"
	}
	return fs.WriteFile("/etc/hosts", []byte(content), 0o600)
}
