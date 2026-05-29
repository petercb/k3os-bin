package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecodeHook_StringToBool(t *testing.T) {
	type Target struct {
		ForceEFI bool `json:"forceEfi"`
		Silent   bool `json:"silent"`
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

func TestDecodeHook_StringToSlice(t *testing.T) {
	type Target struct {
		Modules []string `json:"modules"`
	}

	data := map[string]interface{}{
		"modules": "single-module",
	}

	var result Target
	err := decodeToObj(data, &result)
	require.NoError(t, err)
	assert.Equal(t, []string{"single-module"}, result.Modules)
}

func TestDecodeHook_MapToStringMap(t *testing.T) {
	type Target struct {
		Labels map[string]string `json:"labels"`
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
