package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMergeData(t *testing.T) {
	t.Run("override semantics", func(t *testing.T) {
		dst := map[string]interface{}{
			"hostname": "host1",
			"k3os": map[string]interface{}{
				"password": "old",
				"token":    "keep-me",
			},
		}
		src := map[string]interface{}{
			"hostname": "host2",
			"k3os": map[string]interface{}{
				"password": "new",
			},
		}

		result, err := mergeData(dst, src)
		require.NoError(t, err)
		assert.Equal(t, "host2", result["hostname"])

		k3os, ok := result["k3os"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "new", k3os["password"])
		assert.Equal(t, "keep-me", k3os["token"])
	})

	t.Run("nil src returns dst unchanged", func(t *testing.T) {
		dst := map[string]interface{}{
			"hostname": "host1",
		}

		result, err := mergeData(dst, nil)
		require.NoError(t, err)
		assert.Equal(t, "host1", result["hostname"])
	})

	t.Run("nil dst creates new map", func(t *testing.T) {
		src := map[string]interface{}{
			"hostname": "host1",
		}

		result, err := mergeData(nil, src)
		require.NoError(t, err)
		assert.Equal(t, "host1", result["hostname"])
	})

	t.Run("nested merge", func(t *testing.T) {
		dst := map[string]interface{}{
			"k3os": map[string]interface{}{
				"install": map[string]interface{}{
					"device": "/dev/sda",
					"silent": true,
				},
			},
		}
		src := map[string]interface{}{
			"k3os": map[string]interface{}{
				"install": map[string]interface{}{
					"device": "/dev/nvme0n1",
				},
			},
		}

		result, err := mergeData(dst, src)
		require.NoError(t, err)

		k3os, ok := result["k3os"].(map[string]interface{})
		require.True(t, ok)
		install, ok := k3os["install"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "/dev/nvme0n1", install["device"])
		assert.Equal(t, true, install["silent"])
	})
}
