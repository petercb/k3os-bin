//go:build linux

package devpopulate

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPopulateDev_CreatesSymlinksForLabel(t *testing.T) {
	t.Parallel()

	devDir := t.TempDir()
	sysBlockDir := t.TempDir()

	// Simulate a block device entry in sysfs.
	require.NoError(t, os.MkdirAll(filepath.Join(sysBlockDir, "vda"), 0o755))

	// Create a fake device node (regular file since we can't mknod in tests).
	devNode := filepath.Join(devDir, "vda")
	require.NoError(t, os.WriteFile(devNode, []byte{}, 0o644))

	// Use a mock prober that returns a label and UUID.
	prober := &mockProber{
		results: map[string]probeResult{
			devNode: {label: "K3OS_STATE", uuid: "abcd-1234"},
		},
	}

	opts := Options{
		DevDir:      devDir,
		SysBlockDir: sysBlockDir,
		Prober:      prober,
		CreateNodes: false, // Skip mknod in unit tests.
	}

	err := PopulateDev(opts)
	require.NoError(t, err)

	// Verify /dev/disk/by-label/K3OS_STATE symlink was created.
	labelLink := filepath.Join(devDir, "disk", "by-label", "K3OS_STATE")
	target, err := os.Readlink(labelLink)
	require.NoError(t, err)
	assert.Equal(t, "../../vda", target)

	// Verify /dev/disk/by-uuid/abcd-1234 symlink was created.
	uuidLink := filepath.Join(devDir, "disk", "by-uuid", "abcd-1234")
	target, err = os.Readlink(uuidLink)
	require.NoError(t, err)
	assert.Equal(t, "../../vda", target)
}

func TestPopulateDev_SkipsDevicesWithoutLabelsOrUUIDs(t *testing.T) {
	t.Parallel()

	devDir := t.TempDir()
	sysBlockDir := t.TempDir()

	// Simulate a block device entry.
	require.NoError(t, os.MkdirAll(filepath.Join(sysBlockDir, "vda"), 0o755))
	devNode := filepath.Join(devDir, "vda")
	require.NoError(t, os.WriteFile(devNode, []byte{}, 0o644))

	// Prober returns no label or UUID.
	prober := &mockProber{
		results: map[string]probeResult{
			devNode: {label: "", uuid: ""},
		},
	}

	opts := Options{
		DevDir:      devDir,
		SysBlockDir: sysBlockDir,
		Prober:      prober,
		CreateNodes: false,
	}

	err := PopulateDev(opts)
	require.NoError(t, err)

	// No symlinks should be created.
	_, err = os.Stat(filepath.Join(devDir, "disk", "by-label"))
	assert.True(t, os.IsNotExist(err), "by-label dir should not exist when no labels found")
}

func TestPopulateDev_HandlesMultipleDevices(t *testing.T) {
	t.Parallel()

	devDir := t.TempDir()
	sysBlockDir := t.TempDir()

	// Simulate multiple block device entries.
	for _, name := range []string{"vda", "vdb", "vdc"} {
		require.NoError(t, os.MkdirAll(filepath.Join(sysBlockDir, name), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(devDir, name), []byte{}, 0o644))
	}

	prober := &mockProber{
		results: map[string]probeResult{
			filepath.Join(devDir, "vda"): {label: "BOOT", uuid: "1111-2222"},
			filepath.Join(devDir, "vdb"): {label: "K3OS_STATE", uuid: "3333-4444"},
			filepath.Join(devDir, "vdc"): {label: "", uuid: "5555-6666"}, // label-less
		},
	}

	opts := Options{
		DevDir:      devDir,
		SysBlockDir: sysBlockDir,
		Prober:      prober,
		CreateNodes: false,
	}

	err := PopulateDev(opts)
	require.NoError(t, err)

	// Check label symlinks.
	target, err := os.Readlink(filepath.Join(devDir, "disk", "by-label", "BOOT"))
	require.NoError(t, err)
	assert.Equal(t, "../../vda", target)

	target, err = os.Readlink(filepath.Join(devDir, "disk", "by-label", "K3OS_STATE"))
	require.NoError(t, err)
	assert.Equal(t, "../../vdb", target)

	// vdc has no label, so no by-label symlink for it.
	_, err = os.Lstat(filepath.Join(devDir, "disk", "by-label", "vdc"))
	assert.True(t, os.IsNotExist(err))

	// Check UUID symlinks for all three.
	target, err = os.Readlink(filepath.Join(devDir, "disk", "by-uuid", "1111-2222"))
	require.NoError(t, err)
	assert.Equal(t, "../../vda", target)

	target, err = os.Readlink(filepath.Join(devDir, "disk", "by-uuid", "3333-4444"))
	require.NoError(t, err)
	assert.Equal(t, "../../vdb", target)

	target, err = os.Readlink(filepath.Join(devDir, "disk", "by-uuid", "5555-6666"))
	require.NoError(t, err)
	assert.Equal(t, "../../vdc", target)
}

