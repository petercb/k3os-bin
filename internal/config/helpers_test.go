package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCamelToSnake(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"ssh_authorized_keys", "sshAuthorizedKeys", "ssh_authorized_keys"},
		{"dns_nameservers", "dnsNameservers", "dns_nameservers"},
		{"force_efi", "forceEfi", "force_efi"},
		{"hostname unchanged", "hostname", "hostname"},
		{"k3s_args", "k3sArgs", "k3s_args"},
		{"iso_url", "isoUrl", "iso_url"},
		{"server_url", "serverUrl", "server_url"},
		{"config_url", "configUrl", "config_url"},
		{"no_format", "noFormat", "no_format"},
		{"ntp_servers", "ntpServers", "ntp_servers"},
		{"consecutive caps ISOURL", "ISOURL", "iso_url"},
		{"all caps TTY", "TTY", "tty"},
		{"empty string", "", ""},
		{"write_files", "writeFiles", "write_files"},
		{"run_cmd", "runCmd", "run_cmd"},
		{"boot_cmd", "bootCmd", "boot_cmd"},
		{"init_cmd", "initCmd", "init_cmd"},
		{"power_off", "powerOff", "power_off"},
		{"data_sources", "dataSources", "data_sources"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := camelToSnake(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestToString(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{"string value", "hello", "hello"},
		{"int value", 123, "123"},
		{"float64 value", 3.14, "3.14"},
		{"bool true", true, "true"},
		{"nil value", nil, "<nil>"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := toString(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestEncodeToMap(t *testing.T) {
	t.Run("simple struct with json tags", func(t *testing.T) {
		type Simple struct {
			Name  string `json:"name"`
			Count int    `json:"count"`
		}

		obj := Simple{Name: "test", Count: 42}
		result, err := encodeToMap(obj)
		require.NoError(t, err)

		assert.Equal(t, "test", result["name"])
		assert.InDelta(t, float64(42), result["count"], 0) // JSON numbers decode as float64
	})

	t.Run("nested struct becomes nested map", func(t *testing.T) {
		type Inner struct {
			Value string `json:"value"`
		}
		type Outer struct {
			Name  string `json:"name"`
			Inner Inner  `json:"inner"`
		}

		obj := Outer{Name: "outer", Inner: Inner{Value: "nested"}}
		result, err := encodeToMap(obj)
		require.NoError(t, err)

		assert.Equal(t, "outer", result["name"])
		inner, ok := result["inner"].(map[string]interface{})
		require.True(t, ok, "inner should be a map")
		assert.Equal(t, "nested", inner["value"])
	})

	t.Run("nil pointer fields omitted with omitempty", func(t *testing.T) {
		type WithPointer struct {
			Name    string  `json:"name"`
			OptPtr  *string `json:"opt_ptr,omitempty"`
			Present string  `json:"present"`
		}

		obj := WithPointer{Name: "test", OptPtr: nil, Present: "here"}
		result, err := encodeToMap(obj)
		require.NoError(t, err)

		assert.Equal(t, "test", result["name"])
		assert.Equal(t, "here", result["present"])
		_, exists := result["opt_ptr"]
		assert.False(t, exists, "nil pointer with omitempty should not be in map")
	})

	t.Run("slice field preserved in map", func(t *testing.T) {
		type WithSlice struct {
			Items []string `json:"items"`
		}

		obj := WithSlice{Items: []string{"a", "b", "c"}}
		result, err := encodeToMap(obj)
		require.NoError(t, err)

		items, ok := result["items"].([]interface{})
		require.True(t, ok, "items should be a slice")
		assert.Len(t, items, 3)
		assert.Equal(t, "a", items[0])
		assert.Equal(t, "b", items[1])
		assert.Equal(t, "c", items[2])
	})
}

func TestGetValue(t *testing.T) {
	data := map[string]interface{}{
		"k3os": map[string]interface{}{
			"hostname": "myhost",
			"install": map[string]interface{}{
				"device": "/dev/sda",
			},
		},
		"top": "level",
	}

	t.Run("nested access", func(t *testing.T) {
		val, ok := getValue(data, "k3os", "hostname")
		assert.True(t, ok)
		assert.Equal(t, "myhost", val)
	})

	t.Run("deeply nested access", func(t *testing.T) {
		val, ok := getValue(data, "k3os", "install", "device")
		assert.True(t, ok)
		assert.Equal(t, "/dev/sda", val)
	})

	t.Run("single key access", func(t *testing.T) {
		val, ok := getValue(data, "top")
		assert.True(t, ok)
		assert.Equal(t, "level", val)
	})

	t.Run("missing key returns false", func(t *testing.T) {
		val, ok := getValue(data, "k3os", "missing")
		assert.False(t, ok)
		assert.Nil(t, val)
	})

	t.Run("missing intermediate key returns false", func(t *testing.T) {
		val, ok := getValue(data, "nonexistent", "key")
		assert.False(t, ok)
		assert.Nil(t, val)
	})

	t.Run("empty keys returns nil false", func(t *testing.T) {
		val, ok := getValue(data)
		assert.False(t, ok)
		assert.Nil(t, val)
	})
}

func TestPutValue(t *testing.T) {
	t.Run("creates intermediate maps for nested keys", func(t *testing.T) {
		data := map[string]interface{}{}
		putValue(data, "myhost", "k3os", "hostname")

		k3os, ok := data["k3os"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "myhost", k3os["hostname"])
	})

	t.Run("creates deep nested path", func(t *testing.T) {
		data := map[string]interface{}{}
		putValue(data, "/dev/sda", "k3os", "install", "device")

		k3os, ok := data["k3os"].(map[string]interface{})
		require.True(t, ok)
		install, ok := k3os["install"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "/dev/sda", install["device"])
	})

	t.Run("overwrites existing values", func(t *testing.T) {
		data := map[string]interface{}{
			"k3os": map[string]interface{}{
				"hostname": "oldhost",
			},
		}
		putValue(data, "newhost", "k3os", "hostname")

		k3os, ok := data["k3os"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "newhost", k3os["hostname"])
	})

	t.Run("single key works", func(t *testing.T) {
		data := map[string]interface{}{}
		putValue(data, "value", "key")
		assert.Equal(t, "value", data["key"])
	})
}
