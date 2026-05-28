// Package main is the entry point for the k3OS binary.
package main

// Copyright 2019 Rancher Labs, Inc.
// SPDX-License-Identifier: Apache-2.0

import (
	"context"
	"os"
	"path/filepath"

	"github.com/moby/sys/reexec"
	"github.com/petercb/k3os-bin/internal/cli/app"
	"github.com/petercb/k3os-bin/internal/enterchroot"
	"github.com/petercb/k3os-bin/internal/mount"
	"github.com/petercb/k3os-bin/internal/transferroot"
	"github.com/sirupsen/logrus"
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
			logrus.Fatal(err)
		}
	}
}

func initrd() {
	enterchroot.DebugCmdline = "k3os.debug"
	transferroot.Relocate()
	if err := mount.Mount("", "/", "none", "rw,remount"); err != nil {
		logrus.Errorf("failed to remount root as rw: %v", err)
	}
	if err := enterchroot.Mount("./k3os/data", os.Args, os.Stdout, os.Stderr); err != nil {
		logrus.Fatalf("failed to enter root: %v", err)
	}
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