func TestPopulateDev_HandlesProberErrors(t *testing.T) {
	t.Parallel()

	devDir := t.TempDir()
	sysBlockDir := t.TempDir()

	// Simulate a block device entry.
	require.NoError(t, os.MkdirAll(filepath.Join(sysBlockDir, "vda"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(devDir, "vda"), []byte{}, 0o644))

	// Prober returns an error for the device.
	prober := &mockProber{
		results: map[string]probeResult{},
		errDevs: map[string]error{
			filepath.Join(devDir, "vda"): os.ErrPermission,
		},
	}

	opts := Options{
		DevDir:      devDir,
		SysBlockDir: sysBlockDir,
		Prober:      prober,
		CreateNodes: false,
	}

	// Should not return an error; individual probe failures are logged and skipped.
	err := PopulateDev(opts)
	require.NoError(t, err)
}

func TestPopulateDev_EmptySysBlockDir(t *testing.T) {
	t.Parallel()

	devDir := t.TempDir()
	sysBlockDir := t.TempDir()

	prober := &mockProber{results: map[string]probeResult{}}

	opts := Options{
		DevDir:      devDir,
		SysBlockDir: sysBlockDir,
		Prober:      prober,
		CreateNodes: false,
	}

	err := PopulateDev(opts)
	require.NoError(t, err)
}

func TestPopulateDev_NonExistentSysBlockDir(t *testing.T) {
	t.Parallel()

	devDir := t.TempDir()

	prober := &mockProber{results: map[string]probeResult{}}

	opts := Options{
		DevDir:      devDir,
		SysBlockDir: "/nonexistent/sys/class/block",
		Prober:      prober,
		CreateNodes: false,
	}

	err := PopulateDev(opts)
	require.Error(t, err)
}

func TestPopulateDev_IncludesPartitions(t *testing.T) {
	t.Parallel()

	devDir := t.TempDir()
	sysBlockDir := t.TempDir()

	// Simulate a disk with a partition (vda/vda1 structure in sysfs).
	require.NoError(t, os.MkdirAll(filepath.Join(sysBlockDir, "vda", "vda1"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(devDir, "vda"), []byte{}, 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(devDir, "vda1"), []byte{}, 0o644))

	prober := &mockProber{
		results: map[string]probeResult{
			filepath.Join(devDir, "vda"):  {label: "", uuid: ""},
			filepath.Join(devDir, "vda1"): {label: "K3OS_STATE", uuid: "aaaa-bbbb"},
		},
	}

	opts := Options{
		DevDir:      devDir,
		SysBlockDir: sysBlockDir,
		Prober:      prober,
		CreateNodes: false,
	}

	err := PopulateDev(opts)
	require.NoError(t, err)

	// The partition's label symlink should exist.
	target, err := os.Readlink(filepath.Join(devDir, "disk", "by-label", "K3OS_STATE"))
	require.NoError(t, err)
	assert.Equal(t, "../../vda1", target)

	target, err = os.Readlink(filepath.Join(devDir, "disk", "by-uuid", "aaaa-bbbb"))
	require.NoError(t, err)
	assert.Equal(t, "../../vda1", target)
}

func TestPopulateDev_NvmePartitionNaming(t *testing.T) {
	t.Parallel()

	devDir := t.TempDir()
	sysBlockDir := t.TempDir()

	// NVMe uses "p" separator: nvme0n1 -> nvme0n1p1
	require.NoError(t, os.MkdirAll(filepath.Join(sysBlockDir, "nvme0n1", "nvme0n1p1"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(devDir, "nvme0n1"), []byte{}, 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(devDir, "nvme0n1p1"), []byte{}, 0o644))

	prober := &mockProber{
		results: map[string]probeResult{
			filepath.Join(devDir, "nvme0n1"):   {label: "", uuid: ""},
			filepath.Join(devDir, "nvme0n1p1"): {label: "ROOT", uuid: "dead-beef"},
		},
	}

	opts := Options{
		DevDir:      devDir,
		SysBlockDir: sysBlockDir,
		Prober:      prober,
		CreateNodes: false,
	}

	err := PopulateDev(opts)
	require.NoError(t, err)

	target, err := os.Readlink(filepath.Join(devDir, "disk", "by-label", "ROOT"))
	require.NoError(t, err)
	assert.Equal(t, "../../nvme0n1p1", target)
}

func TestPopulateDev_CreateNodesWithSysfsDevFile(t *testing.T) {
	t.Parallel()

	devDir := t.TempDir()
	sysBlockDir := t.TempDir()

	// Simulate sysfs entry with dev file (8:0 = sda).
	sysEntry := filepath.Join(sysBlockDir, "sda")
	require.NoError(t, os.MkdirAll(sysEntry, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(sysEntry, "dev"), []byte("8:0\n"), 0o444))

	// Don't pre-create the device node; CreateNodes should attempt mknod.
	// In non-root tests, mknod will fail but we test the logic path.
	prober := &mockProber{results: map[string]probeResult{}}

	opts := Options{
		DevDir:      devDir,
		SysBlockDir: sysBlockDir,
		Prober:      prober,
		CreateNodes: true,
	}

	// This won't create the actual node (EPERM in non-root) but should not error out.
	err := PopulateDev(opts)
	require.NoError(t, err)
}

func TestParseMajorMinor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		major   uint32
		minor   uint32
		wantErr bool
	}{
		{name: "valid", input: "8:0", major: 8, minor: 0},
		{name: "large_minor", input: "259:1", major: 259, minor: 1},
		{name: "empty", input: "", wantErr: true},
		{name: "no_colon", input: "80", wantErr: true},
		{name: "bad_major", input: "abc:0", wantErr: true},
		{name: "bad_minor", input: "8:xyz", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			major, minor, err := parseMajorMinor(tt.input)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.major, major)
				assert.Equal(t, tt.minor, minor)
			}
		})
	}
}

