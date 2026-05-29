package system

import (
	"os"
	"path/filepath"
	"testing"

	cp "github.com/otiai10/copy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCopyCopyContract validates that the otiai10/copy package correctly copies
// files and directories recursively. This serves as a regression test for the
// dependency upgrade from v1.7.0 to v1.14.1.
func TestCopyCopyContract(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, src string)
		check func(t *testing.T, dst string)
	}{
		{
			name: "copies a single file",
			setup: func(t *testing.T, src string) {
				t.Helper()
				require.NoError(t, os.WriteFile(filepath.Join(src, "file.txt"), []byte("hello"), 0o644))
			},
			check: func(t *testing.T, dst string) {
				t.Helper()
				content, err := os.ReadFile(filepath.Join(dst, "file.txt"))
				require.NoError(t, err)
				assert.Equal(t, "hello", string(content))
			},
		},
		{
			name: "copies nested directories recursively",
			setup: func(t *testing.T, src string) {
				t.Helper()
				nested := filepath.Join(src, "a", "b", "c")
				require.NoError(t, os.MkdirAll(nested, 0o755))
				require.NoError(t, os.WriteFile(filepath.Join(nested, "deep.txt"), []byte("deep"), 0o644))
				require.NoError(t, os.WriteFile(filepath.Join(src, "top.txt"), []byte("top"), 0o644))
			},
			check: func(t *testing.T, dst string) {
				t.Helper()
				content, err := os.ReadFile(filepath.Join(dst, "a", "b", "c", "deep.txt"))
				require.NoError(t, err)
				assert.Equal(t, "deep", string(content))

				content, err = os.ReadFile(filepath.Join(dst, "top.txt"))
				require.NoError(t, err)
				assert.Equal(t, "top", string(content))
			},
		},
		{
			name: "preserves file permissions",
			setup: func(t *testing.T, src string) {
				t.Helper()
				require.NoError(t, os.WriteFile(filepath.Join(src, "exec.sh"), []byte("#!/bin/sh"), 0o755))
			},
			check: func(t *testing.T, dst string) {
				t.Helper()
				info, err := os.Stat(filepath.Join(dst, "exec.sh"))
				require.NoError(t, err)
				assert.Equal(t, os.FileMode(0o755), info.Mode().Perm())
			},
		},
		{
			name: "copies multiple files in same directory",
			setup: func(t *testing.T, src string) {
				t.Helper()
				require.NoError(t, os.WriteFile(filepath.Join(src, "a.txt"), []byte("aaa"), 0o644))
				require.NoError(t, os.WriteFile(filepath.Join(src, "b.txt"), []byte("bbb"), 0o644))
				require.NoError(t, os.WriteFile(filepath.Join(src, "c.txt"), []byte("ccc"), 0o644))
			},
			check: func(t *testing.T, dst string) {
				t.Helper()
				for _, name := range []string{"a.txt", "b.txt", "c.txt"} {
					_, err := os.Stat(filepath.Join(dst, name))
					assert.NoError(t, err, "expected file %s to exist", name)
				}
			},
		},
		{
			name: "copies symlink as symlink (shallow copy)",
			setup: func(t *testing.T, src string) {
				t.Helper()
				require.NoError(t, os.WriteFile(filepath.Join(src, "target.txt"), []byte("content"), 0o644))
				require.NoError(t, os.Symlink("target.txt", filepath.Join(src, "link.txt")))
			},
			check: func(t *testing.T, dst string) {
				t.Helper()
				linkPath := filepath.Join(dst, "link.txt")
				info, err := os.Lstat(linkPath)
				require.NoError(t, err)
				assert.NotZero(t, info.Mode()&os.ModeSymlink, "expected link.txt to be a symlink")

				target, err := os.Readlink(linkPath)
				require.NoError(t, err)
				assert.Equal(t, "target.txt", target)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			src := t.TempDir()
			dst := filepath.Join(t.TempDir(), "dest")

			tc.setup(t, src)

			err := cp.Copy(src, dst)
			require.NoError(t, err)

			tc.check(t, dst)
		})
	}
}

// TestStatComponentVersion tests the StatComponentVersion function which
// dereferences a version symlink for a component.
func TestStatComponentVersion(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(t *testing.T, root string)
		key       string
		alias     VersionName
		wantName  string
		wantError bool
	}{
		{
			name: "resolves current symlink to version directory",
			setup: func(t *testing.T, root string) {
				t.Helper()
				compDir := filepath.Join(root, "k3s")
				require.NoError(t, os.MkdirAll(filepath.Join(compDir, "v1.2.3"), 0o755))
				require.NoError(t, os.Symlink("v1.2.3", filepath.Join(compDir, "current")))
			},
			key:      "k3s",
			alias:    VersionCurrent,
			wantName: "v1.2.3",
		},
		{
			name: "resolves previous symlink to version directory",
			setup: func(t *testing.T, root string) {
				t.Helper()
				compDir := filepath.Join(root, "k3s")
				require.NoError(t, os.MkdirAll(filepath.Join(compDir, "v1.1.0"), 0o755))
				require.NoError(t, os.Symlink("v1.1.0", filepath.Join(compDir, "previous")))
			},
			key:      "k3s",
			alias:    VersionPrevious,
			wantName: "v1.1.0",
		},
		{
			name:      "returns error when alias path does not exist",
			setup:     func(t *testing.T, _ string) { t.Helper() },
			key:       "k3s",
			alias:     VersionCurrent,
			wantError: true,
		},
		{
			name: "returns error when alias is not a directory",
			setup: func(t *testing.T, root string) {
				t.Helper()
				compDir := filepath.Join(root, "k3s")
				require.NoError(t, os.MkdirAll(compDir, 0o755))
				// Create a regular file instead of a symlink to a directory
				require.NoError(t, os.WriteFile(filepath.Join(compDir, "current"), []byte("not a dir"), 0o644))
			},
			key:       "k3s",
			alias:     VersionCurrent,
			wantError: true,
		},
		{
			name: "returns error when alias is a real directory not a symlink",
			setup: func(t *testing.T, root string) {
				t.Helper()
				// Create a real directory named "current" (not a symlink).
				// os.Stat + IsDir passes, but os.Readlink fails with EINVAL.
				compDir := filepath.Join(root, "k3s")
				require.NoError(t, os.MkdirAll(filepath.Join(compDir, "current"), 0o755))
			},
			key:       "k3s",
			alias:     VersionCurrent,
			wantError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			tc.setup(t, root)

			info, err := StatComponentVersion(root, tc.key, tc.alias)
			if tc.wantError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.wantName, info.Name())
			assert.True(t, info.IsDir())
		})
	}
}

