package config

import (
	"fmt"
	"os"
	"strconv"
)

// K3OS holds the k3OS-specific configuration options.
type K3OS struct {
	DataSources    []string          `json:"dataSources,omitempty" yaml:"data_sources,omitempty"`
	Modules        []string          `json:"modules,omitempty" yaml:"modules,omitempty"`
	Sysctls        map[string]string `json:"sysctls,omitempty" yaml:"sysctls,omitempty"`
	NTPServers     []string          `json:"ntpServers,omitempty" yaml:"ntp_servers,omitempty"`
	DNSNameservers []string          `json:"dnsNameservers,omitempty" yaml:"dns_nameservers,omitempty"`
	Wifi           []Wifi            `json:"wifi,omitempty" yaml:"wifi,omitempty"`
	Password       string            `json:"password,omitempty" yaml:"password,omitempty"`
	ServerURL      string            `json:"serverUrl,omitempty" yaml:"server_url,omitempty"`
	Token          string            `json:"token,omitempty" yaml:"token,omitempty"`
	Labels         map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	K3sArgs        []string          `json:"k3sArgs,omitempty" yaml:"k3s_args,omitempty"`
	Environment    map[string]string `json:"environment,omitempty" yaml:"environment,omitempty"`
	Taints         []string          `json:"taints,omitempty" yaml:"taints,omitempty"`
	Install        *Install          `json:"install,omitempty" yaml:"install,omitempty"`
}

// Wifi represents a WiFi network configuration with name and passphrase.
type Wifi struct {
	Name       string `json:"name,omitempty" yaml:"name,omitempty"`
	Passphrase string `json:"passphrase,omitempty" yaml:"passphrase,omitempty"`
}

// Install holds the configuration for a k3OS disk installation.
type Install struct {
	ForceEFI  bool   `json:"forceEfi,omitempty" yaml:"force_efi,omitempty"`
	Device    string `json:"device,omitempty" yaml:"device,omitempty"`
	ConfigURL string `json:"configUrl,omitempty" yaml:"config_url,omitempty"`
	Silent    bool   `json:"silent,omitempty" yaml:"silent,omitempty"`
	ISOURL    string `json:"isoUrl,omitempty" yaml:"iso_url,omitempty"`
	PowerOff  bool   `json:"powerOff,omitempty" yaml:"power_off,omitempty"`
	NoFormat  bool   `json:"noFormat,omitempty" yaml:"no_format,omitempty"`
	Debug     bool   `json:"debug,omitempty" yaml:"debug,omitempty"`
	TTY       string `json:"tty,omitempty" yaml:"tty,omitempty"`
}

// CloudConfig is the top-level cloud-init configuration structure for k3OS.
type CloudConfig struct {
	SSHAuthorizedKeys []string `json:"sshAuthorizedKeys,omitempty" yaml:"ssh_authorized_keys,omitempty"`
	WriteFiles        []File   `json:"writeFiles,omitempty" yaml:"write_files,omitempty"`
	Hostname          string   `json:"hostname,omitempty" yaml:"hostname,omitempty"`
	K3OS              K3OS     `json:"k3os,omitempty" yaml:"k3os,omitempty"`
	Runcmd            []string `json:"runCmd,omitempty" yaml:"run_cmd,omitempty"`
	Bootcmd           []string `json:"bootCmd,omitempty" yaml:"boot_cmd,omitempty"`
	Initcmd           []string `json:"initCmd,omitempty" yaml:"init_cmd,omitempty"`
}

// File represents a file to be written to the filesystem during cloud-init.
type File struct {
	Encoding           string `json:"encoding" yaml:"encoding"`
	Content            string `json:"content" yaml:"content"`
	Owner              string `json:"owner" yaml:"owner"`
	Path               string `json:"path" yaml:"path"`
	RawFilePermissions string `json:"permissions" yaml:"permissions"`
}

// Permissions parses the raw file permissions string and returns an os.FileMode.
func (f *File) Permissions() (os.FileMode, error) {
	if f.RawFilePermissions == "" {
		return os.FileMode(0o644), nil
	}
	// parse string representation of file mode as integer
	perm, err := strconv.ParseInt(f.RawFilePermissions, 8, 32)
	if err != nil {
		return 0, fmt.Errorf("unable to parse file permissions %q as integer", f.RawFilePermissions)
	}
	return os.FileMode(perm), nil
}
