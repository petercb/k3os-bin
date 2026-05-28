// Package config implements the k3OS config sub-command.
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

// Command `config`
func Command() *cli.Command {
	return &cli.Command{
		Name:    "config",
		Usage:   "configure k3OS",
		Aliases: []string{"cfg"},
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:        "initrd",
				Destination: &initrd,
				Usage:       "Run initrd stage",
			},
			&cli.BoolFlag{
				Name:        "boot",
				Destination: &bootPhase,
				Usage:       "Run boot stage",
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

// Main `config`
func Main() error {
	cfg, err := config.ReadConfig()
	if err != nil {
		return err
	}

	applier := cc.NewDefaultApplier()

	//nolint:gocritic
	if initrd {
		return applier.InitApply(&cfg)
	} else if bootPhase {
		return applier.BootApply(&cfg)
	} else if installPhase {
		return applier.InstallApply(&cfg)
	} else if dump {
		return config.Write(cfg, os.Stdout)
	} else if dumpJSON {
		return json.NewEncoder(os.Stdout).Encode(&cfg)
	}

	return applier.RunApply(&cfg)
}
