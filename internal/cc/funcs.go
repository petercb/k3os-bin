package cc

import (
	"bufio"
	"bytes"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"

	"github.com/petercb/k3os-bin/internal/config"
	"github.com/petercb/k3os-bin/internal/hostname"
	"github.com/petercb/k3os-bin/internal/mode"
	"github.com/petercb/k3os-bin/internal/ssh"
	"github.com/petercb/k3os-bin/internal/version"
	"github.com/petercb/k3os-bin/internal/writefile"
)

// ApplyModules loads configured kernel modules.
func (a *Applier) ApplyModules(cfg *config.CloudConfig) error {
	loaded, err := a.Modules.LoadedModules()
	if err != nil {
		return err
	}
	for _, m := range cfg.K3OS.Modules {
		if loaded[m] {
			continue
		}
		params := strings.Split(m, " ")
		slog.Debug("module loading", "module", m, "parameters", params)
		if err := a.Modules.LoadModule(params[0], strings.Join(params[1:], " ")); err != nil {
			return fmt.Errorf("could not load module %s with parameters [%s], err %w", m, params, err)
		}
		slog.Debug("module loaded", "module", m)
	}
	return nil
}

// ApplySysctls applies configured sysctl values.
func (a *Applier) ApplySysctls(cfg *config.CloudConfig) error {
	for k, v := range cfg.K3OS.Sysctls {
		if err := a.Sysctl.Set(k, v); err != nil {
			return err
		}
	}
	return nil
}

// ApplyHostname applies the configured hostname.
func (a *Applier) ApplyHostname(cfg *config.CloudConfig) error {
	return hostname.SetHostname(cfg, a.Hostname, a.FS)
}

// ApplyPassword applies the configured rancher user password.
func (a *Applier) ApplyPassword(cfg *config.CloudConfig) error {
	if cfg.K3OS.Password == "" {
		return nil
	}
	return a.Password.SetPassword(a.FS, "rancher", cfg.K3OS.Password)
}

// ApplyRuncmd executes configured run commands.
func (a *Applier) ApplyRuncmd(cfg *config.CloudConfig) error {
	for _, cmd := range cfg.Runcmd {
		slog.Debug("running command", "cmd", cmd)
		if err := a.Cmd.RunShell(cmd); err != nil {
			return err
		}
	}
	return nil
}

// ApplyBootcmd executes configured boot commands.
func (a *Applier) ApplyBootcmd(cfg *config.CloudConfig) error {
	for _, cmd := range cfg.Bootcmd {
		slog.Debug("running command", "cmd", cmd)
		if err := a.Cmd.RunShell(cmd); err != nil {
			return err
		}
	}
	return nil
}

// ApplyInitcmd executes configured init commands.
func (a *Applier) ApplyInitcmd(cfg *config.CloudConfig) error {
	for _, cmd := range cfg.Initcmd {
		slog.Debug("running command", "cmd", cmd)
		if err := a.Cmd.RunShell(cmd); err != nil {
			return err
		}
	}
	return nil
}

// ApplyWriteFiles writes configured files to disk.
func (a *Applier) ApplyWriteFiles(cfg *config.CloudConfig) error {
	writefile.WriteFiles(cfg, a.FS, a.Cmd)
	return nil
}

// ApplySSHKeys applies configured SSH authorized keys without network fetches.
func (a *Applier) ApplySSHKeys(cfg *config.CloudConfig) error {
	return ssh.SetAuthorizedKeys(cfg, false, a.FS)
}

// ApplySSHKeysWithNet applies configured SSH authorized keys with network fetches enabled.
func (a *Applier) ApplySSHKeysWithNet(cfg *config.CloudConfig) error {
	return ssh.SetAuthorizedKeys(cfg, true, a.FS)
}

// ApplyK3SWithRestart applies k3s configuration and allows service restart.
func (a *Applier) ApplyK3SWithRestart(cfg *config.CloudConfig) error {
	return a.ApplyK3S(cfg, true, false)
}

// ApplyK3SInstall applies k3s installation configuration.
func (a *Applier) ApplyK3SInstall(cfg *config.CloudConfig) error {
	return a.ApplyK3S(cfg, true, true)
}

