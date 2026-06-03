// Package main is the entry point for the k3OS binary.
package main

// Copyright 2019 Rancher Labs, Inc.
// SPDX-License-Identifier: Apache-2.0

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/moby/sys/reexec"
	cp "github.com/otiai10/copy"
	"github.com/petercb/k3os-bin/internal/boot"
	"github.com/petercb/k3os-bin/internal/boot/bootstrap"
	"github.com/petercb/k3os-bin/internal/boot/finalize"
	"github.com/petercb/k3os-bin/internal/boot/modes"
	"github.com/petercb/k3os-bin/internal/boot/testmode"
	"github.com/petercb/k3os-bin/internal/cli/app"
	cliconfig "github.com/petercb/k3os-bin/internal/cli/config"
	"github.com/petercb/k3os-bin/internal/cli/rc"
	"github.com/petercb/k3os-bin/internal/cmdline"
	"github.com/petercb/k3os-bin/internal/enterchroot"
	"github.com/petercb/k3os-bin/internal/iface/osimpl"
	"github.com/petercb/k3os-bin/internal/kernel"
	"github.com/petercb/k3os-bin/internal/mode"
	"github.com/petercb/k3os-bin/internal/mount"
	"github.com/petercb/k3os-bin/internal/transferroot"
	"github.com/petercb/k3os-bin/internal/virt"
	cli "github.com/urfave/cli/v3"
	"golang.org/x/sys/unix"
)

func main() {
	reexec.Register("/init", initrd)      // mode=live: kernel boots with /init as argv[0]
	reexec.Register("/sbin/init", initrd) // mode=local: systemd/openrc invokes /sbin/init
	reexec.Register("enter-root", enterchroot.Enter)

	if !reexec.Init() {
		cmd := app.New()
		args := []string{cmd.Name}
		path := filepath.Base(os.Args[0])
		if path != cmd.Name && findCommand(cmd, path) != nil {
			args = append(args, path)
		}
		args = append(args, os.Args[1:]...)
		// this will bomb if the app has any non-defaulted, required flags
		err := cmd.Run(context.Background(), args)
		if err != nil {
			slog.Error("fatal error", "error", err)
			os.Exit(1)
		}
	}
}

// postChrootSentinel is the path that indicates we are running post-chroot.
// The pivot_root in enterchroot uses ".base" as the put-old directory,
// which becomes /.base after the pivot.
var postChrootSentinel = "/.base"

// statFunc is the function used to check for file existence (injectable for tests).
var statFunc = os.Stat

func initrd() {
	// Detect whether we are running pre-chroot (initramfs) or post-chroot.
	// After enterchroot pivots root, /.base exists as the put-old directory.
	if _, err := statFunc(postChrootSentinel); err == nil {
		postChroot()
		return
	}

	// Pre-chroot: relocate, remount, and enter the chroot.
	enterchroot.DebugCmdline = "k3os.debug"
	transferroot.Relocate()
	if err := mount.Mount("", "/", "none", "rw,remount"); err != nil {
		slog.Error("failed to remount root as rw", "error", err)
	}
	if err := enterchroot.Mount("./k3os/data", os.Args, os.Stdout, os.Stderr); err != nil {
		slog.Error("failed to enter root", "error", err)
		os.Exit(1)
	}
}