func TestIsPartition(t *testing.T) {
	t.Parallel()

	tests := []struct {
		parent string
		child  string
		want   bool
	}{
		{"vda", "vda1", true},
		{"vda", "vda12", true},
		{"sda", "sda1", true},
		{"nvme0n1", "nvme0n1p1", true},
		{"nvme0n1", "nvme0n1p12", true},
		{"vda", "vda", false},
		{"vda", "vdb1", false},
		{"vda", "vd", false},
		{"nvme0n1", "nvme0n1x1", false},
	}

	for _, tt := range tests {
		t.Run(tt.parent+"_"+tt.child, func(t *testing.T) {
			t.Parallel()
			got := isPartition(tt.parent, tt.child)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDefaultOptions(t *testing.T) {
	t.Parallel()

	opts := DefaultOptions()
	assert.Equal(t, "/dev", opts.DevDir)
	assert.Equal(t, "/sys/class/block", opts.SysBlockDir)
	assert.NotNil(t, opts.Prober)
	assert.True(t, opts.CreateNodes)
}

func TestPopulateDev_IdempotentSymlinks(t *testing.T) {
	t.Parallel()

	devDir := t.TempDir()
	sysBlockDir := t.TempDir()

	require.NoError(t, os.MkdirAll(filepath.Join(sysBlockDir, "vda"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(devDir, "vda"), []byte{}, 0o644))

	prober := &mockProber{
		results: map[string]probeResult{
			filepath.Join(devDir, "vda"): {label: "TEST", uuid: "1234"},
		},
	}

	opts := Options{
		DevDir:      devDir,
		SysBlockDir: sysBlockDir,
		Prober:      prober,
		CreateNodes: false,
	}

	// Run twice — should be idempotent.
	require.NoError(t, PopulateDev(opts))
	require.NoError(t, PopulateDev(opts))

	target, err := os.Readlink(filepath.Join(devDir, "disk", "by-label", "TEST"))
	require.NoError(t, err)
	assert.Equal(t, "../../vda", target)
}

// mockProber is a test double for the Prober interface.
type mockProber struct {
	results map[string]probeResult
	errDevs map[string]error
}

type probeResult struct {
	label string
	uuid  string
}

func (m *mockProber) Probe(devPath string) (label, uuid string, err error) {
	if m.errDevs != nil {
		if e, ok := m.errDevs[devPath]; ok {
			return "", "", e
		}
	}
	r, ok := m.results[devPath]
	if !ok {
		return "", "", nil
	}
	return r.label, r.uuid, nil
}
