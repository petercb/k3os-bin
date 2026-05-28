// Package install implements the k3OS install sub-command.
package install

import (
	"context"
	"errors"
	"os"

	"github.com/petercb/k3os-bin/internal/cliinstall"
	"github.com/petercb/k3os-bin/internal/mode"
	cli "github.com/urfave/cli/v3"
)

// Command returns the CLI command for installing k3OS to disk.
func Command() *cli.Command {
	m, _ := mode.Get()
	return &cli.Command{
		Name:  "install",
		Usage: "install k3OS",
		Flags: []cli.Flag{},
		Before: func(_ context.Context, _ *cli.Command) (context.Context, error) {
			if os.Getuid() != 0 {
				return nil, errors.New("must be run as root")
			}
			return nil, nil
		},
		Action: func(_ context.Context, _ *cli.Command) error {
			return cliinstall.Run()
		},
		Hidden: m == "local",
	}
}
