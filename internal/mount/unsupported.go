//go:build !linux
// +build !linux

// Package mount provides Linux mount helpers.
package mount

import "fmt"

// Mounted reports unsupported mount inspection on non-Linux hosts.
func Mounted(_ string) (bool, error) {
	return false, unsupported("mounted")
}

// Mount reports unsupported mount operations on non-Linux hosts.
func Mount(_, _, _, _ string) error {
	return unsupported("mount")
}

// ForceMount reports unsupported force mount operations on non-Linux hosts.
func ForceMount(_, _, _, _ string) error {
	return unsupported("force mount")
}

func unsupported(operation string) error {
	return fmt.Errorf("%s is only supported on linux", operation)
}
