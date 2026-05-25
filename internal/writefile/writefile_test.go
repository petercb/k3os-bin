package writefile

import (
	"errors"
	"os"
	"testing"

	"github.com/petercb/k3os-bin/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// ensureDirectoryExists
// ---------------------------------------------------------------------------

func TestEnsureDirectoryExists(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		setupMock   func(fs *MockFileSystem)
		wantErr     bool
		errContains string
	}{
		{
			name: "dir exists and is dir",
			setupMock: func(fs *MockFileSystem) {
				fs.On("Stat", "/etc/foo").Return(&mockFileInfo{isDir: true}, nil)
			},
			wantErr: false,
		},
		{
			name: "path exists but not dir",
			setupMock: func(fs *MockFileSystem) {
				fs.On("Stat", "/etc/foo").Return(&mockFileInfo{isDir: false}, nil)
			},
			wantErr:     true,
			errContains: "is not a directory",
		},
		{
			name: "Stat error triggers MkdirAll",
			setupMock: func(fs *MockFileSystem) {
				fs.On("Stat", "/etc/foo").Return(nil, os.ErrNotExist)
				fs.On("MkdirAll", "/etc/foo", os.FileMode(0o755)).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "MkdirAll error propagated",
			setupMock: func(fs *MockFileSystem) {
				fs.On("Stat", "/etc/foo").Return(nil, os.ErrNotExist)
				fs.On("MkdirAll", "/etc/foo", os.FileMode(0o755)).Return(errors.New("permission denied"))
			},
			wantErr:     true,
			errContains: "permission denied",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			fs := &MockFileSystem{}
			tc.setupMock(fs)

			err := ensureDirectoryExists(fs, "/etc/foo")

			if tc.wantErr {
				require.Error(t, err)
				if tc.errContains != "" {
					assert.Contains(t, err.Error(), tc.errContains)
				}
			} else {
				require.NoError(t, err)
			}

			fs.AssertExpectations(t)
		})
	}
}

// ---------------------------------------------------------------------------
// WriteFile
// ---------------------------------------------------------------------------

