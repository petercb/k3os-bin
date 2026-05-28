package cc

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/petercb/k3os-bin/internal/config"
	"github.com/petercb/k3os-bin/internal/system"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// runApplies
// ---------------------------------------------------------------------------

func TestRunApplies_AllSucceed(t *testing.T) {
	t.Parallel()

	a := newTestApplier(nil, nil, nil, nil, nil)

	succeedA := func(_ *config.CloudConfig) error { return nil }
	succeedB := func(_ *config.CloudConfig) error { return nil }
	succeedC := func(_ *config.CloudConfig) error { return nil }

	err := a.runApplies(&config.CloudConfig{}, succeedA, succeedB, succeedC)
	require.NoError(t, err)
}

func TestRunApplies_SingleError(t *testing.T) {
	t.Parallel()

	a := newTestApplier(nil, nil, nil, nil, nil)

	succeedA := func(_ *config.CloudConfig) error { return nil }
	failB := func(_ *config.CloudConfig) error { return errors.New("applier B failed") }
	succeedC := func(_ *config.CloudConfig) error { return nil }

	err := a.runApplies(&config.CloudConfig{}, succeedA, failB, succeedC)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "applier B failed")
}

func TestRunApplies_MultipleErrors_AllRun(t *testing.T) {
	t.Parallel()

	a := newTestApplier(nil, nil, nil, nil, nil)

	var called [3]bool

	failFirst := func(_ *config.CloudConfig) error {
		called[0] = true
		return errors.New("first failed")
	}
	succeedSecond := func(_ *config.CloudConfig) error {
		called[1] = true
		return nil
	}
	failThird := func(_ *config.CloudConfig) error {
		called[2] = true
		return errors.New("third failed")
	}

	err := a.runApplies(&config.CloudConfig{}, failFirst, succeedSecond, failThird)

	// All three appliers must have been called
	assert.True(t, called[0], "first applier should have been called")
	assert.True(t, called[1], "second applier should have been called")
	assert.True(t, called[2], "third applier should have been called")

	// Error must be non-nil
	require.Error(t, err)

	// Error must be a joined error (errors.Join implements Unwrap() []error)
	var joinedErr interface{ Unwrap() []error }
	require.ErrorAs(t, err, &joinedErr)

	// Must contain exactly 2 sub-errors
	subErrors := joinedErr.Unwrap()
	require.Len(t, subErrors, 2)

	// Both error messages must be present
	assert.Equal(t, "first failed", subErrors[0].Error())
	assert.Equal(t, "third failed", subErrors[1].Error())
}

// ---------------------------------------------------------------------------
// RunApply — Chain Composition
// ---------------------------------------------------------------------------

