package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMergeData_Override(t *testing.T) {
	dst := map[string]interface{}{
		"hostname": "oldhost",
		"token":    "keep-this",
	}
	src := map[string]interface{}{
		"hostname": "newhost",
	}

	result, err := mergeData(dst, src)
	require.NoError(t, err)
	assert.Equal(t, "newhost", result["hostname"])
	assert.Equal(t, "keep-this", result["token"])
}

func TestMergeData_DeepMerge(t *testing.T) {
	dst := map[string]interface{}{
		"k3os": map[string]interface{}{
			"hostname": "oldhost",
			"token":    "keep",
		},
	}
	src := map[string]interface{}{
		"k3os": map[string]interface{}{
			"hostname":   "newhost",
			"server_url": "https://new:6443",
		},
	}

	result, err := mergeData(dst, src)
	require.NoError(t, err)

	k3os, ok := result["k3os"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "newhost", k3os["hostname"])
	assert.Equal(t, "keep", k3os["token"])
	assert.Equal(t, "https://new:6443", k3os["server_url"])
}

func TestMergeData_SliceOverride(t *testing.T) {
	dst := map[string]interface{}{
		"modules": []interface{}{"mod1", "mod2"},
	}
	src := map[string]interface{}{
		"modules": []interface{}{"mod3"},
	}

	result, err := mergeData(dst, src)
	require.NoError(t, err)

	modules, ok := result["modules"].([]interface{})
	require.True(t, ok)
	assert.Equal(t, []interface{}{"mod3"}, modules)
}

func TestMergeData_NilSource(t *testing.T) {
	dst := map[string]interface{}{
		"hostname": "keep",
	}

	result, err := mergeData(dst, nil)
	require.NoError(t, err)
	assert.Equal(t, "keep", result["hostname"])
}

func TestMergeData_EmptyDst(t *testing.T) {
	src := map[string]interface{}{
		"hostname": "fromSrc",
	}

	result, err := mergeData(map[string]interface{}{}, src)
	require.NoError(t, err)
	assert.Equal(t, "fromSrc", result["hostname"])
}
