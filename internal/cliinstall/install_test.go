package cliinstall

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunCCApply_DelegatesToCCApplyFunc(t *testing.T) {
	orig := ccApplyFunc
	t.Cleanup(func() { ccApplyFunc = orig })

	var called bool
	ccApplyFunc = func() error {
		called = true
		return nil
	}

	err := runCCApply()
	require.NoError(t, err)
	assert.True(t, called, "runCCApply should delegate to ccApplyFunc")
}

func TestRunCCApply_PropagatesError(t *testing.T) {
	orig := ccApplyFunc
	t.Cleanup(func() { ccApplyFunc = orig })

	expectedErr := assert.AnError
	ccApplyFunc = func() error {
		return expectedErr
	}

	err := runCCApply()
	assert.ErrorIs(t, err, expectedErr)
}
