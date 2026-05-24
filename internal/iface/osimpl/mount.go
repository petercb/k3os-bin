//go:build linux
// +build linux

package osimpl

import "github.com/petercb/k3os-bin/internal/mount"

// LinuxMounter implements iface.Mounter using real Linux mount syscalls.
type LinuxMounter struct{}

// Mount mounts a device at a target path.
func (LinuxMounter) Mount(device, target, mType, options string) error {
	return mount.Mount(device, target, mType, options)
}

// ForceMount forcefully mounts a device at a target path.
func (LinuxMounter) ForceMount(device, target, mType, options string) error {
	return mount.ForceMount(device, target, mType, options)
}

// Mounted reports whether the target path is currently mounted.
func (LinuxMounter) Mounted(target string) (bool, error) {
	return mount.Mounted(target)
}
