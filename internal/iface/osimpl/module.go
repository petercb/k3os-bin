//go:build linux
// +build linux

package osimpl

import (
	"bufio"
	"os"
	"strings"

	"pault.ag/go/modprobe"
)

// LinuxModuleLoader implements iface.ModuleLoader using /proc/modules and modprobe.
type LinuxModuleLoader struct{}

// LoadedModules returns the set of currently loaded Linux kernel modules.
func (LinuxModuleLoader) LoadedModules() (map[string]bool, error) {
	loaded := map[string]bool{}
	f, err := os.Open("/proc/modules")
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = f.Close()
	}()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		loaded[strings.SplitN(sc.Text(), " ", 2)[0]] = true
	}
	return loaded, sc.Err()
}

// LoadModule loads a Linux kernel module with optional parameters.
func (LinuxModuleLoader) LoadModule(name string, params string) error {
	return modprobe.Load(name, params)
}
