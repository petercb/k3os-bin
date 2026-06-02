package shadow

import (
	"errors"
	"strings"
	"testing"

	"github.com/go-crypt/crypt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const testShadowContent = "root:!:19000:0:99999:7:::\nrancher:$6$oldsalt$oldhash:19000:0:99999:7:::\nnobody:!:19000:0:99999:7:::\n"

func TestSetPassword_PlaintextIsHashedAndWritten(t *testing.T) {
	t.Parallel()

	tmpFile := &MockFile{name: "/etc/shadow.tmp123"}
	fs := &MockFileSystem{}
	fs.On("ReadFile", "/etc/shadow").Return([]byte(testShadowContent), nil)
	fs.On("CreateTemp", "/etc", "shadow.*").Return(tmpFile, nil)
	fs.On("Chmod", "/etc/shadow.tmp123", mock.Anything).Return(nil)
	fs.On("Rename", "/etc/shadow.tmp123", "/etc/shadow").Return(nil)

	s := Setter{}
	err := s.SetPassword(fs, "rancher", "hunter2")

	require.NoError(t, err)
	fs.AssertExpectations(t)

	// Verify the written content contains a valid SHA-512 hash for rancher
	written := tmpFile.Written()
	lines := strings.Split(written, "\n")
	found := false
	for _, line := range lines {
		fields := strings.SplitN(line, ":", 3)
		if len(fields) >= 2 && fields[0] == "rancher" {
			assert.True(t, strings.HasPrefix(fields[1], "$6$"), "hash should start with $6$")
			found = true
			break
		}
	}
	assert.True(t, found, "rancher entry should be present in output")
}

func TestSetPassword_PrehashedUsedDirectly(t *testing.T) {
	t.Parallel()

	preHashed := "$6$rounds=4096$saltsalt$hashedvalue"

	tmpFile := &MockFile{name: "/etc/shadow.tmp123"}
	fs := &MockFileSystem{}
	fs.On("ReadFile", "/etc/shadow").Return([]byte(testShadowContent), nil)
	fs.On("CreateTemp", "/etc", "shadow.*").Return(tmpFile, nil)
	fs.On("Chmod", "/etc/shadow.tmp123", mock.Anything).Return(nil)
	fs.On("Rename", "/etc/shadow.tmp123", "/etc/shadow").Return(nil)

	s := Setter{}
	err := s.SetPassword(fs, "rancher", preHashed)

	require.NoError(t, err)
	fs.AssertExpectations(t)

	// Verify the pre-hashed value was written directly
	written := tmpFile.Written()
	lines := strings.Split(written, "\n")
	for _, line := range lines {
		fields := strings.SplitN(line, ":", 3)
		if len(fields) >= 2 && fields[0] == "rancher" {
			assert.Equal(t, preHashed, fields[1])
			break
		}
	}
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

func TestSetPassword_CreateTempErrorIsPropagated(t *testing.T) {
	t.Parallel()

	fs := &MockFileSystem{}
	fs.On("ReadFile", "/etc/shadow").Return([]byte(testShadowContent), nil)
	fs.On("CreateTemp", "/etc", "shadow.*").Return(nil, errors.New("no space"))

	s := Setter{}
	err := s.SetPassword(fs, "rancher", "hunter2")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no space")
	fs.AssertExpectations(t)
}

func TestSetPassword_RenameErrorCleansUp(t *testing.T) {
	t.Parallel()

	tmpFile := &MockFile{name: "/etc/shadow.tmp123"}
	fs := &MockFileSystem{}
	fs.On("ReadFile", "/etc/shadow").Return([]byte(testShadowContent), nil)
	fs.On("CreateTemp", "/etc", "shadow.*").Return(tmpFile, nil)
	fs.On("Chmod", "/etc/shadow.tmp123", mock.Anything).Return(nil)
	fs.On("Rename", "/etc/shadow.tmp123", "/etc/shadow").Return(errors.New("cross-device link"))
	fs.On("Remove", "/etc/shadow.tmp123").Return(nil)

	s := Setter{}
	err := s.SetPassword(fs, "rancher", "hunter2")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "cross-device link")
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

func TestHashPassword_RoundTripVerification(t *testing.T) {
	t.Parallel()

	password := "correct horse battery staple"

	hash, err := HashPassword(password)
	require.NoError(t, err)

	// Verify the hash authenticates against the original password
	valid, err := crypt.CheckPassword(password, hash)
	require.NoError(t, err)
	assert.True(t, valid, "hash should verify against the original password")

	// Verify the hash does NOT authenticate against a wrong password
	valid, err = crypt.CheckPassword("wrong password", hash)
	require.NoError(t, err)
	assert.False(t, valid, "hash should not verify against a wrong password")
}
