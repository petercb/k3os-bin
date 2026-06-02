// Package cc implements cloud-config applier orchestration for k3os boot phases.
package cc

import (
	"errors"

	"github.com/petercb/k3os-bin/internal/config"
	"github.com/petercb/k3os-bin/internal/iface"
	"github.com/petercb/k3os-bin/internal/iface/osimpl"
	"github.com/petercb/k3os-bin/internal/shadow"
)

type applier func(cfg *config.CloudConfig) error

// Applier holds the dependencies needed by cloud-config applier functions.
type Applier struct {
	FS         iface.FileSystem
	Cmd        iface.CommandRunner
	Modules    iface.ModuleLoader
	Sysctl     iface.SysctlApplier
	Mounter    iface.Mounter
	Hostname   iface.HostnameSetter
	Password   shadow.PasswordSetter
	modePrefix []string // injected in tests; nil preserves production default
}

// NewDefaultApplier creates an Applier with production OS implementations.
func NewDefaultApplier() *Applier {
	return &Applier{
		FS:       osimpl.OSFileSystem{},
		Cmd:      osimpl.ShellRunner{},
		Modules:  osimpl.LinuxModuleLoader{},
		Sysctl:   osimpl.LinuxSysctlApplier{},
		Mounter:  osimpl.LinuxMounter{},
		Hostname: osimpl.LinuxHostnameSetter{},
		Password: shadow.Setter{},
	}
}

func (a *Applier) runApplies(cfg *config.CloudConfig, appliers ...applier) error {
	var errs []error

	for _, app := range appliers {
		err := app(cfg)
		if err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

// RunApply runs the normal cloud-config apply sequence.
func (a *Applier) RunApply(cfg *config.CloudConfig) error {
	return a.runApplies(cfg,
		a.ApplyModules,
		a.ApplySysctls,
		a.ApplyHostname,
		a.ApplyDNS,
		a.ApplyWifi,
		a.ApplyPassword,
		a.ApplySSHKeysWithNet,
		a.ApplyWriteFiles,
		a.ApplyEnvironment,
		a.ApplyRuncmd,
		a.ApplyInstall,
		a.ApplyK3SInstall,
	)
}

// InstallApply runs the install-phase cloud-config apply sequence.
func (a *Applier) InstallApply(cfg *config.CloudConfig) error {
	return a.runApplies(cfg,
		a.ApplyK3SWithRestart,
	)
}

// BootApply runs the boot-phase cloud-config apply sequence.
func (a *Applier) BootApply(cfg *config.CloudConfig) error {
	return a.runApplies(cfg,
		a.ApplyDataSource,
		a.ApplyModules,
		a.ApplySysctls,
		a.ApplyHostname,
		a.ApplyDNS,
		a.ApplyWifi,
		a.ApplyPassword,
		a.ApplySSHKeys,
		a.ApplyK3SNoRestart,
		a.ApplyWriteFiles,
		a.ApplyEnvironment,
		a.ApplyBootcmd,
	)
}

// InitApply runs the initrd-phase cloud-config apply sequence.
func (a *Applier) InitApply(cfg *config.CloudConfig) error {
	return a.runApplies(cfg,
		a.ApplyModules,
		a.ApplySysctls,
		a.ApplyHostname,
		a.ApplyWriteFiles,
		a.ApplyEnvironment,
		a.ApplyInitcmd,
	)
}
