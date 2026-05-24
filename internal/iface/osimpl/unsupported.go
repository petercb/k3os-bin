//go:build !linux
// +build !linux

package osimpl

import "fmt"

// LinuxModuleLoader reports unsupported Linux module operations on non-Linux hosts.
type LinuxModuleLoader struct{}

// LoadedModules reports that Linux module inspection is unsupported.
func (LinuxModuleLoader) LoadedModules() (map[string]bool, error) {
	return nil, unsupported("loaded modules")
}

// LoadModule reports that Linux module loading is unsupported.
func (LinuxModuleLoader) LoadModule(_ string, _ string) error {
	return unsupported("load module")
}

// LinuxSysctlApplier reports unsupported sysctl operations on non-Linux hosts.
type LinuxSysctlApplier struct{}

// Set reports that Linux sysctl configuration is unsupported.
func (LinuxSysctlApplier) Set(_ string, _ string) error {
	return unsupported("set sysctl")
}

// LinuxMounter reports unsupported mount operations on non-Linux hosts.
type LinuxMounter struct{}

// Mount reports that Linux mount operations are unsupported.
func (LinuxMounter) Mount(_, _, _, _ string) error {
	return unsupported("mount")
}

// ForceMount reports that Linux force mount operations are unsupported.
func (LinuxMounter) ForceMount(_, _, _, _ string) error {
	return unsupported("force mount")
}

// Mounted reports that Linux mount inspection is unsupported.
func (LinuxMounter) Mounted(_ string) (bool, error) {
	return false, unsupported("mounted")
}

// LinuxHostnameSetter reports unsupported hostname operations on non-Linux hosts.
type LinuxHostnameSetter struct{}

// SetHostname reports that Linux hostname changes are unsupported.
func (LinuxHostnameSetter) SetHostname(_ string) error {
	return unsupported("set hostname")
}

func unsupported(operation string) error {
	return fmt.Errorf("%s is only supported on linux", operation)
}
