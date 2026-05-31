// Package config implements the k3OS config sub-command.
//
// NOTE: As of the Go-based init implementation, the boot sequence calls
// RunInitrd() and RunBoot() directly in-process. The --initrd and --boot
// CLI flags are deprecated and retained only for backward compatibility.
// The default action (no flags) is used by the ccapply OpenRC service and
// for diagnostic purposes.
package config

import (
	"context"
	"encoding/json"
	"errors"
	"os"

	"github.com/petercb/k3os-bin/internal/cc"
	"github.com/petercb/k3os-bin/internal/config"
	cli "github.com/urfave/cli/v3"
)

var (
	initrd       = false
	bootPhase    = false
	installPhase = false
	dump         = false
	dumpJSON     = false
)

// Command returns the CLI command for k3OS configuration management.
// The default action (no flags) applies the full cloud-config sequence;
// this is used by the ccapply OpenRC service at the "default" runlevel.
// The --dump and --dump-json flags are useful for diagnostics.
func Command() *cli.Command {
	return &cli.Command{
		Name:    "config",
		Usage:   "apply or inspect k3OS cloud-config",
		Aliases: []string{"cfg"},
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:        "initrd",
				Destination: &initrd,
				Usage:       "[deprecated] Run initrd stage (now handled internally during boot)",
			},
			&cli.BoolFlag{
				Name:        "boot",
				Destination: &bootPhase,
				Usage:       "[deprecated] Run boot stage (now handled internally during boot)",
			},
			&cli.BoolFlag{
				Name:        "install",
				Destination: &installPhase,
				Usage:       "Run install stage",
			},
			&cli.BoolFlag{
				Name:        "dump",
				Destination: &dump,
				Usage:       "Print current configuration",
			},
			&cli.BoolFlag{
				Name:        "dump-json",
				Destination: &dumpJSON,
				Usage:       "Print current configuration in json",
			},
		},
		Before: func(_ context.Context, _ *cli.Command) (context.Context, error) {
			if os.Getuid() != 0 {
				return nil, errors.New("must be run as root")
			}
			return nil, nil
		},
		Action: func(_ context.Context, _ *cli.Command) error {
			return Main()
		},
	}
}

// RunInitrd performs the initrd-phase config application.
func RunInitrd() error {
	cfg, err := config.ReadConfig()
	if err != nil {
		return err
	}
	return cc.NewDefaultApplier().InitApply(&cfg)
}

// RunBoot performs the boot-phase config application.
func RunBoot() error {
	cfg, err := config.ReadConfig()
	if err != nil {
		return err
	}
	return cc.NewDefaultApplier().BootApply(&cfg)
}

// Main `config`
func Main() error {
	//nolint:gocritic
	if initrd {
		return RunInitrd()
	} else if bootPhase {
		return RunBoot()
	} else if installPhase {
		cfg, err := config.ReadConfig()
		if err != nil {
			return err
		}
		return cc.NewDefaultApplier().InstallApply(&cfg)
	} else if dump {
		cfg, err := config.ReadConfig()
		if err != nil {
			return err
		}
		return config.Write(cfg, os.Stdout)
	} else if dumpJSON {
		cfg, err := config.ReadConfig()
		if err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(&cfg)
	}

	cfg, err := config.ReadConfig()
	if err != nil {
		return err
	}
	return cc.NewDefaultApplier().RunApply(&cfg)
}
