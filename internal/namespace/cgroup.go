//go:build linux

package namespace

import (
	"encoding/csv"
	"log"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// CgroupMounts mounts cgroup filesystems for all enabled cgroups found in
// /proc/cgroups.
type CgroupMounts struct{}

// Create reads /proc/cgroups and mounts a cgroup filesystem for each enabled
// subsystem under /sys/fs/cgroup/. Errors are logged and iteration continues
// to match the original boot behavior where individual failures do not prevent
// remaining subsystems from being mounted.
func (c CgroupMounts) Create() error {
	for _, cg := range cgroupList() {
		path := filepath.Join("/sys/fs/cgroup", cg)
		if err := os.MkdirAll(path, 0o555); err != nil {
			log.Printf("cgroup mkdir %s: %v", path, err)
			continue
		}
		flags := unix.MS_NOEXEC | unix.MS_NOSUID | unix.MS_NODEV
		if err := unix.Mount(cg, path, "cgroup", uintptr(flags), cg); err != nil {
			log.Printf("cgroup mount %s: %v", path, err)
		}
	}
	return nil
}

func (c CgroupMounts) String() string {
	return "cgroup-mounts{/sys/fs/cgroup/*}"
}

// cgroupList returns the names of all enabled cgroups from /proc/cgroups.
func cgroupList() []string {
	list := []string{}
	f, err := os.Open("/proc/cgroups")
	if err != nil {
		return list
	}
	defer func() { _ = f.Close() }()

	reader := csv.NewReader(f)
	reader.Comma = '\t'
	reader.FieldsPerRecord = 4

	cgroups, err := reader.ReadAll()
	if err != nil {
		return list
	}

	for _, cg := range cgroups {
		if cg[3] == "1" {
			list = append(list, cg[0])
		}
	}
	return list
}
