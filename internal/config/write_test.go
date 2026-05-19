package config

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToBytes(t *testing.T) {
	cfg := CloudConfig{
		Hostname: "test-host",
		K3OS: K3OS{
			Password: "test-password",
			Install: &Install{
				Device: "/dev/sda",
			},
		},
	}

	out, err := ToBytes(cfg)
	require.NoError(t, err)

	yamlStr := string(out)
	assert.Contains(t, yamlStr, "hostname: test-host")
	assert.Contains(t, yamlStr, "password: test-password")
	assert.NotContains(t, yamlStr, "install:")
	assert.NotContains(t, yamlStr, "/dev/sda")
}

func TestWrite(t *testing.T) {
	cfg := CloudConfig{
		Hostname: "test-write",
	}

	var buf bytes.Buffer
	err := Write(cfg, &buf)
	require.NoError(t, err)

	assert.Contains(t, buf.String(), "hostname: test-write")
}

func TestPrintInstall(t *testing.T) {
	cfg := CloudConfig{
		Hostname: "should-not-be-printed",
		K3OS: K3OS{
			Password: "should-not-be-printed-password",
			Install: &Install{
				Device:   "/dev/sda",
				ForceEFI: true,
			},
		},
	}

	out, err := PrintInstall(cfg)
	require.NoError(t, err)

	yamlStr := string(out)
	assert.Contains(t, yamlStr, "device: /dev/sda")
	// Note: Rancher mapper or toYAMLKeys might convert ForceEFI to force_efi
	assert.Contains(t, yamlStr, "force_efi: true")
	assert.NotContains(t, yamlStr, "hostname:")
	assert.NotContains(t, yamlStr, "password:")
}

func TestToYAMLKeys(t *testing.T) {
	data := map[string]interface{}{
		"camelCaseKey": "value",
		"nestedMap": map[string]interface{}{
			"innerCamelCase": "innerValue",
		},
		"sshAuthorizedKeys": []string{"key1"},
	}

	toYAMLKeys(data)

	// verify top level
	assert.NotContains(t, data, "camelCaseKey")
	assert.Contains(t, data, "camel_case_key")
	assert.Equal(t, "value", data["camel_case_key"])

	// verify ssh keys
	assert.NotContains(t, data, "sshAuthorizedKeys")
	assert.Contains(t, data, "ssh_authorized_keys")

	// verify nested
	assert.NotContains(t, data, "nestedMap")
	assert.Contains(t, data, "nested_map")

	nested, ok := data["nested_map"].(map[string]interface{})
	require.True(t, ok)
	assert.NotContains(t, nested, "innerCamelCase")
	assert.Contains(t, nested, "inner_camel_case")
	assert.Equal(t, "innerValue", nested["inner_camel_case"])
}
