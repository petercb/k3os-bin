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
	require.NoError(t, f.ModifySchema(schema, nil))

	tests := []struct {
		name      string
		input     map[string]interface{}
		wantKey   string
		wantValue interface{}
	}{
		{"password maps to passphrase", map[string]interface{}{"password": "my-password"}, "passphrase", "my-password"},
		{"pass maps to passphrase", map[string]interface{}{"pass": "my-pass"}, "passphrase", "my-pass"},
		{"ssh_authorized_key maps to plural", map[string]interface{}{"ssh_authorized_key": "k"}, "sshAuthorizedKeys", "k"},
		{"environment maps to environments", map[string]interface{}{"environment": "env"}, "environments", "env"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NoError(t, f.ToInternal(tt.input))
			assert.Contains(t, tt.input, tt.wantKey)
			assert.Equal(t, tt.wantValue, tt.input[tt.wantKey])
		})
	}
}
