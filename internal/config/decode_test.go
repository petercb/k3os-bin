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

func TestDecodeToObj_BoolCoercion_NonBooleanStrings(t *testing.T) {
	type Target struct {
		A bool `json:"a"`
		B bool `json:"b"`
		C bool `json:"c"`
		D bool `json:"d"`
		E bool `json:"e"`
	}

	data := map[string]interface{}{
		"a": "yes",
		"b": "1",
		"c": "",
		"d": "anything",
		"e": "TRUE",
	}

	var result Target
	err := decodeToObj(data, &result)
	require.NoError(t, err)
	// Old behavior: only "true" (case-insensitive) is true, everything else is false
	assert.False(t, result.A, "yes should be false")
	assert.False(t, result.B, "1 should be false")
	assert.False(t, result.C, "empty string should be false")
	assert.False(t, result.D, "arbitrary string should be false")
	assert.True(t, result.E, "TRUE should be true (case-insensitive)")
}

func TestNormalizeData(t *testing.T) {
	t.Run("nil data does not panic", func(_ *testing.T) {
		normalizeData(nil) // should not panic
	})

	t.Run("camelCase keys normalized to snake_case", func(t *testing.T) {
		data := map[string]interface{}{
			"sshAuthorizedKeys": []interface{}{"key1"},
			"dnsNameservers":    []interface{}{"8.8.8.8"},
			"hostname":          "myhost",
		}
		normalizeData(data)
		assert.Contains(t, data, "ssh_authorized_keys")
		assert.Contains(t, data, "dns_nameservers")
		assert.Contains(t, data, "hostname")
		assert.NotContains(t, data, "sshAuthorizedKeys")
		assert.NotContains(t, data, "dnsNameservers")
	})

	t.Run("nested maps normalized recursively", func(t *testing.T) {
		data := map[string]interface{}{
			"k3os": map[string]interface{}{
				"serverUrl":      "https://server:6443",
				"dnsNameservers": []interface{}{"8.8.8.8"},
				"install": map[string]interface{}{
					"forceEfi": "true",
					"isoUrl":   "http://example.com/k3os.iso",
				},
			},
		}
		normalizeData(data)

		k3os, ok := data["k3os"].(map[string]interface{})
		require.True(t, ok)
		assert.Contains(t, k3os, "server_url")
		assert.Contains(t, k3os, "dns_nameservers")
		assert.NotContains(t, k3os, "serverUrl")
		assert.NotContains(t, k3os, "dnsNameservers")

		install, ok := k3os["install"].(map[string]interface{})
		require.True(t, ok)
		assert.Contains(t, install, "force_efi")
		assert.Contains(t, install, "iso_url")
		assert.NotContains(t, install, "forceEfi")
		assert.NotContains(t, install, "isoUrl")
	})

	t.Run("already snake_case keys unchanged", func(t *testing.T) {
		data := map[string]interface{}{
			"ssh_authorized_keys": []interface{}{"key1"},
			"dns_nameservers":     []interface{}{"8.8.8.8"},
		}
		normalizeData(data)
		assert.Contains(t, data, "ssh_authorized_keys")
		assert.Contains(t, data, "dns_nameservers")
		assert.Equal(t, []interface{}{"key1"}, data["ssh_authorized_keys"])
	})

	t.Run("empty map does not panic", func(t *testing.T) {
		data := map[string]interface{}{}
		normalizeData(data)
		assert.Empty(t, data)
	})
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