func TestRunApply_ChainComposition(t *testing.T) {
	t.Parallel()

	// Set up a temp dir with an "install" mode file so ApplyK3S returns nil
	// immediately (early return on install mode) and ApplyInstall calls
	// Run("k3os", "install") which we mock.
	root := t.TempDir()
	modePath := filepath.Join(root, system.StatePath("mode"))
	require.NoError(t, os.MkdirAll(filepath.Dir(modePath), 0o755))
	require.NoError(t, os.WriteFile(modePath, []byte("install"), 0o644))

	mockFS := &MockFileSystem{}
	mockCmd := &MockCommandRunner{}
	mockMod := &MockModuleLoader{}
	mockSys := &MockSysctlApplier{}
	mockHN := &MockHostnameSetter{}

	a := newTestApplier(mockFS, mockCmd, mockMod, mockSys, mockHN)
	a.modePrefix = []string{root}

	// --- ApplyModules: no modules configured → LoadedModules called, returns empty map ---
	mockMod.On("LoadedModules").Return(map[string]bool{}, nil)

	// --- ApplyDNS: writes /etc/connman/main.conf with default DNS ---
	mockFS.On("WriteFile", "/etc/connman/main.conf", mock.Anything, os.FileMode(0o644)).Return(nil)

	// --- ApplySSHKeysWithNet: reads /etc/passwd (key assertion for this test) ---
	mockFS.On("ReadFile", "/etc/passwd").Return([]byte(mockPasswd), nil)
	mockFS.On("Stat", "/home/rancher/.ssh").Return(nil, os.ErrNotExist)
	mockFS.On("MkdirAll", "/home/rancher/.ssh", os.FileMode(0o700)).Return(nil)
	mockFS.On("Chown", "/home/rancher/.ssh", 1000, 1000).Return(nil)

	// --- ApplyK3SInstall: mode is "install" → returns nil immediately (early return) ---
	// --- ApplyInstall: mode is "install" → calls Run("k3os", "install") ---
	mockCmd.On("Run", "k3os", "install").Return(nil)

	// --- ApplyEnvironment: empty map → no-op (handled by code, no FS calls) ---
	// --- ApplyWriteFiles: empty list → no-op ---
	// --- ApplyRuncmd: empty list → no-op ---
	// --- ApplyPassword: empty password → no-op ---
	// --- ApplyWifi: empty wifi → no-op ---
	// --- ApplyHostname: empty hostname → no-op ---
	// --- ApplySysctls: empty map → no-op ---

	// Use a minimal CloudConfig that exercises the chain without errors.
	// All fields are zero-value/empty except what's needed to avoid nil panics.
	cfg := &config.CloudConfig{}

	err := a.RunApply(cfg)
	require.NoError(t, err)

	// Assert all mock expectations are met.
	mockFS.AssertExpectations(t)
	mockCmd.AssertExpectations(t)
	mockMod.AssertExpectations(t)
	mockSys.AssertExpectations(t)
	mockHN.AssertExpectations(t)

	// Verify ApplySSHKeysWithNet path was exercised (not ApplySSHKeys).
	// The key indicator is that ReadFile("/etc/passwd") was called — this is
	// the entry point for the SSH key logic.
	mockFS.AssertCalled(t, "ReadFile", "/etc/passwd")

	// Verify LoadedModules was called (ApplyModules was in the chain).
	mockMod.AssertCalled(t, "LoadedModules")

	// Verify WriteFile for DNS was called (ApplyDNS was in the chain).
	mockFS.AssertCalled(t, "WriteFile", "/etc/connman/main.conf", mock.Anything, os.FileMode(0o644))
}

// ---------------------------------------------------------------------------
// InitApply — Chain Composition
// ---------------------------------------------------------------------------

