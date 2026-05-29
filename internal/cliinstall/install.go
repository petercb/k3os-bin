package cliinstall

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/petercb/k3os-bin/internal/config"
	"github.com/petercb/k3os-bin/internal/questions"
	"gopkg.in/yaml.v3"
)

// Run executes the interactive k3OS installation and configuration workflow.
func Run() error {
	fmt.Println("\nRunning k3OS configuration")

	cfg, err := config.ReadConfig()
	if err != nil {
		return err
	}

	isInstall, err := Ask(&cfg)
	if err != nil {
		return err
	}

	if isInstall {
		return runInstall(cfg)
	}

	bytes, err := config.ToBytes(cfg)
	if err != nil {
		return err
	}

	f, err := os.Create(config.SystemConfig)
	if err != nil {
		f, err = os.Create(config.LocalConfig)
		if err != nil {
			return err
		}
	}
	defer func() { _ = f.Close() }()

	if _, err := f.Write(bytes); err != nil {
		return err
	}

	if err := f.Close(); err != nil {
		return err
	}
	return runCCApply()
}

func runCCApply() error {
	cmd := exec.Command(os.Args[0], "config", "--install")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func runInstall(cfg config.CloudConfig) error {
	var (
		err      error
		tempFile *os.File
	)

	installBytes, err := config.PrintInstall(cfg)
	if err != nil {
		return err
	}

	if !cfg.K3OS.Install.Silent {
		var val bool
		val, err = questions.PromptBool("\nConfiguration\n"+"-------------\n\n"+
			string(installBytes)+
			"\nYour disk will be formatted and k3OS will be installed with the above configuration.\nContinue?", false)
		if err != nil || !val {
			return err
		}
	}

	if cfg.K3OS.Install.ConfigURL == "" {
		tempFile, err = os.CreateTemp("/tmp", "k3os.XXXXXXXX")
		if err != nil {
			return err
		}
		defer func() { _ = tempFile.Close() }()

		cfg.K3OS.Install.ConfigURL = tempFile.Name()
	}

	ev, err := config.ToEnv(cfg)
	if err != nil {
		return err
	}

	if tempFile != nil {
		cfg.K3OS.Install = nil
		bytes, err := yaml.Marshal(&cfg)
		if err != nil {
			return err
		}
		if _, err := tempFile.Write(bytes); err != nil {
			return err
		}
		if err := tempFile.Close(); err != nil {
			return err
		}
		defer func() { _ = os.Remove(tempFile.Name()) }()
	}

	cmd := exec.Command("/usr/libexec/k3os/install")
	cmd.Env = append(os.Environ(), ev...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	return cmd.Run()
}
