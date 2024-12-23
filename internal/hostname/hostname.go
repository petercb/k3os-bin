package hostname

import (
	"bufio"
	"os"
	"strings"
	"syscall"

	"github.com/petercb/k3os-bin/internal/config"
)

func SetHostname(c *config.CloudConfig) error {
	hostname := c.Hostname
	if hostname == "" {
		return nil
	}
	if err := syscall.Sethostname([]byte(hostname)); err != nil {
		return err
	}
	return syncHostname()
}

func syncHostname() error {
	hostname, err := os.Hostname()
	if err != nil {
		return err
	}
	if hostname == "" {
		return nil
	}

	if err := os.WriteFile("/etc/hostname", []byte(hostname+"\n"), 0644); err != nil {
		return err
	}

	hosts, err := os.Open("/etc/hosts")
	if err != nil {
		return err
	}
	defer hosts.Close()

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
	return os.WriteFile("/etc/hosts", []byte(content), 0600)
}
