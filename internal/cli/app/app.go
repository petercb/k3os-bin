// Package app provides the top-level CLI application for k3OS.
package app

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/petercb/k3os-bin/internal/cli/config"
	"github.com/petercb/k3os-bin/internal/cli/install"
	"github.com/petercb/k3os-bin/internal/cli/rc"
	"github.com/petercb/k3os-bin/internal/cli/upgrade"
	"github.com/petercb/k3os-bin/internal/version"
	cli "github.com/urfave/cli/v3"
)

// Debug enables debug-level logging when set to true.
var Debug bool

// New CLI App
func New() *cli.Command {
	cmd := &cli.Command{
		Name:    "k3os",
		Usage:   "Booting to k3s so you don't have to",
		Version: version.Version,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:        "debug",
				Usage:       "Turn on debug logs",
				Sources:     cli.EnvVars("K3OS_DEBUG"),
				Destination: &Debug,
			},
		},
		Commands: []*cli.Command{
			rc.Command(), //nolint:staticcheck // retained for backward compatibility
			config.Command(),
			install.Command(),
			upgrade.Command(),
		},
		Before: func(_ context.Context, _ *cli.Command) (context.Context, error) {
			if Debug {
				slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
					Level: slog.LevelDebug,
				})))
			}
			return nil, nil
		},
	}

	cli.VersionPrinter = func(c *cli.Command) {
		fmt.Printf("%s CLI version %s\n", c.Root().Name, c.Root().Version)
	}

	return cmd
}
