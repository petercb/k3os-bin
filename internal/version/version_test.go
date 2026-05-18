package version

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVersion_DefaultValue(t *testing.T) {
	require.NotEmpty(t, Version, "Version should not be empty")
	assert.Equal(t, "HEAD", Version, "Default version should be HEAD when not set by ldflags")
}

func TestVersion_IsString(t *testing.T) {
	original := Version
	defer func() { Version = original }()

	Version = "v1.0.0-test"
	assert.Equal(t, "v1.0.0-test", Version)
}
