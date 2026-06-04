// Package rc implements the k3OS rc (run-control) sub-command.
//
// NOTE: As of the Go-based init implementation, the boot sequence calls
// Run() directly in-process during the bootstrap phase. The "k3os rc" CLI
// command is deprecated and retained only for backward compatibility and
// debugging purposes.
package rc

// forked from https://github.com/linuxkit/linuxkit

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/petercb/k3os-bin/internal/devpopulate"
	"github.com/petercb/k3os-bin/internal/iface"
	"github.com/petercb/k3os-bin/internal/iface/osimpl"
	"github.com/petercb/k3os-bin/internal/modalias"
	"github.com/petercb/k3os-bin/internal/mount"
	"github.com/petercb/k3os-bin/internal/namespace"
	"github.com/u-root/u-root/pkg/libinit"
	cli "github.com/urfave/cli/v3"
	"golang.org/x/sys/unix"
)

// Run executes the rc sequence directly (mounts, hotplug, clock, loopback,
// hostname, resolv.conf). It is called by the bootstrap phase in-process
// and also by the CLI action when invoked as "k3os rc".
func Run() error {
	doMounts()
	doHotplug()
	doClock()
	doLoopback()
	doHostname()
	doResolvConf()
	return nil
}

// Command returns the CLI command for early phase run commands and run control.
//
// Deprecated: The boot sequence now calls Run() directly in-process.
// This CLI command is retained for backward compatibility and debugging.
func Command() *cli.Command {
	return &cli.Command{
		Name:  "rc",
		Usage: "[deprecated] early phase run control (now handled internally during boot)",
		Flags: []cli.Flag{},
		Before: func(_ context.Context, _ *cli.Command) (context.Context, error) {
			if os.Getuid() != 0 {
				return nil, errors.New("must be run as root")
			}
			return nil, nil
		},
		Action: func(_ context.Context, _ *cli.Command) error {
			return Run()
		},
	}
}

const (
	nodev    = unix.MS_NODEV
	noexec   = unix.MS_NOEXEC
	nosuid   = unix.MS_NOSUID
	rec      = unix.MS_REC
	relatime = unix.MS_RELATIME
	shared   = unix.MS_SHARED
)

// clockSyncer is the RTC-to-system-clock synchronizer used by doClock.
var clockSyncer iface.ClockSyncer = osimpl.RTCClockSyncer{}

// MountPool tracks all mounts performed during early boot for ordered teardown.
// It is intended to be consumed by a single shutdown hook; concurrent writes
// during boot are safe (mutex-protected) but callers should not call UnmountAll
// until the boot sequence is complete.
var MountPool = mount.NewPool(mount.Unmount)

