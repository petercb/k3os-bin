package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCamelToSnake(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"empty string", "", ""},
		{"lowercase", "hostname", "hostname"},
		{"camelCase", "serverUrl", "server_url"},
		{"PascalCase", "DataSources", "data_sources"},
		{"acronym prefix", "DNSNameservers", "dns_nameservers"},
		{"acronym suffix", "configURL", "config_url"},
		{"multiple words", "sshAuthorizedKeys", "ssh_authorized_keys"},
		{"already snake_case", "already_snake", "already_snake"},
		{"single letter", "a", "a"},
		{"all caps", "URL", "url"},
		{"mixed acronym", "isoURL", "iso_url"},
		{"NTP servers", "NTPServers", "ntp_servers"},
		{"force EFI", "forceEfi", "force_efi"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := camelToSnake(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetValue(t *testing.T) {
	data := map[string]interface{}{
		"k3os": map[string]interface{}{
			"install": map[string]interface{}{
				"device": "/dev/sda",
			},
			"password": "secret",
		},
		"hostname": "myhost",
	}

	tests := []struct {
		name   string
		keys   []string
		want   interface{}
		wantOK bool
	}{
		{"top-level key", []string{"hostname"}, "myhost", true},
		{"nested key", []string{"k3os", "password"}, "secret", true},
		{"deeply nested", []string{"k3os", "install", "device"}, "/dev/sda", true},
		{"missing key", []string{"nonexistent"}, nil, false},
		{"missing nested key", []string{"k3os", "nonexistent"}, nil, false},
		{"empty keys", []string{}, nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := getValue(data, tt.keys...)
			assert.Equal(t, tt.wantOK, ok)
			if tt.wantOK {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestPutValue(t *testing.T) {
	t.Run("simple key", func(t *testing.T) {
		data := map[string]interface{}{}
		putValue(data, "myhost", "hostname")
		assert.Equal(t, "myhost", data["hostname"])
	})

	t.Run("nested key creates intermediate maps", func(t *testing.T) {
		data := map[string]interface{}{}
		putValue(data, "/dev/sda", "k3os", "install", "device")

		k3os, ok := data["k3os"].(map[string]interface{})
		assert.True(t, ok)
		install, ok := k3os["install"].(map[string]interface{})
		assert.True(t, ok)
		assert.Equal(t, "/dev/sda", install["device"])
	})

	t.Run("overwrite existing value", func(t *testing.T) {
		data := map[string]interface{}{
			"hostname": "old",
		}
		putValue(data, "new", "hostname")
		assert.Equal(t, "new", data["hostname"])
	})

	t.Run("empty keys does nothing", func(t *testing.T) {
		data := map[string]interface{}{}
		putValue(data, "value")
		assert.Empty(t, data)
	})
}
