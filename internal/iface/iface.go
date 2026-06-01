// Package iface defines injectable operating-system boundaries.
package iface

import (
	"io"
	"os"
)

// File abstracts read/write/close/name operations on a single file handle.
type File interface {
	io.ReadWriteCloser
	Name() string
}

// DirEntry abstracts directory entry information.
type DirEntry interface {
	Name() string
	IsDir() bool
	Type() os.FileMode
	Info() (os.FileInfo, error)
}

// FileSystem abstracts file I/O operations used by appliers.
type FileSystem interface {
	WriteFile(name string, data []byte, perm os.FileMode) error
	ReadFile(name string) ([]byte, error)
	MkdirAll(path string, perm os.FileMode) error
	Stat(name string) (os.FileInfo, error)
	Lstat(name string) (os.FileInfo, error)
	Open(name string) (File, error)
	Create(name string) (File, error)
	CreateTemp(dir, pattern string) (File, error)
	Chown(name string, uid, gid int) error
	Chmod(name string, mode os.FileMode) error
	Rename(oldpath, newpath string) error
	Remove(name string) error
	RemoveAll(path string) error
	Symlink(oldname, newname string) error
	Readlink(name string) (string, error)
	ReadDir(name string) ([]DirEntry, error)
	Hostname() (string, error)
}

// CommandRunner abstracts shell command execution.
type CommandRunner interface {
	Run(name string, args ...string) error
	RunOutput(name string, args ...string) (string, error)
	RunWithStdin(stdin string, name string, args ...string) error
	RunShell(command string) error
	RunWithEnv(env []string, name string, args ...string) error
}

// ModuleLoader abstracts kernel module loading.
type ModuleLoader interface {
	LoadedModules() (map[string]bool, error)
	LoadModule(name string, params string) error
}

// SysctlApplier abstracts sysctl configuration.
type SysctlApplier interface {
	Set(key string, value string) error
}

// Mounter abstracts filesystem mount operations.
type Mounter interface {
	Mount(device, target, mType, options string) error
	ForceMount(device, target, mType, options string) error
	Mounted(target string) (bool, error)
}

// HostnameSetter abstracts the syscall to set the system hostname.
type HostnameSetter interface {
	SetHostname(name string) error
}

// BlockProber abstracts block device discovery.
type BlockProber interface {
	// FindByLabel returns the device path for a filesystem label.
	FindByLabel(label string) (string, error)
	// ListDisks returns device names of all block devices of type "disk".
	ListDisks() ([]string, error)
}

// LoopDevice abstracts a single attached loop device.
type LoopDevice interface {
	Path() string
	Detach() error
	SetAutoclear() error
}

// LoopAttacher abstracts loop device attachment operations.
type LoopAttacher interface {
	Attach(backingFile string, offset uint64, readOnly bool) (LoopDevice, error)
}

// PartitionGrower grows a partition to fill available space on a GPT disk.
type PartitionGrower interface {
	GrowPartition(device string, partNum int) error
}

// CmdlineParser abstracts kernel command-line parsing.
type CmdlineParser interface {
	Flag(name string) (string, bool)
	Contains(name string) bool
	Consoles() []string
	Raw() string
}