// TestCopyComponent tests the CopyComponent function with remount=false to
// avoid needing mount privileges.
func TestCopyComponent(t *testing.T) {
	tests := []struct {
		name           string
		setup          func(t *testing.T, src, dst string)
		key            string
		wantCopied     bool
		wantError      bool
		wantPrevTarget string // expected target of the "previous" symlink after copy
	}{
		{
			name: "copies component and updates symlinks",
			setup: func(t *testing.T, src, dst string) {
				t.Helper()
				// Source: component with version directory and current symlink
				srcComp := filepath.Join(src, "k3s")
				require.NoError(t, os.MkdirAll(filepath.Join(srcComp, "v2.0.0"), 0o755))
				require.NoError(t, os.WriteFile(filepath.Join(srcComp, "v2.0.0", "bin"), []byte("binary"), 0o755))
				require.NoError(t, os.Symlink("v2.0.0", filepath.Join(srcComp, "current")))

				// Destination: existing older version with current symlink
				dstComp := filepath.Join(dst, "k3s")
				require.NoError(t, os.MkdirAll(filepath.Join(dstComp, "v1.0.0"), 0o755))
				require.NoError(t, os.WriteFile(filepath.Join(dstComp, "v1.0.0", "bin"), []byte("old"), 0o644))
				require.NoError(t, os.Symlink("v1.0.0", filepath.Join(dstComp, "current")))
			},
			key:            "k3s",
			wantCopied:     true,
			wantPrevTarget: "v1.0.0",
		},
		{
			name: "skips when source and destination versions match",
			setup: func(t *testing.T, src, dst string) {
				t.Helper()
				// Source
				srcComp := filepath.Join(src, "k3s")
				require.NoError(t, os.MkdirAll(filepath.Join(srcComp, "v1.0.0"), 0o755))
				require.NoError(t, os.Symlink("v1.0.0", filepath.Join(srcComp, "current")))

				// Destination has same version
				dstComp := filepath.Join(dst, "k3s")
				require.NoError(t, os.MkdirAll(filepath.Join(dstComp, "v1.0.0"), 0o755))
				require.NoError(t, os.Symlink("v1.0.0", filepath.Join(dstComp, "current")))
			},
			key:        "k3s",
			wantCopied: false,
		},
		{
			name: "returns error when source component does not exist",
			setup: func(t *testing.T, _, _ string) {
				t.Helper()
				// No component set up in source
			},
			key:       "k3s",
			wantError: true,
		},
		{
			name: "copies to destination with no existing version",
			setup: func(t *testing.T, src, dst string) {
				t.Helper()
				// Source with version
				srcComp := filepath.Join(src, "k3s")
				require.NoError(t, os.MkdirAll(filepath.Join(srcComp, "v1.5.0"), 0o755))
				require.NoError(t, os.WriteFile(filepath.Join(srcComp, "v1.5.0", "data.txt"), []byte("data"), 0o644))
				require.NoError(t, os.Symlink("v1.5.0", filepath.Join(srcComp, "current")))

				// Destination: component directory exists but no current symlink
				dstComp := filepath.Join(dst, "k3s")
				require.NoError(t, os.MkdirAll(dstComp, 0o755))
			},
			key:        "k3s",
			wantCopied: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			src := t.TempDir()
			dst := t.TempDir()
			tc.setup(t, src, dst)

			copied, err := CopyComponent(src, dst, false, tc.key)
			if tc.wantError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.wantCopied, copied)

			if copied {
				// Verify the current symlink was updated in the destination
				dstCurrPath := filepath.Join(dst, tc.key, "current")
				target, err := os.Readlink(dstCurrPath)
				require.NoError(t, err)

				// Determine what version was in source
				srcCurrPath := filepath.Join(src, tc.key, "current")
				srcTarget, err := os.Readlink(srcCurrPath)
				require.NoError(t, err)

				assert.Equal(t, srcTarget, target)

				// Verify the version directory was copied
				versionDir := filepath.Join(dst, tc.key, srcTarget)
				info, err := os.Stat(versionDir)
				require.NoError(t, err)
				assert.True(t, info.IsDir())

				// Verify the previous symlink was rotated correctly
				if tc.wantPrevTarget != "" {
					dstPrevPath := filepath.Join(dst, tc.key, "previous")
					prevInfo, err := os.Lstat(dstPrevPath)
					require.NoError(t, err)
					assert.NotZero(t, prevInfo.Mode()&os.ModeSymlink, "expected previous to be a symlink")

					prevTarget, err := os.Readlink(dstPrevPath)
					require.NoError(t, err)
					assert.Equal(t, tc.wantPrevTarget, prevTarget)
				}
			}
		})
	}
}
