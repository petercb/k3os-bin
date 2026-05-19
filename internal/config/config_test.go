package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCloudConfig_Initialization(t *testing.T) {
	cfg := CloudConfig{
		Hostname: "test-host",
		K3OS: K3OS{
			Password: "test-password",
		},
	}

	assert.Equal(t, "test-host", cfg.Hostname)
	assert.Equal(t, "test-password", cfg.K3OS.Password)
}

func TestFile_Permissions(t *testing.T) {
	tests := []struct {
		name        string
		file        File
		expected    os.FileMode
		expectError bool
	}{
		{
			name: "default permissions when empty",
			file: File{
				RawFilePermissions: "",
			},
			expected:    os.FileMode(0o644),
			expectError: false,
		},
		{
			name: "valid octal permissions",
			file: File{
				RawFilePermissions: "0600",
			},
			expected:    os.FileMode(0o600),
			expectError: false,
		},
		{
			name: "invalid permissions",
			file: File{
				RawFilePermissions: "invalid",
			},
			expected:    0,
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			perm, err := tc.file.Permissions()
			if tc.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "unable to parse file permissions")
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expected, perm)
			}
		})
	}
}