func TestInitApply_ChainComposition(t *testing.T) {
	t.Parallel()

	mockFS := &MockFileSystem{}
	mockCmd := &MockCommandRunner{}
	mockMod := &MockModuleLoader{}
	mockSys := &MockSysctlApplier{}
	mockHN := &MockHostnameSetter{}

	a := newTestApplier(mockFS, mockCmd, mockMod, mockSys, mockHN)

	// --- ApplyModules: one module not yet loaded → LoadModule called ---
	mockMod.On("LoadedModules").Return(map[string]bool{}, nil)
	mockMod.On("LoadModule", "ext4", "").Return(nil)

	// --- ApplySysctls: one sysctl entry → Set called ---
	mockSys.On("Set", "net.ipv4.ip_forward", "1").Return(nil)

	// --- ApplyHostname: hostname set → SetHostname + syncHostname calls ---
	mockHN.On("SetHostname", "inithost").Return(nil)
	mockFS.On("Hostname").Return("inithost", nil)
	mockFS.On("WriteFile", "/etc/hostname", []byte("inithost\n"), os.FileMode(0o644)).Return(nil)
	hostsFile := &MockFile{}
	hostsFile.buf.WriteString("127.0.0.1 localhost\n127.0.1.1 oldhost\n")
	hostsFile.On("Close").Return(nil)
	mockFS.On("Open", "/etc/hosts").Return(hostsFile, nil)
	mockFS.On("WriteFile", "/etc/hosts", mock.Anything, os.FileMode(0o600)).Return(nil)

	// --- ApplyWriteFiles: one write_files entry → full FS call sequence ---
	tmpFile := &MockFile{}
	tmpFile.On("Close").Return(nil)
	tmpFile.On("Name").Return("/tmp/wfs-temp123")
	mockFS.On("Stat", "/").Return(fakeDir{}, nil)
	mockFS.On("CreateTemp", "/", "wfs-temp").Return(tmpFile, nil)
	mockFS.On("Chmod", "/tmp/wfs-temp123", os.FileMode(0o644)).Return(nil)
	mockFS.On("Rename", "/tmp/wfs-temp123", "/hello.txt").Return(nil)

	// --- ApplyEnvironment: one env var, no existing file → ReadFile fails, WriteFile called ---
	mockFS.On("ReadFile", "/etc/environment").Return(nil, os.ErrNotExist)
	mockFS.On("WriteFile", "/etc/environment", mock.Anything, os.FileMode(0o644)).Return(nil)

	// --- ApplyInitcmd: one command → RunShell called ---
	mockCmd.On("RunShell", "echo init").Return(nil)

	// Build a CloudConfig that exercises all chain members.
	cfg := &config.CloudConfig{
		Hostname: "inithost",
		Initcmd:  []string{"echo init"},
		Runcmd:   []string{"echo run"}, // should NOT be called
		WriteFiles: []config.File{
			{Path: "/hello.txt", Content: "world"},
		},
	}
	cfg.K3OS.Modules = []string{"ext4"}
	cfg.K3OS.Sysctls = map[string]string{"net.ipv4.ip_forward": "1"}
	cfg.K3OS.Environment = map[string]string{"FOO": "bar"}
	cfg.SSHAuthorizedKeys = []string{"ssh-rsa AAAA..."} // should NOT trigger SSH key logic

	err := a.InitApply(cfg)
	require.NoError(t, err)

	// Assert all mock expectations are met.
	mockFS.AssertExpectations(t)
	mockCmd.AssertExpectations(t)
	mockMod.AssertExpectations(t)
	mockSys.AssertExpectations(t)
	mockHN.AssertExpectations(t)

	// Verify ApplyModules was exercised.
	mockMod.AssertCalled(t, "LoadedModules")
	mockMod.AssertCalled(t, "LoadModule", "ext4", "")

	// Verify ApplySysctls was exercised.
	mockSys.AssertCalled(t, "Set", "net.ipv4.ip_forward", "1")

	// Verify ApplyHostname was exercised.
	mockHN.AssertCalled(t, "SetHostname", "inithost")

	// Verify ApplyWriteFiles was exercised.
	mockFS.AssertCalled(t, "CreateTemp", "/", "wfs-temp")

	// Verify ApplyEnvironment was exercised.
	mockFS.AssertCalled(t, "ReadFile", "/etc/environment")

	// Verify ApplyInitcmd was exercised.
	mockCmd.AssertCalled(t, "RunShell", "echo init")

	// Verify ApplyRuncmd is NOT in the chain: RunShell must not be called
	// with the runcmd value.
	mockCmd.AssertNotCalled(t, "RunShell", "echo run")

	// Verify ApplySSHKeys is NOT in the chain: ReadFile("/etc/passwd") must
	// not be called (that's the entry point for SSH key logic).
	mockFS.AssertNotCalled(t, "ReadFile", "/etc/passwd")
}

// fakeDir implements os.FileInfo for directory stat checks in WriteFiles.
type fakeDir struct{}

func (fakeDir) Name() string      { return "/" }
func (fakeDir) Size() int64       { return 0 }
func (fakeDir) Mode() os.FileMode { return os.ModeDir | 0o755 }
func (fakeDir) ModTime() time.Time {
	return time.Time{}
}
func (fakeDir) IsDir() bool      { return true }
func (fakeDir) Sys() interface{} { return nil }

