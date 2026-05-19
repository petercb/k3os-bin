package config

import (
	"testing"

	"github.com/rancher/mapper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFuzzyNames(t *testing.T) {
	f := &FuzzyNames{}

	schema := &mapper.Schema{
		ResourceFields: map[string]mapper.Field{
			"passphrases":       {},
			"sshAuthorizedKeys": {},
			"environments":      {},
		},
	}

	err := f.ModifySchema(schema, nil)
	require.NoError(t, err)

	data := map[string]interface{}{
		"pass":               "my-pass",
		"ssh_authorized_key": "my-key",
		"environment":        "env",
		"password":           "my-password",
	}

	err = f.ToInternal(data)
	require.NoError(t, err)

	assert.Contains(t, data, "passphrase")
	assert.Equal(t, "my-password", data["passphrase"]) // Last one evaluated overrides. Actually, 'pass' or 'password' map to 'passphrase'

	assert.Contains(t, data, "sshAuthorizedKeys")
	assert.Equal(t, "my-key", data["sshAuthorizedKeys"])

	assert.Contains(t, data, "environments")
	assert.Equal(t, "env", data["environments"])
}