func TestWriteFile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		file        config.File
		setupMock   func(fs *MockFileSystem, cmd *MockCommandRunner)
		wantPath    string
		wantErr     bool
		errContains string
	}{
		{
			name: "full success path",
			file: config.File{
				Path:    "/etc/foo",
				Content: "hello world",
			},
			setupMock: func(fs *MockFileSystem, _ *MockCommandRunner) {
				// ensureDirectoryExists: Stat succeeds, dir exists
				fs.On("Stat", "/etc").Return(&mockFileInfo{isDir: true}, nil)

				// CreateTemp in the same directory
				tmpFile := &MockFile{}
				tmpFile.On("Close").Return(nil)
				tmpFile.On("Name").Return("/etc/wfs-temp-001")
				fs.On("CreateTemp", "/etc", "wfs-temp").Return(tmpFile, nil)

				// Chmod with default permission 0o644
				fs.On("Chmod", "/etc/wfs-temp-001", os.FileMode(0o644)).Return(nil)

				// Rename temp file to final path
				fs.On("Rename", "/etc/wfs-temp-001", "/etc/foo").Return(nil)
			},
			wantPath: "/etc/foo",
			wantErr:  false,
		},
		{
			name: "encoding not empty returns error immediately",
			file: config.File{
				Path:     "/etc/foo",
				Content:  "hello",
				Encoding: "b64",
			},
			setupMock: func(_ *MockFileSystem, _ *MockCommandRunner) {
				// No calls expected
			},
			wantPath:    "",
			wantErr:     true,
			errContains: "unable to write file with encoding",
		},
		{
			name: "CreateTemp error",
			file: config.File{
				Path:    "/etc/foo",
				Content: "data",
			},
			setupMock: func(fs *MockFileSystem, _ *MockCommandRunner) {
				fs.On("Stat", "/etc").Return(&mockFileInfo{isDir: true}, nil)
				fs.On("CreateTemp", "/etc", "wfs-temp").Return(nil, errors.New("no space"))
			},
			wantPath:    "",
			wantErr:     true,
			errContains: "no space",
		},
		{
			name: "Chmod error",
			file: config.File{
				Path:    "/etc/bar",
				Content: "data",
			},
			setupMock: func(fs *MockFileSystem, _ *MockCommandRunner) {
				fs.On("Stat", "/etc").Return(&mockFileInfo{isDir: true}, nil)

				tmpFile := &MockFile{}
				tmpFile.On("Close").Return(nil)
				tmpFile.On("Name").Return("/etc/wfs-temp-002")
				fs.On("CreateTemp", "/etc", "wfs-temp").Return(tmpFile, nil)

				fs.On("Chmod", "/etc/wfs-temp-002", os.FileMode(0o644)).Return(errors.New("chmod failed"))
			},
			wantPath:    "",
			wantErr:     true,
			errContains: "chmod failed",
		},
		{
			name: "Rename error",
			file: config.File{
				Path:    "/etc/baz",
				Content: "data",
			},
			setupMock: func(fs *MockFileSystem, _ *MockCommandRunner) {
				fs.On("Stat", "/etc").Return(&mockFileInfo{isDir: true}, nil)

				tmpFile := &MockFile{}
				tmpFile.On("Close").Return(nil)
				tmpFile.On("Name").Return("/etc/wfs-temp-003")
				fs.On("CreateTemp", "/etc", "wfs-temp").Return(tmpFile, nil)

				fs.On("Chmod", "/etc/wfs-temp-003", os.FileMode(0o644)).Return(nil)
				fs.On("Rename", "/etc/wfs-temp-003", "/etc/baz").Return(errors.New("rename failed"))
			},
			wantPath:    "",
			wantErr:     true,
			errContains: "rename failed",
		},
		{
			name: "owner field triggers cmd.Run chown",
			file: config.File{
				Path:    "/etc/owned",
				Content: "data",
				Owner:   "root:root",
			},
			setupMock: func(fs *MockFileSystem, cmd *MockCommandRunner) {
				fs.On("Stat", "/etc").Return(&mockFileInfo{isDir: true}, nil)

				tmpFile := &MockFile{}
				tmpFile.On("Close").Return(nil)
				tmpFile.On("Name").Return("/etc/wfs-temp-004")
				fs.On("CreateTemp", "/etc", "wfs-temp").Return(tmpFile, nil)

				fs.On("Chmod", "/etc/wfs-temp-004", os.FileMode(0o644)).Return(nil)

				cmd.On("Run", "chown", "root:root", "/etc/wfs-temp-004").Return(nil)

				fs.On("Rename", "/etc/wfs-temp-004", "/etc/owned").Return(nil)
			},
			wantPath: "/etc/owned",
			wantErr:  false,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			fs := &MockFileSystem{}
			cmd := &MockCommandRunner{}
			tc.setupMock(fs, cmd)

			p, err := WriteFile(&tc.file, "/", fs, cmd)

			if tc.wantErr {
				require.Error(t, err)
				if tc.errContains != "" {
					assert.Contains(t, err.Error(), tc.errContains)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.wantPath, p)
			}

			fs.AssertExpectations(t)
			cmd.AssertExpectations(t)
		})
	}
}

// ---------------------------------------------------------------------------
// WriteFiles
// ---------------------------------------------------------------------------

func TestWriteFiles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		files     []config.File
		setupMock func(fs *MockFileSystem, cmd *MockCommandRunner)
	}{
		{
			name:  "empty list no-op",
			files: []config.File{},
			setupMock: func(_ *MockFileSystem, _ *MockCommandRunner) {
				// No calls expected
			},
		},
		{
			name: "multiple entries with one invalid encoding logs error and continues",
			files: []config.File{
				{
					Path:     "/etc/first",
					Content:  "data",
					Encoding: "unsupported_encoding",
				},
				{
					Path:    "/etc/second",
					Content: "good data",
				},
			},
			setupMock: func(fs *MockFileSystem, _ *MockCommandRunner) {
				// First entry has invalid encoding so DecodeContent returns error.
				// WriteFiles logs error and continues to the second entry.

				// Second entry should proceed normally
				fs.On("Stat", "/etc").Return(&mockFileInfo{isDir: true}, nil)

				tmpFile := &MockFile{}
				tmpFile.On("Close").Return(nil)
				tmpFile.On("Name").Return("/etc/wfs-temp-005")
				fs.On("CreateTemp", "/etc", "wfs-temp").Return(tmpFile, nil)

				fs.On("Chmod", "/etc/wfs-temp-005", os.FileMode(0o644)).Return(nil)
				fs.On("Rename", "/etc/wfs-temp-005", "/etc/second").Return(nil)
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			fs := &MockFileSystem{}
			cmd := &MockCommandRunner{}
			tc.setupMock(fs, cmd)

			cfg := &config.CloudConfig{}
			cfg.WriteFiles = tc.files

			// WriteFiles does not return errors -- it logs them
			WriteFiles(cfg, fs, cmd)

			fs.AssertExpectations(t)
			cmd.AssertExpectations(t)
		})
	}
}
