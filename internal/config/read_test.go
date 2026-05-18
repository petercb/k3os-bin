package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDataSource(t *testing.T) {
	cc, err := readersToObject(func() (map[string]interface{}, error) {
		return map[string]interface{}{
			"k3os": map[string]interface{}{
				"datasource": "foo",
			},
		}, nil
	})
	require.NoError(t, err)
	require.Len(t, cc.K3OS.DataSources, 1, "expected exactly one datasource")
	assert.Equal(t, "foo", cc.K3OS.DataSources[0])
}

func TestAuthorizedKeys(t *testing.T) {
	c1 := map[string]interface{}{
		"ssh_authorized_keys": []string{
			"one...",
		},
	}
	c2 := map[string]interface{}{
		"ssh_authorized_keys": []string{
			"two...",
		},
	}
	cc, err := readersToObject(
		func() (map[string]interface{}, error) {
			return c1, nil
		},
		func() (map[string]interface{}, error) {
			return c2, nil
		},
	)
	require.NoError(t, err)
	require.Len(t, cc.SSHAuthorizedKeys, 1, "got %d keys, expected 2", len(cc.SSHAuthorizedKeys))
}