// rcNamespace declares all the mounts, directories, devices, and symlinks
// that doMounts() creates during early boot.
var rcNamespace = []namespace.Creator{
	// mount proc filesystem
	namespace.Mount{Source: "proc", Target: "/proc", FSType: "proc", Flags: nodev | nosuid | noexec | relatime, Silent: true},

	// mount tmpfs for /tmp and /run
	namespace.Mount{Source: "tmpfs", Target: "/run", FSType: "tmpfs", Flags: nodev | nosuid | noexec | relatime, Data: "size=10%,mode=755"},
	namespace.Mount{Source: "tmpfs", Target: "/tmp", FSType: "tmpfs", Flags: nodev | nosuid | noexec | relatime, Data: "size=10%,mode=1777"},

	// add standard directories in /var
	namespace.Dir{Name: "/var/cache", Mode: 0o755},
	namespace.Dir{Name: "/var/empty", Mode: 0o555},
	namespace.Dir{Name: "/var/lib", Mode: 0o755},
	namespace.Dir{Name: "/var/local/bin", Mode: 0o755},
	namespace.Dir{Name: "/var/lock", Mode: 0o755},
	namespace.Dir{Name: "/var/log", Mode: 0o755},
	namespace.Dir{Name: "/var/opt", Mode: 0o755},
	namespace.Dir{Name: "/var/spool", Mode: 0o755},
	namespace.Dir{Name: "/var/tmp", Mode: 0o1777},
	namespace.Dir{Name: "/home", Mode: 0o755},
	namespace.Symlink{Target: "/run", NewPath: "/var/run"},

	// mount devfs
	namespace.Mount{Source: "dev", Target: "/dev", FSType: "devtmpfs", Flags: nosuid | noexec | relatime, Data: "size=10m,nr_inodes=248418,mode=755"},
	// make minimum necessary devices
	namespace.Dev{Name: "/dev/console", Mode: 0o600, Major: 5, Minor: 1},
	namespace.Dev{Name: "/dev/tty1", Mode: 0o620, Major: 4, Minor: 1},
	namespace.Dev{Name: "/dev/tty", Mode: 0o666, Major: 5, Minor: 0},
	namespace.Dev{Name: "/dev/null", Mode: 0o666, Major: 1, Minor: 3},
	namespace.Dev{Name: "/dev/kmsg", Mode: 0o660, Major: 1, Minor: 11},
	// make standard symlinks
	namespace.Symlink{Target: "/proc/self/fd", NewPath: "/dev/fd"},
	namespace.Symlink{Target: "/proc/self/fd/0", NewPath: "/dev/stdin"},
	namespace.Symlink{Target: "/proc/self/fd/1", NewPath: "/dev/stdout"},
	namespace.Symlink{Target: "/proc/self/fd/2", NewPath: "/dev/stderr"},
	namespace.Symlink{Target: "/proc/kcore", NewPath: "/dev/kcore"},
	// dev mountpoints
	namespace.Dir{Name: "/dev/mqueue", Mode: 0o1777},
	namespace.Dir{Name: "/dev/shm", Mode: 0o1777},
	namespace.Dir{Name: "/dev/pts", Mode: 0o755},
	// mounts on /dev
	namespace.Mount{Source: "mqueue", Target: "/dev/mqueue", FSType: "mqueue", Flags: noexec | nosuid | nodev},
	namespace.Mount{Source: "shm", Target: "/dev/shm", FSType: "tmpfs", Flags: noexec | nosuid | nodev, Data: "mode=1777"},
	namespace.Mount{Source: "devpts", Target: "/dev/pts", FSType: "devpts", Flags: noexec | nosuid, Data: "gid=5,mode=0620"},

	// sysfs
	namespace.Mount{Source: "sysfs", Target: "/sys", FSType: "sysfs", Flags: noexec | nosuid | nodev},
	// some of the subsystems may not exist, so ignore errors
	namespace.Mount{Source: "securityfs", Target: "/sys/kernel/security", FSType: "securityfs", Flags: noexec | nosuid | nodev, Silent: true},
	namespace.Mount{Source: "debugfs", Target: "/sys/kernel/debug", FSType: "debugfs", Flags: noexec | nosuid | nodev, Silent: true},
	namespace.Mount{Source: "configfs", Target: "/sys/kernel/config", FSType: "configfs", Flags: noexec | nosuid | nodev, Silent: true},
	namespace.Mount{Source: "fusectl", Target: "/sys/fs/fuse/connections", FSType: "fusectl", Flags: noexec | nosuid | nodev, Silent: true},
	namespace.Mount{Source: "selinuxfs", Target: "/sys/fs/selinux", FSType: "selinuxfs", Flags: noexec | nosuid, Silent: true},
	namespace.Mount{Source: "pstore", Target: "/sys/fs/pstore", FSType: "pstore", Flags: noexec | nosuid | nodev, Silent: true},
	namespace.Mount{Source: "efivarfs", Target: "/sys/firmware/efi/efivars", FSType: "efivarfs", Flags: noexec | nosuid | nodev, Silent: true},

	// misc /proc mounted fs
	namespace.Mount{Source: "binfmt_misc", Target: "/proc/sys/fs/binfmt_misc", FSType: "binfmt_misc", Flags: noexec | nosuid | nodev, Silent: true},

	// mount cgroup2 unified hierarchy
	namespace.Mount{Source: "cgroup2", Target: "/sys/fs/cgroup", FSType: "cgroup2", Flags: nodev | noexec | nosuid},

	// make / rshared
	namespace.Mount{Target: "/", Flags: rec | shared},
}