// ApplyK3SNoRestart applies k3s configuration without starting the service.
func (a *Applier) ApplyK3SNoRestart(cfg *config.CloudConfig) error {
	return a.ApplyK3S(cfg, false, false)
}

// ApplyK3S applies k3s installation and runtime arguments.
func (a *Applier) ApplyK3S(cfg *config.CloudConfig, restart, install bool) error {
	mode, err := mode.Get(a.modePrefix...)
	if err != nil {
		return err
	}
	if mode == "install" {
		return nil
	}

	k3sExists := false
	k3sLocalExists := false
	if _, err := a.FS.Stat("/sbin/k3s"); err == nil {
		k3sExists = true
	}
	if _, err := a.FS.Stat("/usr/local/bin/k3s"); err == nil {
		k3sLocalExists = true
	}

	args := cfg.K3OS.K3sArgs
	vars := []string{
		"INSTALL_K3S_NAME=service",
	}

	if !k3sExists && !restart {
		return nil
	}

	//nolint:gocritic
	if k3sExists {
		vars = append(vars, "INSTALL_K3S_SKIP_DOWNLOAD=true")
		vars = append(vars, "INSTALL_K3S_BIN_DIR=/sbin")
		vars = append(vars, "INSTALL_K3S_BIN_DIR_READ_ONLY=true")
	} else if k3sLocalExists {
		vars = append(vars, "INSTALL_K3S_SKIP_DOWNLOAD=true")
	} else if !install {
		return nil
	}

	if !restart {
		vars = append(vars, "INSTALL_K3S_SKIP_START=true")
	}

	if cfg.K3OS.ServerURL == "" {
		if len(args) == 0 {
			args = append(args, "server")
		}
	} else {
		vars = append(vars, "K3S_URL="+cfg.K3OS.ServerURL)
		if len(args) == 0 {
			args = append(args, "agent")
		}
	}

	if cfg.K3OS.Token != "" {
		vars = append(vars, "K3S_TOKEN="+cfg.K3OS.Token)
	}

	var labels []string
	for k, v := range cfg.K3OS.Labels {
		labels = append(labels, fmt.Sprintf("%s=%s", k, v))
	}
	if mode != "" {
		labels = append(labels, "k3os.io/mode="+mode)
	}
	labels = append(labels, "k3os.io/version="+version.Version)
	sort.Strings(labels)

	for _, l := range labels {
		args = append(args, "--node-label", l)
	}

	for _, taint := range cfg.K3OS.Taints {
		args = append(args, "--kubelet-arg", "register-with-taints="+taint)
	}

	slog.Debug("running k3s install", "args", args, "vars", vars)

	return a.Cmd.RunWithEnv(vars, "/usr/libexec/k3os/k3s-install.sh", args...)
}

// ApplyInstall invokes k3os install mode when requested.
func (a *Applier) ApplyInstall(_ *config.CloudConfig) error {
	mode, err := mode.Get(a.modePrefix...)
	if err != nil {
		return err
	}
	if mode != "install" {
		return nil
	}

	return a.Cmd.Run("k3os", "install")
}

// ApplyDNS writes connman DNS and NTP configuration.
func (a *Applier) ApplyDNS(cfg *config.CloudConfig) error {
	buf := &bytes.Buffer{}
	buf.WriteString("[General]\n")
	buf.WriteString("NetworkInterfaceBlacklist=veth\n")
	buf.WriteString("PreferredTechnologies=ethernet,wifi\n")
	if len(cfg.K3OS.DNSNameservers) > 0 {
		dns := strings.Join(cfg.K3OS.DNSNameservers, ",")
		buf.WriteString("FallbackNameservers=")
		buf.WriteString(dns)
		buf.WriteString("\n")
	} else {
		buf.WriteString("FallbackNameservers=8.8.8.8\n")
	}

	if len(cfg.K3OS.NTPServers) > 0 {
		ntp := strings.Join(cfg.K3OS.NTPServers, ",")
		buf.WriteString("FallbackTimeservers=")
		buf.WriteString(ntp)
		buf.WriteString("\n")
	}

	err := a.FS.WriteFile("/etc/connman/main.conf", buf.Bytes(), 0o644)
	if err != nil {
		return fmt.Errorf("failed to write /etc/connman/main.conf: %w", err)
	}

	return nil
}

