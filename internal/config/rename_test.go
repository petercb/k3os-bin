package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMatchName_Aliases(t *testing.T) {
	tests := []struct {
		name      string
		mapKey    string
		fieldName string
		expected  bool
	}{
		{"password maps to passphrase", "password", "passphrase", true},
		{"pass maps to passphrase", "pass", "passphrase", true},
		{"ssh_authorized_key maps to sshAuthorizedKeys", "ssh_authorized_key", "sshAuthorizedKeys", true},
		{"environment maps to environments", "environment", "environments", true},
		{"unrelated field does not match", "hostname", "password", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchName(tt.mapKey, tt.fieldName)
			assert.Equal(t, tt.expected, result)
		})
	}
}
