package install

import (
	"errors"
	"os"

	"github.com/petercb/k3os-bin/pkg/cliinstall"
	"github.com/petercb/k3os-bin/pkg/mode"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

func Command() cli.Command {
	mode, _ := mode.Get()
	return cli.Command{
		Name:  "install",
		Usage: "install k3OS",
		Flags: []cli.Flag{},
		Before: func(_ *cli.Context) error {
			if os.Getuid() != 0 {
				return errors.New("must be run as root")
			}
			return nil
		},
		Action: func(*cli.Context) {
			if err := cliinstall.Run(); err != nil {
				logrus.Error(err)
			}
		},
		Hidden: mode == "local",
	}
}
