package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMatchName(t *testing.T) {
	tests := []struct {
		name      string
		mapKey    string
		fieldName string
		want      bool
	}{
		{"exact match", "hostname", "hostname", true},
		{"camelCase to snake_case", "serverUrl", "server_url", true},
		{"snake_case to camelCase", "server_url", "serverUrl", true},
		{"singular to plural (s)", "module", "modules", true},
		{"singular to plural (es)", "datasource", "data_sources", true},
		{"password to passphrase", "password", "passphrase", true},
		{"pass to passphrase", "pass", "passphrase", true},
		{"compound word datasource to data_sources", "datasource", "data_sources", true},
		{"compound word ntpserver to ntp_servers", "ntpserver", "ntp_servers", true},
		{"no match", "hostname", "password", false},
		{"PascalCase to snake_case", "DataSources", "data_sources", true},
		{"case insensitive", "Hostname", "hostname", true},
		{"dns_nameserver to dns_nameservers", "dns_nameserver", "dns_nameservers", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchName(tt.mapKey, tt.fieldName)
			assert.Equal(t, tt.want, got, "matchName(%q, %q)", tt.mapKey, tt.fieldName)
		})
	}
}

func TestDecodeToObj(t *testing.T) {
	input := map[string]interface{}{
		"hostname": "my-host",
		"k3os": map[string]interface{}{
			"password":   "secret",
			"server_url": "https://example.com",
			"modules":    []interface{}{"mod1", "mod2"},
			"labels": map[string]interface{}{
				"role": "worker",
			},
			"install": map[string]interface{}{
				"device":    "/dev/sda",
				"force_efi": true,
				"silent":    "true",
			},
		},
		"ssh_authorized_keys": []interface{}{"ssh-rsa AAAA"},
	}

	var cfg CloudConfig
	err := decodeToObj(input, &cfg)
	require.NoError(t, err)

	assert.Equal(t, "my-host", cfg.Hostname)
	assert.Equal(t, "secret", cfg.K3OS.Password)
	assert.Equal(t, "https://example.com", cfg.K3OS.ServerURL)
	assert.Equal(t, []string{"mod1", "mod2"}, cfg.K3OS.Modules)
	assert.Equal(t, "worker", cfg.K3OS.Labels["role"])
	assert.Equal(t, "/dev/sda", cfg.K3OS.Install.Device)
	assert.True(t, cfg.K3OS.Install.ForceEFI)
	assert.True(t, cfg.K3OS.Install.Silent)
	assert.Equal(t, []string{"ssh-rsa AAAA"}, cfg.SSHAuthorizedKeys)
}
