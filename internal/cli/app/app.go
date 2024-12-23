package app

import (
	"fmt"

	"github.com/petercb/k3os-bin/internal/cli/config"
	"github.com/petercb/k3os-bin/internal/cli/install"
	"github.com/petercb/k3os-bin/internal/cli/rc"
	"github.com/petercb/k3os-bin/internal/cli/upgrade"
	"github.com/petercb/k3os-bin/internal/version"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var (
	Debug bool
)

// New CLI App
func New() *cli.App {
	app := cli.NewApp()
	app.Name = "k3os"
	app.Usage = "Booting to k3s so you don't have to"
	app.Version = version.Version
	cli.VersionPrinter = func(_ *cli.Context) {
		fmt.Printf("%s CLI version %s\n", app.Name, app.Version)
	}
	// required flags without defaults will break symlinking to exe with name of sub-command as target
	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:        "debug",
			Usage:       "Turn on debug logs",
			EnvVar:      "K3OS_DEBUG",
			Destination: &Debug,
		},
	}

	app.Commands = []cli.Command{
		rc.Command(),
		config.Command(),
		install.Command(),
		upgrade.Command(),
	}

	app.Before = func(_ *cli.Context) error {
		if Debug {
			logrus.SetLevel(logrus.DebugLevel)
		}
		return nil
	}

	return app
}
