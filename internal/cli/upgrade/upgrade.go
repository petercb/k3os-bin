// Package upgrade implements the k3OS upgrade sub-command.
package upgrade

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/petercb/k3os-bin/internal/system"
	"github.com/sirupsen/logrus"
	cli "github.com/urfave/cli/v3"
	"golang.org/x/sys/unix"
)

// upgradeOpts holds all flag destinations for the upgrade command.
type upgradeOpts struct {
	upgradeK3OS    bool
	upgradeK3S     bool
	upgradeKernel  bool
	upgradeRootFS  bool
	doRemount      bool
	doSync         bool
	doReboot       bool
	sourceDir      string
	destinationDir string
	lockFile       string
}

// Command is the `upgrade` sub-command, it performs upgrades to k3OS.
func Command() *cli.Command {
	opts := &upgradeOpts{}

	return &cli.Command{
		Name:  "upgrade",
		Usage: "perform upgrades",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:        "k3os",
				Sources:     cli.EnvVars("K3OS_UPGRADE_K3OS"),
				Destination: &opts.upgradeK3OS,
				Hidden:      true,
			},
			&cli.BoolFlag{
				Name:        "k3s",
				Sources:     cli.EnvVars("K3OS_UPGRADE_K3S"),
				Destination: &opts.upgradeK3S,
				Hidden:      true,
			},
			&cli.BoolFlag{
				Name:        "kernel",
				Usage:       "upgrade the kernel",
				Sources:     cli.EnvVars("K3OS_UPGRADE_KERNEL"),
				Destination: &opts.upgradeKernel,
			},
			&cli.BoolFlag{
				Name:        "rootfs",
				Usage:       "upgrade k3os+k3s",
				Sources:     cli.EnvVars("K3OS_UPGRADE_ROOTFS"),
				Destination: &opts.upgradeRootFS,
			},
			&cli.BoolFlag{
				Name:        "remount",
				Usage:       "pre-upgrade remount?",
				Sources:     cli.EnvVars("K3OS_UPGRADE_REMOUNT"),
				Destination: &opts.doRemount,
			},
			&cli.BoolFlag{
				Name:        "sync",
				Usage:       "post-upgrade sync?",
				Sources:     cli.EnvVars("K3OS_UPGRADE_SYNC"),
				Destination: &opts.doSync,
			},
			&cli.BoolFlag{
				Name:        "reboot",
				Usage:       "post-upgrade reboot?",
				Sources:     cli.EnvVars("K3OS_UPGRADE_REBOOT"),
				Destination: &opts.doReboot,
			},
			&cli.StringFlag{
				Name:        "source",
				Sources:     cli.EnvVars("K3OS_UPGRADE_SOURCE"),
				Value:       system.RootPath(),
				Required:    true,
				Destination: &opts.sourceDir,
			},
			&cli.StringFlag{
				Name:        "destination",
				Sources:     cli.EnvVars("K3OS_UPGRADE_DESTINATION"),
				Value:       system.RootPath(),
				Required:    true,
				Destination: &opts.destinationDir,
			},
			&cli.StringFlag{
				Name:        "lock-file",
				Sources:     cli.EnvVars("K3OS_UPGRADE_LOCK_FILE"),
				Value:       system.StatePath("upgrade.lock"),
				Hidden:      true,
				Destination: &opts.lockFile,
			},
		},
		Before: func(_ context.Context, cmd *cli.Command) (context.Context, error) {
			if opts.destinationDir == opts.sourceDir {
				_ = cli.ShowSubcommandHelp(cmd)
				return nil, fmt.Errorf("the `destination` cannot be the `source`: %s", opts.destinationDir)
			}
			if opts.upgradeRootFS {
				opts.upgradeK3S = true
				opts.upgradeK3OS = true
			}
			if !opts.upgradeK3OS && !opts.upgradeK3S && !opts.upgradeKernel {
				_ = cli.ShowSubcommandHelp(cmd)
				return nil, fmt.Errorf("must specify components to upgrade, e.g. `rootfs`, `kernel`")
			}
			return nil, nil
		},
		Action: func(_ context.Context, _ *cli.Command) error {
			return Run(opts)
		},
	}
}

// Run the `upgrade` sub-command
//
//nolint:gocognit
func Run(opts *upgradeOpts) error {
	if err := validateSystemRoot(opts.sourceDir); err != nil {
		return err
	}
	if err := validateSystemRoot(opts.destinationDir); err != nil {
		return err
	}

	// establish the lock
	lf, err := os.OpenFile(opts.lockFile, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer func() { _ = lf.Close() }()
	if err = unix.Flock(int(lf.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		return err
	}
	defer func() {
		if unlockerr := unix.Flock(int(lf.Fd()), unix.LOCK_UN); unlockerr != nil {
			logrus.Error(unlockerr)
		}
	}()

	var atLeastOneComponentCopied bool

	if opts.upgradeK3OS {
		if copied, err := system.CopyComponent(opts.sourceDir, opts.destinationDir, opts.doRemount, "k3os"); err != nil {
			logrus.Error(err)
		} else if copied {
			atLeastOneComponentCopied = true
			opts.doRemount = false
		}
	}
	if opts.upgradeK3S {
		if copied, err := system.CopyComponent(opts.sourceDir, opts.destinationDir, opts.doRemount, "k3s"); err != nil {
			logrus.Error(err)
		} else if copied {
			atLeastOneComponentCopied = true
			opts.doRemount = false
		}
	}
	if opts.upgradeKernel {
		if copied, err := system.CopyComponent(opts.sourceDir, opts.destinationDir, opts.doRemount, "kernel"); err != nil {
			logrus.Error(err)
		} else if copied {
			atLeastOneComponentCopied = true
			opts.doRemount = false
		}
	}

	if atLeastOneComponentCopied && opts.doSync {
		unix.Sync()
	}

	if atLeastOneComponentCopied && opts.doReboot {
		// nsenter -m -u -i -n -p -t 1 -- reboot
		if _, err := exec.LookPath("nsenter"); err != nil {
			logrus.Warn(err)
			if opts.destinationDir != system.RootPath() {
				root := filepath.Clean(filepath.Join(opts.destinationDir, "..", ".."))
				logrus.Debugf("attempting chroot: %v", root)
				if err := unix.Chroot(root); err != nil {
					return err
				}
				if err := os.Chdir("/"); err != nil {
					return err
				}
			}
		}
		cmd := exec.Command("nsenter", "-m", "-u", "-i", "-n", "-p", "-t", "1", "reboot")
		cmd.Stderr = os.Stderr
		cmd.Stdout = os.Stdout
		if err := cmd.Run(); err != nil {
			return err
		}
	}

	return nil
}

func validateSystemRoot(root string) error {
	info, err := os.Stat(root)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("stat %s: not a directory", root)
	}
	return nil
}
