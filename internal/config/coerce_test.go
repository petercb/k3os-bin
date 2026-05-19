package config

import (
	"testing"

	"github.com/rancher/mapper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewToMap(t *testing.T) {
	m := NewToMap()

	schema := &mapper.Schema{
		ResourceFields: map[string]mapper.Field{
			"labels": {Type: "map[string]"},
		},
	}

	err := m.ModifySchema(schema, nil)
	require.NoError(t, err)

	data := map[string]interface{}{
		"labels": map[string]interface{}{
			"key1": "value1",
			"key2": 123,
		},
		"other": map[string]interface{}{
			"key3": 456,
		},
	}

	err = m.ToInternal(data)
	require.NoError(t, err)

	labels, ok := data["labels"].(map[string]string)
	require.True(t, ok, "labels should be converted to map[string]string")
	assert.Equal(t, "value1", labels["key1"])
	assert.Equal(t, "123", labels["key2"])

	// other should remain unchanged
	other, ok := data["other"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, 456, other["key3"])
}

func TestNewToSlice(t *testing.T) {
	m := NewToSlice()

	schema := &mapper.Schema{
		ResourceFields: map[string]mapper.Field{
			"modules": {Type: "array[string]"},
		},
	}

	err := m.ModifySchema(schema, nil)
	require.NoError(t, err)

	data := map[string]interface{}{
		"modules": "single-module",
	}

	err = m.ToInternal(data)
	require.NoError(t, err)

	modules, ok := data["modules"].([]string)
	require.True(t, ok, "modules should be converted to []string")
	assert.Equal(t, []string{"single-module"}, modules)
}

func TestNewToBool(t *testing.T) {
	m := NewToBool()

	schema := &mapper.Schema{
		ResourceFields: map[string]mapper.Field{
			"forceEfi": {Type: "boolean"},
		},
	}

	err := m.ModifySchema(schema, nil)
	require.NoError(t, err)

	data := map[string]interface{}{
		"forceEfi": "true",
	}

	err = m.ToInternal(data)
	require.NoError(t, err)

	forceEfi, ok := data["forceEfi"].(bool)
	require.True(t, ok, "forceEfi should be converted to bool")
	assert.True(t, forceEfi)

	data["forceEfi"] = "false"
	err = m.ToInternal(data)
	require.NoError(t, err)

	forceEfiFalse, ok := data["forceEfi"].(bool)
	require.True(t, ok, "forceEfi should be converted to bool even if false")
	assert.False(t, forceEfiFalse)
}