// postChroot runs the Go-based init orchestrator, replacing the original
// /usr/init shell script. It wires up all real dependencies and calls
// boot.Init.Run().
func postChroot() {
	// Set PATH early so os.Exec or CommandRunner can find binaries in the rootfs.
	// This matches the original shell script: export PATH=/bin:/sbin:/usr/bin:/usr/sbin:...
	_ = os.Setenv("PATH", "/bin:/sbin:/usr/bin:/usr/sbin:/usr/local/bin:/usr/local/sbin")

	fs := osimpl.OSFileSystem{}
	cmd := osimpl.ShellRunner{}
	mounter := osimpl.LinuxMounter{}

	kver, err := kernel.GetKernelVersion()
	if err != nil {
		slog.Error("failed to get kernel version", "error", err)
		kver = "unknown"
	}

	// Read the k3OS version from /usr/lib/os-release if available.
	versionID := readVersionID()

	// Build the mode registry with real dependencies.
	modeDeps := &modes.Deps{
		FS:              fs,
		Cmd:             cmd,
		Mounter:         mounter,
		BlockProber:     osimpl.SysfsBlockProber{},
		PartitionGrower: &osimpl.PartitionGrower{},
		LoopDetacher:    osimpl.LoopPathDetacher{},
		Proc:            &realProcessExecutor{},
		CopyDir:         func(src, dst string) error { return cp.Copy(src, dst) },
		KernelVersion:   kver,
		VersionID:       versionID,
	}
	registry := modes.NewRegistry(modeDeps)

	// Build the bootstrapper.
	bs := &bootstrap.Bootstrapper{
		FS:      fs,
		Mounter: mounter,
		Cmd:     cmd,
		CopyDir: func(src, dst string) error {
			return cp.Copy(src, dst, cp.Options{
				PreserveTimes: true,
				PreserveOwner: true,
			})
		},
		RCRunner:      rc.Run,
		ConfigRunner:  cliconfig.RunInitrd,
		KernelVersion: kver,
	}

	// Build the finalizer.
	cl := cmdline.New()
	fin := &finalize.Finalizer{
		FS:              fs,
		Mounter:         mounter,
		Cmd:             cmd,
		BlockProber:     osimpl.SysfsBlockProber{},
		PartitionGrower: &osimpl.PartitionGrower{},
		SleepFunc:       time.Sleep,
		Cmdline:         cl,
		RandFunc:        cryptoRandUint32,
		VirtDetector:    virt.NewDMIDetector().Detect,
		ConfigRunner:    cliconfig.RunBoot,
		ManifestCopier: func(src, dst string) error {
			return cp.Copy(src, dst, cp.Options{
				PreserveTimes: true,
				PreserveOwner: true,
				Skip: func(_ os.FileInfo, srcPath, _ string) (bool, error) {
					if filepath.Ext(srcPath) == ".example" {
						return true, nil
					}
					return false, nil
				},
			})
		},
	}

	// Build the init orchestrator.
	initOrch := &boot.Init{
		Bootstrap: bs,
		ModeDetector: func() (string, error) {
			detector := &mode.Detector{
				Cmdline:       cl,
				BlockProber:   modeDeps.BlockProber.FindByLabel,
				StatfsChecker: statfsCheck,
				EnvReader:     os.Getenv,
				FileWriter:    os.WriteFile,
				MkdirAll:      os.MkdirAll,
				SleepFunc:     time.Sleep,
				Timeout:       30 * time.Second,
				SleepInterval: 1 * time.Second,
			}
			return detector.Detect(context.Background())
		},
		ModeRegistry: func(m string) (boot.ModeHandler, error) {
			return registry.Get(m)
		},
		Finalizer: fin,
		Cmdline:   cl,
		ExecFunc:  syscall.Exec,
		RescueFunc: func() error {
			return syscall.Exec("/bin/bash", []string{"bash"}, os.Environ())
		},
		ConsoleRedirect: consoleRedirect,
		ModeSetterFunc: func(m string) {
			fin.Mode = m
		},
	}

	// If k3os.test_mode is on the kernel cmdline, replace ExecFunc with the
	// test mode verifier. The init sequence still runs fully (bootstrap, mode
	// detection, mode handler, finalization) but instead of exec'ing OpenRC
	// it runs verification checks and powers off.
	if cl.Contains("k3os.test_mode") {
		initOrch.ExecFunc = func(_ string, _ []string, _ []string) error {
			// Open /dev/ttyS0 directly to ensure test results are written
			// to the serial port, which QEMU captures to the log file.
			serialOut, err := os.OpenFile("/dev/ttyS0", os.O_WRONLY, 0)
			if err != nil {
				// Fall back to stdout if serial port is unavailable.
				serialOut = os.Stdout
			}
			v := &testmode.Verifier{
				StatFunc:     os.Stat,
				ReadFileFunc: os.ReadFile,
				HostnameFunc: os.Hostname,
				Output:       serialOut,
				RebootFunc: func() error {
					return syscall.Reboot(syscall.LINUX_REBOOT_CMD_POWER_OFF)
				},
			}
			return v.Run()
		}
	}

	initOrch.Run()
	// Should not reach here; exec replaces the process.
	os.Exit(1)
}

// consoleRedirect opens /dev/console and redirects stdin, stdout, and stderr
// to it, matching the shell's exec >/dev/console </dev/console 2>&1.
func consoleRedirect() error {
	f, err := os.OpenFile("/dev/console", os.O_RDWR, 0)
	if err != nil {
		return err
	}
	fd := int(f.Fd())
	if err := unix.Dup2(fd, 0); err != nil {
		_ = f.Close()
		return err
	}
	if err := unix.Dup2(fd, 1); err != nil {
		_ = f.Close()
		return err
	}
	if err := unix.Dup2(fd, 2); err != nil {
		_ = f.Close()
		return err
	}
	// Do not close f; the duplicated fds keep it open.
	return nil
}

// statfsCheck returns the filesystem type name for the given path.
func statfsCheck(path string) (string, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return "", err
	}
	return fsTypeName(stat.Type), nil
}

// fsTypeName returns a human-readable name for a filesystem magic number.
func fsTypeName(magic int64) string {
	// Common filesystem type magic numbers from statfs(2).
	switch magic {
	case 0x01021994:
		return "tmpfs"
	case 0xEF53:
		return "ext2/ext3/ext4"
	case 0x58465342:
		return "xfs"
	case 0x9123683E:
		return "btrfs"
	default:
		return "unknown"
	}
}

// readVersionID reads VERSION_ID from /usr/lib/os-release.
func readVersionID() string {
	data, err := os.ReadFile("/usr/lib/os-release")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "VERSION_ID=") {
			return strings.Trim(strings.TrimPrefix(line, "VERSION_ID="), "\"")
		}
	}
	return ""
}

// cryptoRandUint32 generates a random uint32 using crypto/rand.
func cryptoRandUint32() (uint32, error) {
	var buf [4]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint32(buf[:]), nil
}

// realProcessExecutor implements modes.ProcessExecutor using real syscalls.
type realProcessExecutor struct{}

func (r *realProcessExecutor) PivotRoot(newRoot, putOld string) error {
	return syscall.PivotRoot(newRoot, putOld)
}

func (r *realProcessExecutor) Exec(path string, args []string, env []string) error {
	return syscall.Exec(path, args, env)
}

// findCommand searches the command's sub-commands for a match by name or alias.
func findCommand(cmd *cli.Command, name string) *cli.Command {
	for _, c := range cmd.Commands {
		if c.Name == name {
			return c
		}
		for _, a := range c.Aliases {
			if a == name {
				return c
			}
		}
	}
	return nil
}
