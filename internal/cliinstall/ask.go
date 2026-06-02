// Package cliinstall implements the interactive k3OS installation workflow.
package cliinstall

import (
	"strings"

	"github.com/petercb/k3os-bin/internal/config"
	"github.com/petercb/k3os-bin/internal/iface"
	"github.com/petercb/k3os-bin/internal/iface/osimpl"
	"github.com/petercb/k3os-bin/internal/mode"
	"github.com/petercb/k3os-bin/internal/questions"
	"github.com/petercb/k3os-bin/internal/shadow"
	"github.com/petercb/k3os-bin/internal/util"
)

// blockProber is the BlockProber used to enumerate disk devices.
// Override in tests.
var blockProber iface.BlockProber = osimpl.SysfsBlockProber{}

// Ask prompts the user to choose between install and server/agent configuration.
func Ask(cfg *config.CloudConfig) (bool, error) {
	if ok, err := isInstall(cfg); err != nil {
		return false, err
	} else if ok {
		return true, AskInstall(cfg)
	}

	return false, AskServerAgent(cfg)
}

func isInstall(_ *config.CloudConfig) (bool, error) {
	mode, err := mode.Get()
	if err != nil {
		return false, err
	}

	switch mode {
	case "install":
		return true, nil
	case "live-server":
		return false, nil
	case "live-agent":
		return false, nil
	}

	i, err := questions.PromptFormattedOptions("Choose operation", 0,
		"Install to disk",
		"Configure server or agent")
	if err != nil {
		return false, err
	}

	return i == 0, nil
}

// AskInstall prompts the user for all installation configuration options.
func AskInstall(cfg *config.CloudConfig) error {
	if cfg.K3OS.Install.Silent {
		return nil
	}

	if err := AskInstallDevice(cfg); err != nil {
		return err
	}

	if err := AskConfigURL(cfg); err != nil {
		return err
	}

	if cfg.K3OS.Install.ConfigURL == "" {
		if err := AskGithub(cfg); err != nil {
			return err
		}

		if err := AskPassword(cfg); err != nil {
			return err
		}

		if err := AskWifi(cfg); err != nil {
			return err
		}

		if err := AskServerAgent(cfg); err != nil {
			return err
		}
	}

	return nil
}

// AskInstallDevice prompts the user to select a target disk device for installation.
func AskInstallDevice(cfg *config.CloudConfig) error {
	if cfg.K3OS.Install.Device != "" {
		return nil
	}

	fields, err := blockProber.ListDisks()
	if err != nil {
		return err
	}
	i, err := questions.PromptFormattedOptions("Installation target. Device will be formatted", -1, fields...)
	if err != nil {
		return err
	}

	cfg.K3OS.Install.Device = "/dev/" + fields[i]
	return nil
}

// AskToken prompts the user for a cluster token or secret.
func AskToken(cfg *config.CloudConfig, server bool) error {
	var (
		token string
		err   error
	)

	if cfg.K3OS.Token != "" {
		return nil
	}

	msg := "Token or cluster secret"
	if server {
		msg += " (optional)"
	}
	if server {
		token, err = questions.PromptOptional(msg+": ", "")
	} else {
		token, err = questions.Prompt(msg+": ", "")
	}
	cfg.K3OS.Token = token

	return err
}

func isServer(cfg *config.CloudConfig) (bool, error) {
	mode, err := mode.Get()
	if err != nil {
		return false, err
	}
	if mode == "live-server" {
		return true, nil
	} else if mode == "live-agent" || (cfg.K3OS.ServerURL != "" && cfg.K3OS.Token != "") {
		return false, nil
	}

	opts := []string{"server", "agent"}
	i, err := questions.PromptFormattedOptions("Run as server or agent?", 0, opts...)
	if err != nil {
		return false, err
	}

	return i == 0, nil
}

// AskServerAgent prompts the user to configure k3s as a server or agent.
func AskServerAgent(cfg *config.CloudConfig) error {
	if cfg.K3OS.ServerURL != "" {
		return nil
	}

	server, err := isServer(cfg)
	if err != nil {
		return err
	}

	if server {
		return AskToken(cfg, true)
	}

	url, err := questions.Prompt("URL of server: ", "")
	if err != nil {
		return err
	}
	cfg.K3OS.ServerURL = url

	return AskToken(cfg, false)
}

// AskPassword prompts the user to set a password for the rancher user.
//
// NOTE: Unlike the previous chpasswd-based implementation, this function always
// stores a SHA-512 hash in cfg.K3OS.Password regardless of whether the caller
// is running as root. The old behavior stored plaintext when non-root, which was
// a security concern. Downstream consumers (e.g., SetPassword) correctly handle
// pre-hashed values (those starting with '$') so this is safe.
func AskPassword(cfg *config.CloudConfig) error {
	if len(cfg.SSHAuthorizedKeys) > 0 || cfg.K3OS.Password != "" {
		return nil
	}

	var (
		ok   = false
		err  error
		pass string
	)

	for !ok {
		pass, ok, err = util.PromptPassword()
		if err != nil {
			return err
		}
	}

	hash, err := shadow.HashPassword(pass)
	if err != nil {
		return err
	}

	cfg.K3OS.Password = hash
	return nil
}

// AskWifi prompts the user to configure WiFi network settings.
func AskWifi(cfg *config.CloudConfig) error {
	if len(cfg.K3OS.Wifi) > 0 {
		return nil
	}

	ok, err := questions.PromptBool("Configure WiFi?", false)
	if !ok || err != nil {
		return err
	}

	for {
		name, err := questions.Prompt("WiFi Name: ", "")
		if err != nil {
			return err
		}

		pass, err := questions.Prompt("WiFi Passphrase: ", "")
		if err != nil {
			return err
		}

		cfg.K3OS.Wifi = append(cfg.K3OS.Wifi, config.Wifi{
			Name:       name,
			Passphrase: pass,
		})

		ok, err := questions.PromptBool("Configure another WiFi network?", false)
		if !ok || err != nil {
			return err
		}
	}
}

// AskGithub prompts the user to authorize GitHub users for SSH access.
func AskGithub(cfg *config.CloudConfig) error {
	if len(cfg.SSHAuthorizedKeys) > 0 || cfg.K3OS.Password != "" {
		return nil
	}

	ok, err := questions.PromptBool("Authorize GitHub users to SSH?", false)
	if !ok || err != nil {
		return err
	}

	str, err := questions.Prompt("Comma separated list of GitHub users to authorize: ", "")
	if err != nil {
		return err
	}

	for _, s := range strings.Split(str, ",") {
		cfg.SSHAuthorizedKeys = append(cfg.SSHAuthorizedKeys, "github:"+strings.TrimSpace(s))
	}

	return nil
}

// AskConfigURL prompts the user for a cloud-init configuration URL.
func AskConfigURL(cfg *config.CloudConfig) error {
	if cfg.K3OS.Install.ConfigURL != "" {
		return nil
	}

	ok, err := questions.PromptBool("Config system with cloud-init file?", false)
	if err != nil {
		return err
	}

	if !ok {
		return nil
	}

	str, err := questions.Prompt("cloud-init file location (file path or http URL): ", "")
	if err != nil {
		return err
	}

	cfg.K3OS.Install.ConfigURL = str
	return nil
}