// ApplyWifi writes connman Wi-Fi configuration.
func (a *Applier) ApplyWifi(cfg *config.CloudConfig) error {
	if len(cfg.K3OS.Wifi) == 0 {
		return nil
	}

	buf := &bytes.Buffer{}

	buf.WriteString("[WiFi]\n")
	buf.WriteString("Enable=true\n")
	buf.WriteString("Tethering=false\n")

	if buf.Len() > 0 {
		if err := a.FS.MkdirAll("/var/lib/connman", 0o755); err != nil {
			return fmt.Errorf("failed to mkdir /var/lib/connman: %w", err)
		}
		if err := a.FS.WriteFile("/var/lib/connman/settings", buf.Bytes(), 0o644); err != nil {
			return fmt.Errorf("failed to write to /var/lib/connman/settings: %w", err)
		}
	}

	buf = &bytes.Buffer{}

	buf.WriteString("[global]\n")
	buf.WriteString("Name=cloud-config\n")
	buf.WriteString("Description=Services defined in the cloud-config\n")

	for i, w := range cfg.K3OS.Wifi {
		name := fmt.Sprintf("wifi%d", i)
		buf.WriteString("[service_")
		buf.WriteString(name)
		buf.WriteString("]\n")
		buf.WriteString("Type=wifi\n")
		buf.WriteString("Passphrase=")
		buf.WriteString(w.Passphrase)
		buf.WriteString("\n")
		buf.WriteString("Name=")
		buf.WriteString(w.Name)
		buf.WriteString("\n")
	}

	if buf.Len() > 0 {
		return a.FS.WriteFile("/var/lib/connman/cloud-config.config", buf.Bytes(), 0o644)
	}

	return nil
}

// ApplyDataSource writes configured cloud-config data source arguments.
func (a *Applier) ApplyDataSource(cfg *config.CloudConfig) error {
	if len(cfg.K3OS.DataSources) == 0 {
		return nil
	}

	args := strings.Join(cfg.K3OS.DataSources, " ")
	buf := &bytes.Buffer{}

	buf.WriteString("command_args=\"")
	buf.WriteString(args)
	buf.WriteString("\"\n")

	if err := a.FS.WriteFile("/etc/conf.d/cloud-config", buf.Bytes(), 0o644); err != nil {
		return fmt.Errorf("failed to write to /etc/conf.d/cloud-config: %w", err)
	}

	return nil
}

// ApplyEnvironment merges configured environment variables into /etc/environment.
func (a *Applier) ApplyEnvironment(cfg *config.CloudConfig) error {
	if len(cfg.K3OS.Environment) == 0 {
		return nil
	}
	env := make(map[string]string, len(cfg.K3OS.Environment))
	if buf, err := a.FS.ReadFile("/etc/environment"); err == nil {
		scanner := bufio.NewScanner(bytes.NewReader(buf))
		for scanner.Scan() {
			line := scanner.Text()
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "#") {
				continue
			}
			line = strings.TrimPrefix(line, "export")
			line = strings.TrimSpace(line)
			if len(line) > 1 {
				parts := strings.SplitN(line, "=", 2)
				key := parts[0]
				val := ""
				if len(parts) > 1 {
					if val, err = strconv.Unquote(parts[1]); err != nil {
						val = parts[1]
					}
				}
				env[key] = val
			}
		}
	}
	for key, val := range cfg.K3OS.Environment {
		env[key] = val
	}
	buf := &bytes.Buffer{}
	for key, val := range env {
		buf.WriteString(key)
		buf.WriteString("=")
		buf.WriteString(strconv.Quote(val))
		buf.WriteString("\n")
	}
	if err := a.FS.WriteFile("/etc/environment", buf.Bytes(), 0o644); err != nil {
		return fmt.Errorf("failed to write to /etc/environment: %w", err)
	}

	return nil
}