// ---------------------------------------------------------------------------
// InstallApply — Chain Composition
// ---------------------------------------------------------------------------

func TestInstallApply_ChainComposition(t *testing.T) {
	t.Parallel()

	// Set up a temp dir with a "live" mode file so ApplyK3S does NOT
	// early-return (it returns nil immediately only when mode == "install").
	root := t.TempDir()
	modePath := filepath.Join(root, system.StatePath("mode"))
	require.NoError(t, os.MkdirAll(filepath.Dir(modePath), 0o755))
	require.NoError(t, os.WriteFile(modePath, []byte("live"), 0o644))

	mockFS := &MockFileSystem{}
	mockCmd := &MockCommandRunner{}
	mockMod := &MockModuleLoader{}
	mockSys := &MockSysctlApplier{}
	mockHN := &MockHostnameSetter{}

	a := newTestApplier(mockFS, mockCmd, mockMod, mockSys, mockHN)
	a.modePrefix = []string{root}

	// --- ApplyK3SWithRestart: mode is "live", /sbin/k3s exists, restart=true ---
	// Stat("/sbin/k3s") succeeds → k3sExists = true
	mockFS.On("Stat", "/sbin/k3s").Return(fakeDir{}, nil)
	// Stat("/usr/local/bin/k3s") also checked
	mockFS.On("Stat", "/usr/local/bin/k3s").Return(nil, os.ErrNotExist)

	// RunWithEnv is called with restart=true (no INSTALL_K3S_SKIP_START),
	// k3sExists=true (INSTALL_K3S_SKIP_DOWNLOAD, INSTALL_K3S_BIN_DIR, etc.)
	// The variadic args expand to individual arguments in the mock, so we
	// register multiple matchers with .Maybe() to handle the variable arg count.
	var runWithEnvCalled bool
	runHandler := func(_ mock.Arguments) { runWithEnvCalled = true }
	mockCmd.On("RunWithEnv", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Run(runHandler).Return(nil).Maybe()
	mockCmd.On("RunWithEnv", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Run(runHandler).Return(nil).Maybe()
	mockCmd.On("RunWithEnv", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Run(runHandler).Return(nil).Maybe()

	cfg := &config.CloudConfig{}

	err := a.InstallApply(cfg)
	require.NoError(t, err)

	// Assert all mock expectations are met.
	mockFS.AssertExpectations(t)
	mockCmd.AssertExpectations(t)
	mockMod.AssertExpectations(t)
	mockSys.AssertExpectations(t)
	mockHN.AssertExpectations(t)

	// Verify ApplyK3SWithRestart path was exercised: RunWithEnv must have
	// been called (the k3s install script invocation).
	assert.True(t, runWithEnvCalled, "RunWithEnv should have been called for k3s install")

	// Verify NO other applier methods are called — only ApplyK3SWithRestart
	// should be in the InstallApply chain.
	mockMod.AssertNotCalled(t, "LoadedModules")
	mockMod.AssertNotCalled(t, "LoadModule", mock.Anything, mock.Anything)
	mockHN.AssertNotCalled(t, "SetHostname", mock.Anything)
	mockCmd.AssertNotCalled(t, "RunShell", mock.Anything)
	mockCmd.AssertNotCalled(t, "Run", mock.Anything, mock.Anything)
	mockCmd.AssertNotCalled(t, "RunWithStdin", mock.Anything, mock.Anything)
	mockFS.AssertNotCalled(t, "ReadFile", "/etc/passwd")
	mockFS.AssertNotCalled(t, "WriteFile", "/etc/connman/main.conf", mock.Anything, mock.Anything)
	mockSys.AssertNotCalled(t, "Set", mock.Anything, mock.Anything)
}

// ---------------------------------------------------------------------------
// BootApply — Chain Composition
// ---------------------------------------------------------------------------

func TestBootApply_ChainComposition(t *testing.T) {
	t.Parallel()

	// Set up a temp dir with a "live" mode file so ApplyK3SNoRestart returns
	// nil immediately (k3s not found + restart=false → early return).
	root := t.TempDir()
	modePath := filepath.Join(root, system.StatePath("mode"))
	require.NoError(t, os.MkdirAll(filepath.Dir(modePath), 0o755))
	require.NoError(t, os.WriteFile(modePath, []byte("live"), 0o644))

	mockFS := &MockFileSystem{}
	mockCmd := &MockCommandRunner{}
	mockMod := &MockModuleLoader{}
	mockSys := &MockSysctlApplier{}
	mockHN := &MockHostnameSetter{}

	a := newTestApplier(mockFS, mockCmd, mockMod, mockSys, mockHN)
	a.modePrefix = []string{root}

	// --- ApplyDataSource: one data source → WriteFile for /etc/conf.d/cloud-config ---
	mockFS.On("WriteFile", "/etc/conf.d/cloud-config", mock.Anything, os.FileMode(0o644)).Return(nil)

	// --- ApplyModules: no modules configured → LoadedModules called, returns empty map ---
	mockMod.On("LoadedModules").Return(map[string]bool{}, nil)

	// --- ApplyDNS: writes /etc/connman/main.conf with default DNS ---
	mockFS.On("WriteFile", "/etc/connman/main.conf", mock.Anything, os.FileMode(0o644)).Return(nil)

	// --- ApplySSHKeys (withNet=false): reads /etc/passwd ---
	mockFS.On("ReadFile", "/etc/passwd").Return([]byte(mockPasswd), nil)
	mockFS.On("Stat", "/home/rancher/.ssh").Return(nil, os.ErrNotExist)
	mockFS.On("MkdirAll", "/home/rancher/.ssh", os.FileMode(0o700)).Return(nil)
	mockFS.On("Chown", "/home/rancher/.ssh", 1000, 1000).Return(nil)

	// --- ApplyK3SNoRestart: mode is "live", /sbin/k3s does not exist, restart=false → early return ---
	mockFS.On("Stat", "/sbin/k3s").Return(nil, os.ErrNotExist)
	mockFS.On("Stat", "/usr/local/bin/k3s").Return(nil, os.ErrNotExist)

	// --- ApplyBootcmd: one command → RunShell called ---
	mockCmd.On("RunShell", "echo boot").Return(nil)

	// Use a CloudConfig that exercises the key chain members:
	// - One data source (ApplyDataSource)
	// - One SSH key entry path (ApplySSHKeys)
	// - One bootcmd (ApplyBootcmd)
	cfg := &config.CloudConfig{}
	cfg.K3OS.DataSources = []string{"cdrom"}
	cfg.Bootcmd = []string{"echo boot"}

	err := a.BootApply(cfg)
	require.NoError(t, err)

	// Assert all mock expectations are met.
	mockFS.AssertExpectations(t)
	mockCmd.AssertExpectations(t)
	mockMod.AssertExpectations(t)
	mockSys.AssertExpectations(t)
	mockHN.AssertExpectations(t)

	// Verify ApplyDataSource was exercised: WriteFile for /etc/conf.d/cloud-config.
	mockFS.AssertCalled(t, "WriteFile", "/etc/conf.d/cloud-config", mock.Anything, os.FileMode(0o644))

	// Verify ApplySSHKeys path was exercised (not ApplySSHKeysWithNet).
	// The key indicator is that ReadFile("/etc/passwd") was called — this is
	// the entry point for the SSH key logic. BootApply uses ApplySSHKeys
	// (withNet=false), not ApplySSHKeysWithNet (withNet=true).
	mockFS.AssertCalled(t, "ReadFile", "/etc/passwd")

	// Verify ApplyBootcmd was exercised: RunShell called with our boot command.
	mockCmd.AssertCalled(t, "RunShell", "echo boot")
}
