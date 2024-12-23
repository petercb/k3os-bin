package sysctl

import (
	"os"
	"path"
	"strings"

	"github.com/petercb/k3os-bin/internal/config"
)

func ConfigureSysctl(cfg *config.CloudConfig) error {
	for k, v := range cfg.K3OS.Sysctls {
		elements := []string{"/proc", "sys"}
		elements = append(elements, strings.Split(k, ".")...)
		path := path.Join(elements...)
		if err := os.WriteFile(path, []byte(v), 0644); err != nil {
			return err
		}
	}
	return nil
}
