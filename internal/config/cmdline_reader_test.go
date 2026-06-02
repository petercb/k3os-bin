package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseCmdlineArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected map[string]interface{}
	}{
		{
			name:     "empty string",
			input:    "",
			expected: map[string]interface{}{},
		},
		{
			name:  "simple key=value",
			input: "k3os.mode=disk",
			expected: map[string]interface{}{
				"k3os": map[string]interface{}{
					"mode": "disk",
				},
			},
		},
		{
			name:  "quoted value",
			input: `k3os.password="my secret"`,
			expected: map[string]interface{}{
				"k3os": map[string]interface{}{
					"password": "my secret",
				},
			},
		},
		{
			name:  "boolean flag without value",
			input: "quiet",
			expected: map[string]interface{}{
				"quiet": "true",
			},
		},
		{
			name:  "repeated keys become list",
			input: "k3os.dns=8.8.8.8 k3os.dns=1.1.1.1",
			expected: map[string]interface{}{
				"k3os": map[string]interface{}{
					"dns": []string{"8.8.8.8", "1.1.1.1"},
				},
			},
		},
		{
			name:  "non-k3os params",
			input: "root=/dev/sda1",
			expected: map[string]interface{}{
				"root": "/dev/sda1",
			},
		},
		{
			name:  "mixed params",
			input: `k3os.hostname=myhost k3os.password="pass" k3os.dns=8.8.8.8 k3os.dns=1.1.1.1 some_other_param=foo`,
			expected: map[string]interface{}{
				"k3os": map[string]interface{}{
					"hostname": "myhost",
					"password": "pass",
					"dns":      []string{"8.8.8.8", "1.1.1.1"},
				},
				"some_other_param": "foo",
			},
		},
		{
			name:  "multiple levels of nesting",
			input: "k3os.install.silent=true k3os.install.tty=ttyS0",
			expected: map[string]interface{}{
				"k3os": map[string]interface{}{
					"install": map[string]interface{}{
						"silent": "true",
						"tty":    "ttyS0",
					},
				},
			},
		},
		{
			name:  "fully quoted token",
			input: `"k3os.mode=disk"`,
			expected: map[string]interface{}{
				"k3os": map[string]interface{}{
					"mode": "disk",
				},
			},
		},
		{
			name:  "mismatched quotes treats rest as one token",
			input: `k3os.mode="disk k3os.hostname=myhost`,
			expected: map[string]interface{}{
				"k3os": map[string]interface{}{
					"mode": "disk k3os.hostname=myhost",
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := parseCmdlineArgs(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestSplitCmdlineTokens(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "fully quoted token",
			input:    `"k3os.mode=disk"`,
			expected: []string{`"k3os.mode=disk"`},
		},
		{
			name:     "mismatched quote groups to end",
			input:    `k3os.mode="disk k3os.hostname=myhost`,
			expected: []string{`k3os.mode="disk k3os.hostname=myhost`},
		},
		{
			name:     "unicode quote chars are not special",
			input:    "\u00ABk3os.mode=disk\u00BB k3os.hostname=test",
			expected: []string{"\u00ABk3os.mode=disk\u00BB", "k3os.hostname=test"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := splitCmdlineTokens(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestReadCmdlineIntegration(t *testing.T) {
	oldCmdlineFile := cmdlineFile
	defer func() { cmdlineFile = oldCmdlineFile }()

	t.Run("missing file returns nil", func(t *testing.T) {
		tempDir := t.TempDir()
		cmdlineFile = filepath.Join(tempDir, "nonexistent")

		data, err := readCmdline()
		require.NoError(t, err)
		assert.Nil(t, data)
	})

	t.Run("empty file returns nil", func(t *testing.T) {
		tempDir := t.TempDir()
		path := filepath.Join(tempDir, "cmdline")
		err := os.WriteFile(path, []byte(""), 0o644)
		require.NoError(t, err)
		cmdlineFile = path

		data, err := readCmdline()
		require.NoError(t, err)
		assert.Nil(t, data)
	})

	t.Run("whitespace only returns nil", func(t *testing.T) {
		tempDir := t.TempDir()
		path := filepath.Join(tempDir, "cmdline")
		err := os.WriteFile(path, []byte("   \n  "), 0o644)
		require.NoError(t, err)
		cmdlineFile = path

		data, err := readCmdline()
		require.NoError(t, err)
		assert.Nil(t, data)
	})

	t.Run("parses valid content", func(t *testing.T) {
		tempDir := t.TempDir()
		path := filepath.Join(tempDir, "cmdline")
		err := os.WriteFile(path, []byte(`k3os.hostname=myhost quiet`), 0o644)
		require.NoError(t, err)
		cmdlineFile = path

		data, err := readCmdline()
		require.NoError(t, err)
		require.NotNil(t, data)

		k3os, ok := data["k3os"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "myhost", k3os["hostname"])
		assert.Equal(t, "true", data["quiet"])
	})
}
