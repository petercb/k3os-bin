package module

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/petercb/k3os-bin/internal/config"
	"github.com/sirupsen/logrus"
	"pault.ag/go/modprobe"
)

const (
	procModulesFile = "/proc/modules"
)

func LoadModules(cfg *config.CloudConfig) error {
	loaded := map[string]bool{}
	f, err := os.Open(procModulesFile)
	if err != nil {
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		loaded[strings.SplitN(sc.Text(), " ", 2)[0]] = true
	}
	modules := cfg.K3OS.Modules
	for _, m := range modules {
		if loaded[m] {
			continue
		}
		params := strings.Split(m, " ")
		logrus.Debugf("module %s with parameters [%s] is loading", m, params)
		if err := modprobe.Load(params[0], strings.Join(params[1:], " ")); err != nil {
			return fmt.Errorf("could not load module %s with parameters [%s], err %w", m, params, err)
		}
		logrus.Debugf("module %s is loaded", m)
	}
	return sc.Err()
}
