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
		expected  bool
	}{
		{
			name:      "singular to plural snake_case",
			mapKey:    "ssh_authorized_key",
			fieldName: "sshAuthorizedKeys",
			expected:  true,
		},
		{
			name:      "password alias passphrase",
			mapKey:    "password",
			fieldName: "passphrase",
			expected:  true,
		},
		{
			name:      "pass alias passphrase",
			mapKey:    "pass",
			fieldName: "passphrase",
			expected:  true,
		},
		{
			name:      "singular dns_nameserver to plural",
			mapKey:    "dns_nameserver",
			fieldName: "dnsNameservers",
			expected:  true,
		},
		{
			name:      "exact camelCase match",
			mapKey:    "sshAuthorizedKeys",
			fieldName: "sshAuthorizedKeys",
			expected:  true,
		},
		{
			name:      "snake_case variant matches camelCase field",
			mapKey:    "ssh_authorized_keys",
			fieldName: "sshAuthorizedKeys",
			expected:  true,
		},
		{
			name:      "exact lowercase match",
			mapKey:    "hostname",
			fieldName: "hostname",
			expected:  true,
		},
		{
			name:      "exact modules match",
			mapKey:    "modules",
			fieldName: "modules",
			expected:  true,
		},
		{
			name:      "singular module matches modules",
			mapKey:    "module",
			fieldName: "modules",
			expected:  true,
		},
		{
			name:      "datasource matches dataSources",
			mapKey:    "datasource",
			fieldName: "dataSources",
			expected:  true,
		},
		{
			name:      "unrelated fields do not match",
			mapKey:    "environment",
			fieldName: "modules",
			expected:  false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := matchName(tc.mapKey, tc.fieldName)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestDecodeToObj_BoolCoercion(t *testing.T) {
	type Target struct {
		ForceEFI bool `json:"forceEfi" yaml:"force_efi"`
		Silent   bool `json:"silent" yaml:"silent"`
	}

	data := map[string]interface{}{
		"forceEfi": "true",
		"silent":   "false",
	}

	var result Target
	err := decodeToObj(data, &result)
	require.NoError(t, err)
	assert.True(t, result.ForceEFI)
	assert.False(t, result.Silent)
}

func TestDecodeToObj_SliceCoercion(t *testing.T) {
	type Target struct {
		Modules []string `json:"modules" yaml:"modules"`
	}

	data := map[string]interface{}{
		"modules": "single-module",
	}

	var result Target
	err := decodeToObj(data, &result)
	require.NoError(t, err)
	assert.Equal(t, []string{"single-module"}, result.Modules)
}

func TestDecodeToObj_MapCoercion(t *testing.T) {
	type Target struct {
		Labels map[string]string `json:"labels" yaml:"labels"`
	}

	data := map[string]interface{}{
		"labels": map[string]interface{}{
			"key1": "value1",
			"key2": 123,
		},
	}

	var result Target
	err := decodeToObj(data, &result)
	require.NoError(t, err)
	assert.Equal(t, "value1", result.Labels["key1"])
	assert.Equal(t, "123", result.Labels["key2"])
}

func TestDecodeToObj_FullCloudConfig(t *testing.T) {
	raw := map[string]interface{}{
		"hostname":           "mynode",
		"sshAuthorizedKeys":  []interface{}{"ssh-rsa AAAA..."},
		"ssh_authorized_key": "ssh-ed25519 BBBB...",
		"k3os": map[string]interface{}{
			"server_url":     "https://server:6443",
			"token":          "secret",
			"dns_nameserver": "8.8.8.8",
			"modules":        "br_netfilter",
			"labels": map[string]interface{}{
				"env":  "prod",
				"tier": 1,
			},
			"install": map[string]interface{}{
				"device":   "/dev/sda",
				"forceEfi": "true",
			},
		},
	}

	var cfg CloudConfig
	err := decodeToObj(raw, &cfg)
	require.NoError(t, err)

	assert.Equal(t, "mynode", cfg.Hostname)
	assert.Contains(t, cfg.K3OS.DNSNameservers, "8.8.8.8")
	assert.Equal(t, "https://server:6443", cfg.K3OS.ServerURL)
	assert.Equal(t, "secret", cfg.K3OS.Token)
	assert.Contains(t, cfg.K3OS.Modules, "br_netfilter")
	assert.Equal(t, "prod", cfg.K3OS.Labels["env"])
	assert.Equal(t, "1", cfg.K3OS.Labels["tier"])
	assert.Equal(t, "/dev/sda", cfg.K3OS.Install.Device)
	assert.True(t, cfg.K3OS.Install.ForceEFI)
}
