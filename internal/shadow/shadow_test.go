package shadow

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const testShadowContent = "root:!:19000:0:99999:7:::\nrancher:$6$oldsalt$oldhash:19000:0:99999:7:::\nnobody:!:19000:0:99999:7:::\n"

func TestSetPassword_PlaintextIsHashedAndWritten(t *testing.T) {
	t.Parallel()

	fs := &MockFileSystem{}
	fs.On("ReadFile", "/etc/shadow").Return([]byte(testShadowContent), nil)
	fs.On("WriteFile", "/etc/shadow", mock.MatchedBy(func(data []byte) bool {
		content := string(data)
		lines := strings.Split(content, "\n")
		for _, line := range lines {
			fields := strings.SplitN(line, ":", 3)
			if len(fields) >= 2 && fields[0] == "rancher" {
				// Must be a SHA-512 crypt hash
				return strings.HasPrefix(fields[1], "$6$")
			}
		}
		return false
	}), os.FileMode(0o640)).Return(nil)

	s := Setter{}
	err := s.SetPassword(fs, "rancher", "hunter2")

	require.NoError(t, err)
	fs.AssertExpectations(t)
}

func TestSetPassword_PrehashedUsedDirectly(t *testing.T) {
	t.Parallel()

	preHashed := "$6$rounds=4096$saltsalt$hashedvalue"

	fs := &MockFileSystem{}
	fs.On("ReadFile", "/etc/shadow").Return([]byte(testShadowContent), nil)
	fs.On("WriteFile", "/etc/shadow", mock.MatchedBy(func(data []byte) bool {
		content := string(data)
		lines := strings.Split(content, "\n")
		for _, line := range lines {
			fields := strings.SplitN(line, ":", 3)
			if len(fields) >= 2 && fields[0] == "rancher" {
				return fields[1] == preHashed
			}
		}
		return false
	}), os.FileMode(0o640)).Return(nil)

	s := Setter{}
	err := s.SetPassword(fs, "rancher", preHashed)

	require.NoError(t, err)
	fs.AssertExpectations(t)
}

func TestSetPassword_UserNotFoundReturnsError(t *testing.T) {
	t.Parallel()

	shadowContent := "root:!:19000:0:99999:7:::\nnobody:!:19000:0:99999:7:::\n"

	fs := &MockFileSystem{}
	fs.On("ReadFile", "/etc/shadow").Return([]byte(shadowContent), nil)

	s := Setter{}
	err := s.SetPassword(fs, "rancher", "hunter2")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "rancher")
	assert.Contains(t, err.Error(), "not found")
	fs.AssertExpectations(t)
}

func TestSetPassword_EmptyPasswordIsNoOp(t *testing.T) {
	t.Parallel()

	fs := &MockFileSystem{}
	// No expectations set - no calls should be made

	s := Setter{}
	err := s.SetPassword(fs, "rancher", "")

	require.NoError(t, err)
	fs.AssertExpectations(t)
}

func TestSetPassword_ReadFileErrorIsPropagated(t *testing.T) {
	t.Parallel()

	fs := &MockFileSystem{}
	fs.On("ReadFile", "/etc/shadow").Return(nil, errors.New("permission denied"))

	s := Setter{}
	err := s.SetPassword(fs, "rancher", "hunter2")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "permission denied")
	fs.AssertExpectations(t)
}

func TestSetPassword_WriteFileErrorIsPropagated(t *testing.T) {
	t.Parallel()

	fs := &MockFileSystem{}
	fs.On("ReadFile", "/etc/shadow").Return([]byte(testShadowContent), nil)
	fs.On("WriteFile", "/etc/shadow", mock.Anything, os.FileMode(0o640)).Return(errors.New("disk full"))

	s := Setter{}
	err := s.SetPassword(fs, "rancher", "hunter2")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "disk full")
	fs.AssertExpectations(t)
}

func TestHashPassword_ReturnsValidSHA512Hash(t *testing.T) {
	t.Parallel()

	hash, err := HashPassword("mypassword")

	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(hash, "$6$"), "hash should start with $6$ prefix")
}

func TestHashPassword_DifferentCallsProduceDifferentSalts(t *testing.T) {
	t.Parallel()

	hash1, err := HashPassword("password")
	require.NoError(t, err)

	hash2, err := HashPassword("password")
	require.NoError(t, err)

	// Different salts means different hashes even for same password
	assert.NotEqual(t, hash1, hash2)
}
