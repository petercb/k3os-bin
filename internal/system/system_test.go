package system

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRootPath(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected string
	}{
		{
			name:     "no arguments",
			input:    []string{},
			expected: "/k3os/system",
		},
		{
			name:     "single argument",
			input:    []string{"foo"},
			expected: "/k3os/system/foo",
		},
		{
			name:     "multiple arguments",
			input:    []string{"foo", "bar"},
			expected: "/k3os/system/foo/bar",
		},
		{
			name:     "absolute path argument",
			input:    []string{"/foo"},
			expected: "/k3os/system/foo",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := RootPath(tc.input...)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestDataPath(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected string
	}{
		{
			name:     "no arguments",
			input:    []string{},
			expected: "/k3os/data",
		},
		{
			name:     "single argument",
			input:    []string{"foo"},
			expected: "/k3os/data/foo",
		},
		{
			name:     "multiple arguments",
			input:    []string{"foo", "bar"},
			expected: "/k3os/data/foo/bar",
		},
		{
			name:     "absolute path argument",
			input:    []string{"/foo"},
			expected: "/k3os/data/foo",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := DataPath(tc.input...)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestLocalPath(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected string
	}{
		{
			name:     "no arguments",
			input:    []string{},
			expected: "/var/lib/rancher/k3os",
		},
		{
			name:     "single argument",
			input:    []string{"foo"},
			expected: "/var/lib/rancher/k3os/foo",
		},
		{
			name:     "multiple arguments",
			input:    []string{"foo", "bar"},
			expected: "/var/lib/rancher/k3os/foo/bar",
		},
		{
			name:     "absolute path argument",
			input:    []string{"/foo"},
			expected: "/var/lib/rancher/k3os/foo",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := LocalPath(tc.input...)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestStatePath(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected string
	}{
		{
			name:     "no arguments",
			input:    []string{},
			expected: "/run/k3os",
		},
		{
			name:     "single argument",
			input:    []string{"foo"},
			expected: "/run/k3os/foo",
		},
		{
			name:     "multiple arguments",
			input:    []string{"foo", "bar"},
			expected: "/run/k3os/foo/bar",
		},
		{
			name:     "absolute path argument",
			input:    []string{"/foo"},
			expected: "/run/k3os/foo",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := StatePath(tc.input...)
			assert.Equal(t, tc.expected, result)
		})
	}
}