// write a file, eg sysfs
func write(path string, value string) {
	err := os.WriteFile(path, []byte(value), 0o600)
	if err != nil {
		log.Printf("cannot write to %s: %v", path, err)
	}
}

// read a file, eg sysfs, strip whitespace, empty string if does not exist
func read(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// read a directory
func readdir(path string) []string {
	names := []string{}
	files, err := os.ReadDir(path)
	if err != nil {
		log.Printf("cannot read directory %s: %v", path, err)
		return names
	}
	for _, f := range files {
		names = append(names, f.Name())
	}
	return names
}

// glob logging errors
func glob(pattern string) []string {
	files, err := filepath.Glob(pattern)
	if err != nil {
		log.Printf("error in glob %s: %v", pattern, err)
		return []string{}
	}
	return files
}

// test if a file exists
func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// modaliases runs modprobe on the modalias(es) file contents
func modaliases(paths ...string) {
	ma, err := modalias.Init()
	if err != nil {
		log.Printf("ERROR failed to parse module aliases: %v", err)
		return
	}

	log.Println("INFO: Loading kernel modules")
	for _, path := range paths {
		aliases := strings.Fields(read(path))

		for _, alias := range aliases {
			if err := ma.Load(alias); err != nil {
				if !strings.Contains(err.Error(), "Module isn't in the module directory") {
					log.Printf("ERROR Kernel load [%s]: %v", alias, err)
				}
			}
		}
	}
}

func doMounts() {
	_ = namespace.ApplyTracked(rcNamespace, MountPool, slog.Default())
}

func doHotplug() {
	// Populate /dev with device nodes and create /dev/disk/by-label and
	// /dev/disk/by-uuid symlinks. This replaces the previous "mdev -s"
	// shell-out with a pure Go implementation that:
	//   1. Walks /sys/class/block and ensures device nodes exist (via Mknod;
	//      on devtmpfs this is a harmless no-op as nodes already exist).
	//   2. Probes each block device for filesystem labels/UUIDs using
	//      go-blockdevice/v2's blkid and creates the appropriate symlinks.
	if err := devpopulate.PopulateDev(devpopulate.DefaultOptions()); err != nil {
		log.Printf("Failed to populate /dev: %v", err)
	}

	// Trigger USB uevent replay for hotplug devices.
	devices := "/sys/devices"
	files := readdir(devices)
	for _, f := range files {
		uevent := filepath.Join(devices, f, "uevent")
		if strings.HasPrefix(f, "usb") && exists(uevent) {
			write(uevent, "add")
		}
	}

	// Load kernel modules for all existing cold plug devices.
	modaliases(glob("/sys/bus/*/devices/*/modalias")...)
}

func doClock() {
	if err := clockSyncer.SyncRTC(); err != nil {
		log.Printf("Failed to sync RTC: %v", err)
	}
}

func doResolvConf() {
	// for containerizing dhcpcd and other containers that need writable /etc/resolv.conf
	// if it is a symlink (usually to /run) make the directory and empty file
	link, err := os.Readlink("/etc/resolv.conf")
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		log.Printf("error making directory %s: %v", filepath.Dir(link), err)
	}
	write(link, "")
}

func doLoopback() {
	// NetInit brings up the loopback interface via netlink but does not assign
	// 127.0.0.1/8 or add a host route. On kernel 6.8+ the address is
	// configured automatically when lo is set up; older kernels will have a
	// functioning interface with no address bound.
	libinit.NetInit()
}

func doHostname() {
	hostname := read("/etc/hostname")
	if hostname != "" {
		if err := unix.Sethostname([]byte(hostname)); err != nil {
			log.Printf("Setting hostname failed: %v", err)
		}
	}
	hostname, err := os.Hostname()
	if err != nil {
		log.Printf("Cannot read hostname: %v", err)
		return
	}

	if hostname != "(none)" && hostname != "" {
		return
	}

	mac := read("/sys/class/net/eth0/address")
	if mac == "" {
		return
	}

	mac = strings.ReplaceAll(mac, ":", "")
	if err := unix.Sethostname([]byte("k3os-" + mac)); err != nil {
		log.Printf("Setting hostname failed: %v", err)
	}
}
