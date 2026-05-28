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

var (
	upgradeK3OS, upgradeK3S             bool
	upgradeKernel, upgradeRootFS        bool
	doRemount, doSync, doReboot         bool
	sourceDir, destinationDir, lockFile string
)

// Command is the `upgrade` sub-command, it performs upgrades to k3OS.
func Command() *cli.Command {
	return &cli.Command{
		Name:  "upgrade",
		Usage: "perform upgrades",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:        "k3os",
				Sources:     cli.EnvVars("K3OS_UPGRADE_K3OS"),
				Destination: &upgradeK3OS,
				Hidden:      true,
			},
			&cli.BoolFlag{
				Name:        "k3s",
				Sources:     cli.EnvVars("K3OS_UPGRADE_K3S"),
				Destination: &upgradeK3S,
				Hidden:      true,
			},
			&cli.BoolFlag{
				Name:        "kernel",
				Usage:       "upgrade the kernel",
				Sources:     cli.EnvVars("K3OS_UPGRADE_KERNEL"),
				Destination: &upgradeKernel,
			},
			&cli.BoolFlag{
				Name:        "rootfs",
				Usage:       "upgrade k3os+k3s",
				Sources:     cli.EnvVars("K3OS_UPGRADE_ROOTFS"),
				Destination: &upgradeRootFS,
			},
			&cli.BoolFlag{
				Name:        "remount",
				Usage:       "pre-upgrade remount?",
				Sources:     cli.EnvVars("K3OS_UPGRADE_REMOUNT"),
				Destination: &doRemount,
			},
			&cli.BoolFlag{
				Name:        "sync",
				Usage:       "post-upgrade sync?",
				Sources:     cli.EnvVars("K3OS_UPGRADE_SYNC"),
				Destination: &doSync,
			},
			&cli.BoolFlag{
				Name:        "reboot",
				Usage:       "post-upgrade reboot?",
				Sources:     cli.EnvVars("K3OS_UPGRADE_REBOOT"),
				Destination: &doReboot,
			},
			&cli.StringFlag{
				Name:        "source",
				Sources:     cli.EnvVars("K3OS_UPGRADE_SOURCE"),
				Value:       system.RootPath(),
				Required:    true,
				Destination: &sourceDir,
			},
			&cli.StringFlag{
				Name:        "destination",
				Sources:     cli.EnvVars("K3OS_UPGRADE_DESTINATION"),
				Value:       system.RootPath(),
				Required:    true,
				Destination: &destinationDir,
			},
			&cli.StringFlag{
				Name:        "lock-file",
				Sources:     cli.EnvVars("K3OS_UPGRADE_LOCK_FILE"),
				Value:       system.StatePath("upgrade.lock"),
				Hidden:      true,
				Destination: &lockFile,
			},
		},
		Before: func(_ context.Context, cmd *cli.Command) (context.Context, error) {
			if destinationDir == sourceDir {
				_ = cli.ShowSubcommandHelp(cmd)
				logrus.Errorf("the `destination` cannot be the `source`: %s", destinationDir)
				os.Exit(1)
			}
			if upgradeRootFS {
				upgradeK3S = true
				upgradeK3OS = true
			}
			if !upgradeK3OS && !upgradeK3S && !upgradeKernel {
				_ = cli.ShowSubcommandHelp(cmd)
				logrus.Error("must specify components to upgrade, e.g. `rootfs`, `kernel`")
				os.Exit(1)
			}
			return nil, nil
		},
		Action: func(_ context.Context, _ *cli.Command) error {
			Run()
			return nil
		},
	}
}

// Run the `upgrade` sub-command
//
//nolint:gocognit
func Run() {
	if err := validateSystemRoot(sourceDir); err != nil {
		logrus.Fatal(err)
	}
	if err := validateSystemRoot(destinationDir); err != nil {
		logrus.Fatal(err)
	}

	// establish the lock
	lf, err := os.OpenFile(lockFile, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		logrus.Fatal(err)
	}
	defer func() { _ = lf.Close() }()
	if err = unix.Flock(int(lf.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		logrus.Fatal(err)
	}
	defer func() {
		if unlockerr := unix.Flock(int(lf.Fd()), unix.LOCK_UN); unlockerr != nil {
			logrus.Error(unlockerr)
		}
	}()

	var atLeastOneComponentCopied bool

	if upgradeK3OS {
		if copied, err := system.CopyComponent(sourceDir, destinationDir, doRemount, "k3os"); err != nil {
			logrus.Error(err)
		} else if copied {
			atLeastOneComponentCopied = true
			doRemount = false
		}
	}
	if upgradeK3S {
		if copied, err := system.CopyComponent(sourceDir, destinationDir, doRemount, "k3s"); err != nil {
			logrus.Error(err)
		} else if copied {
			atLeastOneComponentCopied = true
			doRemount = false
		}
	}
	if upgradeKernel {
		if copied, err := system.CopyComponent(sourceDir, destinationDir, doRemount, "kernel"); err != nil {
			logrus.Error(err)
		} else if copied {
			atLeastOneComponentCopied = true
			doRemount = false
		}
	}

	if atLeastOneComponentCopied && doSync {
		unix.Sync()
	}

	if atLeastOneComponentCopied && doReboot {
		// nsenter -m -u -i -n -p -t 1 -- reboot
		if _, err := exec.LookPath("nsenter"); err != nil {
			logrus.Warn(err)
			if destinationDir != system.RootPath() {
				root := filepath.Clean(filepath.Join(destinationDir, "..", ".."))
				logrus.Debugf("attempting chroot: %v", root)
				if err := unix.Chroot(root); err != nil {
					logrus.Fatal(err)
				}
				if err := os.Chdir("/"); err != nil {
					logrus.Fatal(err)
				}
			}
		}
		cmd := exec.Command("nsenter", "-m", "-u", "-i", "-n", "-p", "-t", "1", "reboot")
		cmd.Stderr = os.Stderr
		cmd.Stdout = os.Stdout
		if err := cmd.Run(); err != nil {
			logrus.Fatal(err)
		}
	}
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
