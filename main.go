// Package main is the entry point for the k3OS binary.
package main

// Copyright 2019 Rancher Labs, Inc.
// SPDX-License-Identifier: Apache-2.0

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/moby/sys/reexec"
	"github.com/petercb/k3os-bin/internal/boot"
	"github.com/petercb/k3os-bin/internal/boot/bootstrap"
	"github.com/petercb/k3os-bin/internal/boot/finalize"
	"github.com/petercb/k3os-bin/internal/boot/modes"
	"github.com/petercb/k3os-bin/internal/cli/app"
	"github.com/petercb/k3os-bin/internal/enterchroot"
	"github.com/petercb/k3os-bin/internal/iface/osimpl"
	"github.com/petercb/k3os-bin/internal/kernel"
	"github.com/petercb/k3os-bin/internal/mode"
	"github.com/petercb/k3os-bin/internal/mount"
	"github.com/petercb/k3os-bin/internal/transferroot"
	cli "github.com/urfave/cli/v3"
)

func main() {
	reexec.Register("init", initrd) // covers both /init (live) and /sbin/init (local)
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
		FS:            fs,
		Cmd:           cmd,
		Mounter:       mounter,
		Proc:          &realProcessExecutor{},
		KernelVersion: kver,
		VersionID:     versionID,
	}
	registry := modes.NewRegistry(modeDeps)

	// Build the bootstrapper.
	bs := &bootstrap.Bootstrapper{
		FS:            fs,
		Mounter:       mounter,
		Cmd:           cmd,
		KernelVersion: kver,
	}

	// Build the finalizer.
	fin := &finalize.Finalizer{
		FS:      fs,
		Mounter: mounter,
		Cmd:     cmd,
	}

	// Build the init orchestrator.
	initOrch := &boot.Init{
		Bootstrap: bs,
		ModeDetector: func() (string, error) {
			detector := &mode.Detector{
				CmdlineReader: readCmdline,
				BlockProber:   blockProbe,
				StatfsChecker: statfsCheck,
				EnvReader:     os.Getenv,
				FileWriter:    os.WriteFile,
				MkdirAll:      os.MkdirAll,
				SleepFunc:     time.Sleep,
				Timeout:       30 * time.Second,
				SleepInterval: 1 * time.Second,
			}
			m, detectErr := detector.Detect(context.Background())
			if detectErr != nil {
				return "", detectErr
			}
			// Update finalizer mode after detection.
			fin.Mode = m
			return m, nil
		},
		ModeRegistry: func(m string) (boot.ModeHandler, error) {
			return registry.Get(m)
		},
		Finalizer:     fin,
		CmdlineReader: readCmdline,
		ExecFunc:      syscall.Exec,
		RescueFunc: func() error {
			return syscall.Exec("/bin/bash", []string{"bash"}, os.Environ())
		},
	}

	initOrch.Run()
	// Should not reach here; exec replaces the process.
	os.Exit(1)
}

// readCmdline reads the kernel command line from /proc/cmdline.
func readCmdline() (string, error) {
	data, err := os.ReadFile("/proc/cmdline")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// blockProbe checks for a block device with the given label using blkid.
func blockProbe(label string) (string, error) {
	runner := osimpl.ShellRunner{}
	return runner.RunOutput("blkid", "-L", label)
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
