package cc

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/petercb/k3os-bin/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// newTestApplier constructs an Applier wired with the provided mock
// implementations. Pass nil for any interface not needed by the function under
// test.
func newTestApplier(fs *MockFileSystem, cmd *MockCommandRunner, mod *MockModuleLoader, sys *MockSysctlApplier, hn *MockHostnameSetter) *Applier {
	return &Applier{FS: fs, Cmd: cmd, Modules: mod, Sysctl: sys, Hostname: hn}
}

// ---------------------------------------------------------------------------
// ApplyModules
// ---------------------------------------------------------------------------

func TestApplyModules(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		modules     []string
		loadedMap   map[string]bool
		setupMock   func(mod *MockModuleLoader)
		wantErr     bool
		errContains string
	}{
		{
			name:      "module not loaded calls LoadModule",
			modules:   []string{"ext4"},
			loadedMap: map[string]bool{},
			setupMock: func(mod *MockModuleLoader) {
				mod.On("LoadedModules").Return(map[string]bool{}, nil)
				mod.On("LoadModule", "ext4", "").Return(nil)
			},
			wantErr: false,
		},
		{
			name:    "module with params splits correctly",
			modules: []string{"foo bar baz"},
			setupMock: func(mod *MockModuleLoader) {
				mod.On("LoadedModules").Return(map[string]bool{}, nil)
				// params[0]="foo", strings.Join(params[1:], " ")="bar baz"
				mod.On("LoadModule", "foo", "bar baz").Return(nil)
			},
			wantErr: false,
		},
		{
			name:    "module already loaded skips LoadModule",
			modules: []string{"ext4"},
			setupMock: func(mod *MockModuleLoader) {
				// The map key is the full module string from cfg.K3OS.Modules
				mod.On("LoadedModules").Return(map[string]bool{"ext4": true}, nil)
				// LoadModule must NOT be called
			},
			wantErr: false,
		},
		{
			name:    "LoadedModules error is propagated",
			modules: []string{"ext4"},
			setupMock: func(mod *MockModuleLoader) {
				mod.On("LoadedModules").Return(nil, errors.New("proc read error"))
			},
			wantErr:     true,
			errContains: "proc read error",
		},
		{
			name:    "LoadModule error is propagated and wrapped",
			modules: []string{"ext4"},
			setupMock: func(mod *MockModuleLoader) {
				mod.On("LoadedModules").Return(map[string]bool{}, nil)
				mod.On("LoadModule", "ext4", "").Return(errors.New("modprobe failed"))
			},
			wantErr:     true,
			errContains: "modprobe failed",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			mod := &MockModuleLoader{}
			tc.setupMock(mod)

			a := newTestApplier(nil, nil, mod, nil, nil)
			cfg := &config.CloudConfig{}
			cfg.K3OS.Modules = tc.modules

			err := a.ApplyModules(cfg)

			if tc.wantErr {
				require.Error(t, err)
				if tc.errContains != "" {
					assert.Contains(t, err.Error(), tc.errContains)
				}
			} else {
				require.NoError(t, err)
			}

			mod.AssertExpectations(t)
		})
	}
}

// ---------------------------------------------------------------------------
// ApplySysctls
// ---------------------------------------------------------------------------

func TestApplySysctls(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		sysctls   map[string]string
		setupMock func(sys *MockSysctlApplier)
		wantErr   bool
	}{
		{
			name:    "one sysctl entry calls Set with correct key and value",
			sysctls: map[string]string{"net.ipv4.ip_forward": "1"},
			setupMock: func(sys *MockSysctlApplier) {
				sys.On("Set", "net.ipv4.ip_forward", "1").Return(nil)
			},
			wantErr: false,
		},
		{
			name:    "empty sysctls map skips Set and returns no error",
			sysctls: map[string]string{},
			setupMock: func(_ *MockSysctlApplier) {
				// Set must NOT be called
			},
			wantErr: false,
		},
		{
			name:    "Set error is propagated",
			sysctls: map[string]string{"kernel.panic": "10"},
			setupMock: func(sys *MockSysctlApplier) {
				sys.On("Set", "kernel.panic", "10").Return(errors.New("sysctl write failed"))
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			sys := &MockSysctlApplier{}
			tc.setupMock(sys)

			a := newTestApplier(nil, nil, nil, sys, nil)
			cfg := &config.CloudConfig{}
			cfg.K3OS.Sysctls = tc.sysctls

			err := a.ApplySysctls(cfg)

			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			sys.AssertExpectations(t)
		})
	}
}

// ---------------------------------------------------------------------------
// ApplyHostname
// ---------------------------------------------------------------------------

func TestApplyHostname(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		hostname    string
		setupMock   func(fs *MockFileSystem, hn *MockHostnameSetter)
		wantErr     bool
		errContains string
	}{
		{
			name:     "hostname set calls SetHostname and FS writes",
			hostname: "myhost",
			setupMock: func(fs *MockFileSystem, hn *MockHostnameSetter) {
				hn.On("SetHostname", "myhost").Return(nil)
				fs.On("Hostname").Return("myhost", nil)
				fs.On("WriteFile", "/etc/hostname", []byte("myhost\n"), os.FileMode(0o644)).Return(nil)

				hostsFile := &MockFile{}
				hostsFile.buf.WriteString("127.0.1.1 oldhost\n")
				hostsFile.On("Close").Return(nil)
				fs.On("Open", "/etc/hosts").Return(hostsFile, nil)

				fs.On("WriteFile", "/etc/hosts", []byte("127.0.1.1 myhost\n"), os.FileMode(0o600)).Return(nil)
			},
			wantErr: false,
		},
		{
			name:     "empty hostname is a no-op",
			hostname: "",
			setupMock: func(_ *MockFileSystem, _ *MockHostnameSetter) {
				// No calls expected
			},
			wantErr: false,
		},
		{
			name:     "SetHostname error is propagated",
			hostname: "myhost",
			setupMock: func(_ *MockFileSystem, hn *MockHostnameSetter) {
				hn.On("SetHostname", "myhost").Return(errors.New("syscall failed"))
			},
			wantErr:     true,
			errContains: "syscall failed",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			fs := &MockFileSystem{}
			hn := &MockHostnameSetter{}
			tc.setupMock(fs, hn)

			a := newTestApplier(fs, nil, nil, nil, hn)
			cfg := &config.CloudConfig{}
			cfg.Hostname = tc.hostname

			err := a.ApplyHostname(cfg)

			if tc.wantErr {
				require.Error(t, err)
				if tc.errContains != "" {
					assert.Contains(t, err.Error(), tc.errContains)
				}
			} else {
				require.NoError(t, err)
			}

			fs.AssertExpectations(t)
			hn.AssertExpectations(t)
		})
	}
}

// ---------------------------------------------------------------------------
// ApplyDNS
// ---------------------------------------------------------------------------

func TestApplyDNS(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		dnsNameservers []string
		ntpServers     []string
		setupMock      func(fs *MockFileSystem, capturedData *[]byte)
		wantErr        bool
		errContains    string
		checkContent   func(t *testing.T, data []byte)
	}{
		{
			name:           "with DNS servers writes FallbackNameservers with configured servers",
			dnsNameservers: []string{"1.1.1.1", "1.0.0.1"},
			setupMock: func(fs *MockFileSystem, capturedData *[]byte) {
				fs.On("WriteFile", "/etc/connman/main.conf", mock.MatchedBy(func(data []byte) bool {
					*capturedData = data
					return true
				}), os.FileMode(0o644)).Return(nil)
			},
			checkContent: func(t *testing.T, data []byte) {
				t.Helper()
				content := string(data)
				assert.Contains(t, content, "FallbackNameservers=1.1.1.1,1.0.0.1\n")
				assert.Contains(t, content, "[General]\n")
				assert.Contains(t, content, "NetworkInterfaceBlacklist=veth\n")
				assert.Contains(t, content, "PreferredTechnologies=ethernet,wifi\n")
			},
		},
		{
			name:           "default DNS when no nameservers configured writes FallbackNameservers=8.8.8.8",
			dnsNameservers: nil,
			setupMock: func(fs *MockFileSystem, capturedData *[]byte) {
				fs.On("WriteFile", "/etc/connman/main.conf", mock.MatchedBy(func(data []byte) bool {
					*capturedData = data
					return true
				}), os.FileMode(0o644)).Return(nil)
			},
			checkContent: func(t *testing.T, data []byte) {
				t.Helper()
				content := string(data)
				assert.Contains(t, content, "FallbackNameservers=8.8.8.8\n")
			},
		},
		{
			name:           "with NTP servers writes FallbackTimeservers line",
			dnsNameservers: nil,
			ntpServers:     []string{"pool.ntp.org", "time.cloudflare.com"},
			setupMock: func(fs *MockFileSystem, capturedData *[]byte) {
				fs.On("WriteFile", "/etc/connman/main.conf", mock.MatchedBy(func(data []byte) bool {
					*capturedData = data
					return true
				}), os.FileMode(0o644)).Return(nil)
			},
			checkContent: func(t *testing.T, data []byte) {
				t.Helper()
				content := string(data)
				assert.Contains(t, content, "FallbackTimeservers=pool.ntp.org,time.cloudflare.com\n")
				// default DNS still present
				assert.Contains(t, content, "FallbackNameservers=8.8.8.8\n")
			},
		},
		{
			name:           "WriteFile error is propagated and wrapped",
			dnsNameservers: nil,
			setupMock: func(fs *MockFileSystem, _ *[]byte) {
				fs.On("WriteFile", "/etc/connman/main.conf", mock.Anything, os.FileMode(0o644)).
					Return(errors.New("disk full"))
			},
			wantErr:     true,
			errContains: "failed to write /etc/connman/main.conf",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			fs := &MockFileSystem{}
			var capturedData []byte
			tc.setupMock(fs, &capturedData)

			a := newTestApplier(fs, nil, nil, nil, nil)
			cfg := &config.CloudConfig{}
			cfg.K3OS.DNSNameservers = tc.dnsNameservers
			cfg.K3OS.NTPServers = tc.ntpServers

			err := a.ApplyDNS(cfg)

			if tc.wantErr {
				require.Error(t, err)
				if tc.errContains != "" {
					assert.Contains(t, err.Error(), tc.errContains)
				}
			} else {
				require.NoError(t, err)
				if tc.checkContent != nil {
					tc.checkContent(t, capturedData)
				}
			}

			fs.AssertExpectations(t)
		})
	}
}

// ---------------------------------------------------------------------------
// ApplyPassword
// ---------------------------------------------------------------------------

func TestApplyPassword(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		password    string
		setupMock   func(cmd *MockCommandRunner)
		wantErr     bool
		errContains string
	}{
		{
			name:     "empty password is a no-op",
			password: "",
			setupMock: func(_ *MockCommandRunner) {
				// RunWithStdin must NOT be called
			},
			wantErr: false,
		},
		{
			name:     "plain password calls RunWithStdin without -e flag",
			password: "hunter2",
			setupMock: func(cmd *MockCommandRunner) {
				cmd.On("RunWithStdin", "rancher:hunter2", "chpasswd").Return(nil)
			},
			wantErr: false,
		},
		{
			name:     "hashed password ($ prefix) calls RunWithStdin with -e flag",
			password: "$6$rounds=4096$saltsalt$hashedvalue",
			setupMock: func(cmd *MockCommandRunner) {
				cmd.On("RunWithStdin", "rancher:$6$rounds=4096$saltsalt$hashedvalue", "chpasswd", "-e").Return(nil)
			},
			wantErr: false,
		},
		{
			name:     "RunWithStdin error is propagated",
			password: "badpass",
			setupMock: func(cmd *MockCommandRunner) {
				cmd.On("RunWithStdin", "rancher:badpass", "chpasswd").Return(errors.New("chpasswd failed"))
			},
			wantErr:     true,
			errContains: "chpasswd failed",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cmd := &MockCommandRunner{}
			tc.setupMock(cmd)

			a := newTestApplier(nil, cmd, nil, nil, nil)
			cfg := &config.CloudConfig{}
			cfg.K3OS.Password = tc.password

			err := a.ApplyPassword(cfg)

			if tc.wantErr {
				require.Error(t, err)
				if tc.errContains != "" {
					assert.Contains(t, err.Error(), tc.errContains)
				}
			} else {
				require.NoError(t, err)
			}

			cmd.AssertExpectations(t)
		})
	}
}

// ---------------------------------------------------------------------------
// ApplyRuncmd
// ---------------------------------------------------------------------------

func TestApplyRuncmd(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		runcmd      []string
		setupMock   func(cmd *MockCommandRunner)
		wantErr     bool
		errContains string
	}{
		{
			name:   "empty command list is a no-op",
			runcmd: []string{},
			setupMock: func(_ *MockCommandRunner) {
				// RunShell must NOT be called
			},
			wantErr: false,
		},
		{
			name:   "one command calls RunShell once with that command",
			runcmd: []string{"echo hello"},
			setupMock: func(cmd *MockCommandRunner) {
				cmd.On("RunShell", "echo hello").Return(nil)
			},
			wantErr: false,
		},
		{
			name:   "RunShell error is propagated",
			runcmd: []string{"bad-command"},
			setupMock: func(cmd *MockCommandRunner) {
				cmd.On("RunShell", "bad-command").Return(errors.New("exit status 127"))
			},
			wantErr:     true,
			errContains: "exit status 127",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cmd := &MockCommandRunner{}
			tc.setupMock(cmd)

			a := newTestApplier(nil, cmd, nil, nil, nil)
			cfg := &config.CloudConfig{}
			cfg.Runcmd = tc.runcmd

			err := a.ApplyRuncmd(cfg)

			if tc.wantErr {
				require.Error(t, err)
				if tc.errContains != "" {
					assert.Contains(t, err.Error(), tc.errContains)
				}
			} else {
				require.NoError(t, err)
			}

			cmd.AssertExpectations(t)
		})
	}
}

// ---------------------------------------------------------------------------
// ApplyBootcmd
// ---------------------------------------------------------------------------

func TestApplyBootcmd(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		bootcmd     []string
		setupMock   func(cmd *MockCommandRunner)
		wantErr     bool
		errContains string
	}{
		{
			name:    "empty command list is a no-op",
			bootcmd: []string{},
			setupMock: func(_ *MockCommandRunner) {
				// RunShell must NOT be called
			},
			wantErr: false,
		},
		{
			name:    "one command calls RunShell once with that command",
			bootcmd: []string{"echo hello"},
			setupMock: func(cmd *MockCommandRunner) {
				cmd.On("RunShell", "echo hello").Return(nil)
			},
			wantErr: false,
		},
		{
			name:    "RunShell error is propagated",
			bootcmd: []string{"bad-command"},
			setupMock: func(cmd *MockCommandRunner) {
				cmd.On("RunShell", "bad-command").Return(errors.New("exit status 127"))
			},
			wantErr:     true,
			errContains: "exit status 127",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cmd := &MockCommandRunner{}
			tc.setupMock(cmd)

			a := newTestApplier(nil, cmd, nil, nil, nil)
			cfg := &config.CloudConfig{}
			cfg.Bootcmd = tc.bootcmd

			err := a.ApplyBootcmd(cfg)

			if tc.wantErr {
				require.Error(t, err)
				if tc.errContains != "" {
					assert.Contains(t, err.Error(), tc.errContains)
				}
			} else {
				require.NoError(t, err)
			}

			cmd.AssertExpectations(t)
		})
	}
}

// ---------------------------------------------------------------------------
// ApplyWifi
// ---------------------------------------------------------------------------

func TestApplyWifi(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		wifi        []config.Wifi
		setupMock   func(fs *MockFileSystem, capturedSettings *[]byte, capturedConfig *[]byte)
		wantErr     bool
		errContains string
		checkFiles  func(t *testing.T, settings, cfgFile []byte)
	}{
		{
			name: "empty wifi list is a no-op",
			wifi: []config.Wifi{},
			setupMock: func(_ *MockFileSystem, _ *[]byte, _ *[]byte) {
				// No FS calls expected
			},
			wantErr: false,
		},
		{
			name: "one wifi entry writes settings and cloud-config.config with correct content",
			wifi: []config.Wifi{
				{Name: "MyNetwork", Passphrase: "s3cr3t"},
			},
			setupMock: func(fs *MockFileSystem, capturedSettings *[]byte, capturedConfig *[]byte) {
				fs.On("MkdirAll", "/var/lib/connman", os.FileMode(0o755)).Return(nil)
				fs.On("WriteFile", "/var/lib/connman/settings", mock.Anything, os.FileMode(0o644)).
					Run(func(args mock.Arguments) {
						*capturedSettings = args.Get(1).([]byte)
					}).Return(nil)
				fs.On("WriteFile", "/var/lib/connman/cloud-config.config", mock.Anything, os.FileMode(0o644)).
					Run(func(args mock.Arguments) {
						*capturedConfig = args.Get(1).([]byte)
					}).Return(nil)
			},
			wantErr: false,
			checkFiles: func(t *testing.T, settings, cfgFile []byte) {
				t.Helper()
				// settings file must contain WiFi section
				settingsStr := string(settings)
				assert.Contains(t, settingsStr, "[WiFi]\n")
				assert.Contains(t, settingsStr, "Enable=true\n")
				assert.Contains(t, settingsStr, "Tethering=false\n")

				// cloud-config.config must contain global header and wifi entry
				cfgStr := string(cfgFile)
				assert.Contains(t, cfgStr, "[global]\n")
				assert.Contains(t, cfgStr, "Name=cloud-config\n")
				assert.Contains(t, cfgStr, "[service_wifi0]\n")
				assert.Contains(t, cfgStr, "Type=wifi\n")
				assert.Contains(t, cfgStr, "Passphrase=s3cr3t\n")
				assert.Contains(t, cfgStr, "Name=MyNetwork\n")
			},
		},
		{
			name: "MkdirAll error is propagated and wrapped",
			wifi: []config.Wifi{
				{Name: "MyNetwork", Passphrase: "s3cr3t"},
			},
			setupMock: func(fs *MockFileSystem, _ *[]byte, _ *[]byte) {
				fs.On("MkdirAll", "/var/lib/connman", os.FileMode(0o755)).
					Return(errors.New("permission denied"))
			},
			wantErr:     true,
			errContains: "failed to mkdir /var/lib/connman",
		},
		{
			name: "WriteFile for settings error is propagated and wrapped",
			wifi: []config.Wifi{
				{Name: "MyNetwork", Passphrase: "s3cr3t"},
			},
			setupMock: func(fs *MockFileSystem, _ *[]byte, _ *[]byte) {
				fs.On("MkdirAll", "/var/lib/connman", os.FileMode(0o755)).Return(nil)
				fs.On("WriteFile", "/var/lib/connman/settings", mock.Anything, os.FileMode(0o644)).
					Return(errors.New("disk full"))
			},
			wantErr:     true,
			errContains: "failed to write to /var/lib/connman/settings",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			fs := &MockFileSystem{}
			var capturedSettings, capturedConfig []byte
			tc.setupMock(fs, &capturedSettings, &capturedConfig)

			a := newTestApplier(fs, nil, nil, nil, nil)
			cfg := &config.CloudConfig{}
			cfg.K3OS.Wifi = tc.wifi

			err := a.ApplyWifi(cfg)

			if tc.wantErr {
				require.Error(t, err)
				if tc.errContains != "" {
					assert.Contains(t, err.Error(), tc.errContains)
				}
			} else {
				require.NoError(t, err)
				if tc.checkFiles != nil {
					tc.checkFiles(t, capturedSettings, capturedConfig)
				}
			}

			fs.AssertExpectations(t)
		})
	}
}

// ---------------------------------------------------------------------------
// ApplyInitcmd
// ---------------------------------------------------------------------------

func TestApplyInitcmd(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		initcmd     []string
		setupMock   func(cmd *MockCommandRunner)
		wantErr     bool
		errContains string
	}{
		{
			name:    "empty command list is a no-op",
			initcmd: []string{},
			setupMock: func(_ *MockCommandRunner) {
				// RunShell must NOT be called
			},
			wantErr: false,
		},
		{
			name:    "one command calls RunShell once with that command",
			initcmd: []string{"echo hello"},
			setupMock: func(cmd *MockCommandRunner) {
				cmd.On("RunShell", "echo hello").Return(nil)
			},
			wantErr: false,
		},
		{
			name:    "RunShell error is propagated",
			initcmd: []string{"bad-command"},
			setupMock: func(cmd *MockCommandRunner) {
				cmd.On("RunShell", "bad-command").Return(errors.New("exit status 127"))
			},
			wantErr:     true,
			errContains: "exit status 127",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cmd := &MockCommandRunner{}
			tc.setupMock(cmd)

			a := newTestApplier(nil, cmd, nil, nil, nil)
			cfg := &config.CloudConfig{}
			cfg.Initcmd = tc.initcmd

			err := a.ApplyInitcmd(cfg)

			if tc.wantErr {
				require.Error(t, err)
				if tc.errContains != "" {
					assert.Contains(t, err.Error(), tc.errContains)
				}
			} else {
				require.NoError(t, err)
			}

			cmd.AssertExpectations(t)
		})
	}
}

// ---------------------------------------------------------------------------
// ApplyWriteFiles
// ---------------------------------------------------------------------------

// mockFileInfo is a minimal os.FileInfo implementation for use in Stat mocks.
type mockFileInfo struct {
	isDir bool
}

func (m *mockFileInfo) Name() string       { return "" }
func (m *mockFileInfo) Size() int64        { return 0 }
func (m *mockFileInfo) Mode() os.FileMode  { return 0 }
func (m *mockFileInfo) ModTime() time.Time { return time.Time{} }
func (m *mockFileInfo) IsDir() bool        { return m.isDir }
func (m *mockFileInfo) Sys() interface{}   { return nil }

func TestApplyWriteFiles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		writeFiles []config.File
		setupMock  func(fs *MockFileSystem)
	}{
		{
			name: "one plain-content entry executes full FS call sequence",
			writeFiles: []config.File{
				{
					Path:    "/etc/foo",
					Content: "hello world",
				},
			},
			setupMock: func(fs *MockFileSystem) {
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
		},
		{
			name:       "empty write_files list makes no FS calls",
			writeFiles: []config.File{},
			setupMock: func(_ *MockFileSystem) {
				// No FS calls expected
			},
		},
		{
			name: "CreateTemp error is logged and function still returns nil",
			writeFiles: []config.File{
				{
					Path:    "/etc/bar",
					Content: "data",
				},
			},
			setupMock: func(fs *MockFileSystem) {
				// ensureDirectoryExists: Stat succeeds
				fs.On("Stat", "/etc").Return(&mockFileInfo{isDir: true}, nil)

				// CreateTemp fails
				fs.On("CreateTemp", "/etc", "wfs-temp").Return(nil, errors.New("no space left on device"))
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			fs := &MockFileSystem{}
			tc.setupMock(fs)

			a := newTestApplier(fs, nil, nil, nil, nil)
			cfg := &config.CloudConfig{}
			cfg.WriteFiles = tc.writeFiles

			// ApplyWriteFiles always returns nil — errors are swallowed by writefile.WriteFiles
			err := a.ApplyWriteFiles(cfg)
			require.NoError(t, err)

			fs.AssertExpectations(t)
		})
	}
}

// ---------------------------------------------------------------------------
// ApplySSHKeys / ApplySSHKeysWithNet
// ---------------------------------------------------------------------------

// mockPasswd is a minimal /etc/passwd entry for the rancher user.
const mockPasswd = "rancher:x:1000:1000::/home/rancher:/bin/sh\n"

func TestApplySSHKeys(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		setupMock   func(fs *MockFileSystem)
		wantErr     bool
		errContains string
	}{
		{
			name: "happy path: valid passwd, no SSH keys configured",
			setupMock: func(fs *MockFileSystem) {
				fs.On("ReadFile", "/etc/passwd").Return([]byte(mockPasswd), nil)
				// Stat returns ErrNotExist → MkdirAll is called
				fs.On("Stat", "/home/rancher/.ssh").Return(nil, os.ErrNotExist)
				fs.On("MkdirAll", "/home/rancher/.ssh", os.FileMode(0o700)).Return(nil)
				fs.On("Chown", "/home/rancher/.ssh", 1000, 1000).Return(nil)
				// No SSHAuthorizedKeys → authorizeSSHKey is never called
			},
			wantErr: false,
		},
		{
			name: "ReadFile /etc/passwd error is propagated",
			setupMock: func(fs *MockFileSystem) {
				fs.On("ReadFile", "/etc/passwd").Return(nil, errors.New("read error"))
			},
			wantErr:     true,
			errContains: "read error",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			fs := &MockFileSystem{}
			tc.setupMock(fs)

			a := newTestApplier(fs, nil, nil, nil, nil)
			cfg := &config.CloudConfig{}
			// SSHAuthorizedKeys intentionally empty — keeps mock setup minimal

			err := a.ApplySSHKeys(cfg)

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
// ApplyEnvironment
// ---------------------------------------------------------------------------

func TestApplyEnvironment(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		environment  map[string]string
		setupMock    func(fs *MockFileSystem, capturedData *[]byte)
		wantErr      bool
		errContains  string
		checkContent func(t *testing.T, data []byte)
	}{
		{
			name:        "new file: ReadFile returns ErrNotExist and configured vars are written",
			environment: map[string]string{"FOO": "bar", "BAZ": "qux"},
			setupMock: func(fs *MockFileSystem, capturedData *[]byte) {
				fs.On("ReadFile", "/etc/environment").Return(nil, os.ErrNotExist)
				fs.On("WriteFile", "/etc/environment", mock.MatchedBy(func(data []byte) bool {
					*capturedData = data
					return true
				}), os.FileMode(0o644)).Return(nil)
			},
			wantErr: false,
			checkContent: func(t *testing.T, data []byte) {
				t.Helper()
				content := string(data)
				assert.Contains(t, content, `FOO="bar"`)
				assert.Contains(t, content, `BAZ="qux"`)
			},
		},
		{
			name:        "merge with existing content: new vars are merged and file is rewritten",
			environment: map[string]string{"NEW_VAR": "new_value"},
			setupMock: func(fs *MockFileSystem, capturedData *[]byte) {
				existing := []byte("EXISTING=\"old_value\"\n")
				fs.On("ReadFile", "/etc/environment").Return(existing, nil)
				fs.On("WriteFile", "/etc/environment", mock.MatchedBy(func(data []byte) bool {
					*capturedData = data
					return true
				}), os.FileMode(0o644)).Return(nil)
			},
			wantErr: false,
			checkContent: func(t *testing.T, data []byte) {
				t.Helper()
				content := string(data)
				// Both existing and new vars must be present
				assert.Contains(t, content, `EXISTING="old_value"`)
				assert.Contains(t, content, `NEW_VAR="new_value"`)
			},
		},
		{
			name:        "empty environment map is a no-op",
			environment: map[string]string{},
			setupMock: func(_ *MockFileSystem, _ *[]byte) {
				// No FS calls expected
			},
			wantErr: false,
		},
		{
			name:        "WriteFile error is propagated and wrapped",
			environment: map[string]string{"KEY": "val"},
			setupMock: func(fs *MockFileSystem, _ *[]byte) {
				fs.On("ReadFile", "/etc/environment").Return(nil, os.ErrNotExist)
				fs.On("WriteFile", "/etc/environment", mock.Anything, os.FileMode(0o644)).
					Return(errors.New("disk full"))
			},
			wantErr:     true,
			errContains: "failed to write to /etc/environment",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			fs := &MockFileSystem{}
			var capturedData []byte
			tc.setupMock(fs, &capturedData)

			a := newTestApplier(fs, nil, nil, nil, nil)
			cfg := &config.CloudConfig{}
			cfg.K3OS.Environment = tc.environment

			err := a.ApplyEnvironment(cfg)

			if tc.wantErr {
				require.Error(t, err)
				if tc.errContains != "" {
					assert.Contains(t, err.Error(), tc.errContains)
				}
			} else {
				require.NoError(t, err)
				if tc.checkContent != nil {
					tc.checkContent(t, capturedData)
				}
			}

			fs.AssertExpectations(t)
		})
	}
}

func TestApplySSHKeysWithNet(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		setupMock   func(fs *MockFileSystem)
		wantErr     bool
		errContains string
	}{
		{
			name: "happy path: valid passwd, no SSH keys configured",
			setupMock: func(fs *MockFileSystem) {
				fs.On("ReadFile", "/etc/passwd").Return([]byte(mockPasswd), nil)
				fs.On("Stat", "/home/rancher/.ssh").Return(nil, os.ErrNotExist)
				fs.On("MkdirAll", "/home/rancher/.ssh", os.FileMode(0o700)).Return(nil)
				fs.On("Chown", "/home/rancher/.ssh", 1000, 1000).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "ReadFile /etc/passwd error is propagated",
			setupMock: func(fs *MockFileSystem) {
				fs.On("ReadFile", "/etc/passwd").Return(nil, errors.New("read error"))
			},
			wantErr:     true,
			errContains: "read error",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			fs := &MockFileSystem{}
			tc.setupMock(fs)

			a := newTestApplier(fs, nil, nil, nil, nil)
			cfg := &config.CloudConfig{}

			err := a.ApplySSHKeysWithNet(cfg)

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
// ApplyDataSource
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// ApplyK3S
// ---------------------------------------------------------------------------

// writeModeFile creates the mode file at the expected path under root.
// mode.Get(prefix...) reads filepath.Join(prefix..., "/run/k3os/mode").
func writeModeFile(t *testing.T, root, content string) {
	t.Helper()
	modePath := root + "/run/k3os/mode"
	require.NoError(t, os.MkdirAll(root+"/run/k3os", 0o755))
	require.NoError(t, os.WriteFile(modePath, []byte(content), 0o644))
}

func TestApplyK3S_InstallModeReturnsNil(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeModeFile(t, root, "install")

	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}

	a := newTestApplier(fs, cmd, nil, nil, nil)
	a.modePrefix = []string{root}

	cfg := &config.CloudConfig{}

	err := a.ApplyK3S(cfg, true, false)
	require.NoError(t, err)

	// No FS.Stat or Cmd.RunWithEnv calls should have been made
	fs.AssertExpectations(t)
	cmd.AssertExpectations(t)
}

func TestApplyK3S_K3sExistsRestartTrue(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeModeFile(t, root, "live")

	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}

	fs.On("Stat", "/sbin/k3s").Return(&mockFileInfo{}, nil)
	fs.On("Stat", "/usr/local/bin/k3s").Return(nil, os.ErrNotExist)

	// Capture the env and args passed to RunWithEnv
	var capturedEnv []string
	var capturedArgs []string
	cmd.On("RunWithEnv", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			capturedEnv = args.Get(0).([]string)
			capturedArgs = []string{}
			for i := 2; i < len(args); i++ {
				capturedArgs = append(capturedArgs, args.Get(i).(string))
			}
		}).Return(nil).Maybe()
	cmd.On("RunWithEnv", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			capturedEnv = args.Get(0).([]string)
			capturedArgs = []string{}
			for i := 2; i < len(args); i++ {
				capturedArgs = append(capturedArgs, args.Get(i).(string))
			}
		}).Return(nil).Maybe()
	cmd.On("RunWithEnv", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			capturedEnv = args.Get(0).([]string)
			capturedArgs = []string{}
			for i := 2; i < len(args); i++ {
				capturedArgs = append(capturedArgs, args.Get(i).(string))
			}
		}).Return(nil).Maybe()

	a := newTestApplier(fs, cmd, nil, nil, nil)
	a.modePrefix = []string{root}

	cfg := &config.CloudConfig{}

	err := a.ApplyK3S(cfg, true, false)
	require.NoError(t, err)

	// Verify env vars contain INSTALL_K3S_SKIP_DOWNLOAD and INSTALL_K3S_BIN_DIR
	assert.Contains(t, capturedEnv, "INSTALL_K3S_SKIP_DOWNLOAD=true")
	assert.Contains(t, capturedEnv, "INSTALL_K3S_BIN_DIR=/sbin")

	fs.AssertExpectations(t)
}

func TestApplyK3S_K3sNotExistsRestartFalse(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeModeFile(t, root, "live")

	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}

	fs.On("Stat", "/sbin/k3s").Return(nil, os.ErrNotExist)
	fs.On("Stat", "/usr/local/bin/k3s").Return(nil, os.ErrNotExist)

	a := newTestApplier(fs, cmd, nil, nil, nil)
	a.modePrefix = []string{root}

	cfg := &config.CloudConfig{}

	err := a.ApplyK3S(cfg, false, false)
	require.NoError(t, err)

	// RunWithEnv must NOT be called — early return because !k3sExists && !restart
	cmd.AssertNotCalled(t, "RunWithEnv")
	fs.AssertExpectations(t)
	cmd.AssertExpectations(t)
}

func TestApplyK3S_ServerURLSetAgentMode(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeModeFile(t, root, "live")

	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}

	fs.On("Stat", "/sbin/k3s").Return(&mockFileInfo{}, nil)
	fs.On("Stat", "/usr/local/bin/k3s").Return(nil, os.ErrNotExist)

	// Capture the env and args passed to RunWithEnv
	var capturedEnv []string
	var capturedArgs []string
	cmd.On("RunWithEnv", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			capturedEnv = args.Get(0).([]string)
			capturedArgs = []string{}
			for i := 2; i < len(args); i++ {
				capturedArgs = append(capturedArgs, args.Get(i).(string))
			}
		}).Return(nil).Maybe()
	cmd.On("RunWithEnv", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			capturedEnv = args.Get(0).([]string)
			capturedArgs = []string{}
			for i := 2; i < len(args); i++ {
				capturedArgs = append(capturedArgs, args.Get(i).(string))
			}
		}).Return(nil).Maybe()
	cmd.On("RunWithEnv", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			capturedEnv = args.Get(0).([]string)
			capturedArgs = []string{}
			for i := 2; i < len(args); i++ {
				capturedArgs = append(capturedArgs, args.Get(i).(string))
			}
		}).Return(nil).Maybe()

	a := newTestApplier(fs, cmd, nil, nil, nil)
	a.modePrefix = []string{root}

	cfg := &config.CloudConfig{}
	cfg.K3OS.ServerURL = "https://10.0.0.1:6443"

	err := a.ApplyK3S(cfg, true, false)
	require.NoError(t, err)

	// Verify K3S_URL env var is set
	assert.Contains(t, capturedEnv, "K3S_URL=https://10.0.0.1:6443")
	// Verify "agent" is the first arg (since K3sArgs is empty and ServerURL is set)
	require.NotEmpty(t, capturedArgs)
	assert.Equal(t, "agent", capturedArgs[0])

	fs.AssertExpectations(t)
}

func TestApplyK3S_ModeGetError(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	// Create a directory at the mode file path so os.ReadFile fails
	// with "is a directory" — this is NOT os.ErrNotExist so mode.Get
	// returns the error.
	modePath := root + "/run/k3os/mode"
	require.NoError(t, os.MkdirAll(modePath, 0o755))

	fs := &MockFileSystem{}
	cmd := &MockCommandRunner{}

	a := newTestApplier(fs, cmd, nil, nil, nil)
	a.modePrefix = []string{root}

	cfg := &config.CloudConfig{}

	err := a.ApplyK3S(cfg, true, false)
	require.Error(t, err)

	// No FS or Cmd calls should have been made
	fs.AssertExpectations(t)
	cmd.AssertExpectations(t)
}

// ---------------------------------------------------------------------------
// ApplyInstall
// ---------------------------------------------------------------------------

func TestApplyInstall(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		modeContent string
		setupMode   func(t *testing.T, root string)
		setupMock   func(cmd *MockCommandRunner)
		wantErr     bool
		errContains string
	}{
		{
			name:        "not install mode does not call Run",
			modeContent: "live",
			setupMock: func(_ *MockCommandRunner) {
				// Run must NOT be called
			},
			wantErr: false,
		},
		{
			name:        "install mode calls Run with k3os install",
			modeContent: "install",
			setupMock: func(cmd *MockCommandRunner) {
				cmd.On("Run", "k3os", "install").Return(nil)
			},
			wantErr: false,
		},
		{
			name: "mode.Get error is propagated",
			setupMode: func(t *testing.T, root string) {
				t.Helper()
				// Create a directory at the mode file path so os.ReadFile fails
				modePath := root + "/run/k3os/mode"
				require.NoError(t, os.MkdirAll(modePath, 0o755))
			},
			setupMock: func(_ *MockCommandRunner) {
				// No calls expected
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			if tc.setupMode != nil {
				tc.setupMode(t, root)
			} else {
				writeModeFile(t, root, tc.modeContent)
			}

			cmd := &MockCommandRunner{}
			tc.setupMock(cmd)

			a := newTestApplier(nil, cmd, nil, nil, nil)
			a.modePrefix = []string{root}

			cfg := &config.CloudConfig{}

			err := a.ApplyInstall(cfg)

			if tc.wantErr {
				require.Error(t, err)
				if tc.errContains != "" {
					assert.Contains(t, err.Error(), tc.errContains)
				}
			} else {
				require.NoError(t, err)
			}

			cmd.AssertExpectations(t)
		})
	}
}

func TestApplyDataSource(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		dataSources  []string
		setupMock    func(fs *MockFileSystem, capturedData *[]byte)
		wantErr      bool
		errContains  string
		checkContent func(t *testing.T, data []byte)
	}{
		{
			name:        "one data source writes correct command_args content",
			dataSources: []string{"cdrom"},
			setupMock: func(fs *MockFileSystem, capturedData *[]byte) {
				fs.On("WriteFile", "/etc/conf.d/cloud-config", mock.MatchedBy(func(data []byte) bool {
					*capturedData = data
					return true
				}), os.FileMode(0o644)).Return(nil)
			},
			wantErr: false,
			checkContent: func(t *testing.T, data []byte) {
				t.Helper()
				content := string(data)
				assert.Equal(t, "command_args=\"cdrom\"\n", content)
			},
		},
		{
			name:        "multiple data sources are joined with spaces",
			dataSources: []string{"cdrom", "url=http://example.com"},
			setupMock: func(fs *MockFileSystem, capturedData *[]byte) {
				fs.On("WriteFile", "/etc/conf.d/cloud-config", mock.MatchedBy(func(data []byte) bool {
					*capturedData = data
					return true
				}), os.FileMode(0o644)).Return(nil)
			},
			wantErr: false,
			checkContent: func(t *testing.T, data []byte) {
				t.Helper()
				content := string(data)
				assert.Equal(t, "command_args=\"cdrom url=http://example.com\"\n", content)
			},
		},
		{
			name:        "empty data sources list is a no-op",
			dataSources: []string{},
			setupMock: func(_ *MockFileSystem, _ *[]byte) {
				// No FS calls expected
			},
			wantErr: false,
		},
		{
			name:        "WriteFile error is propagated and wrapped",
			dataSources: []string{"cdrom"},
			setupMock: func(fs *MockFileSystem, _ *[]byte) {
				fs.On("WriteFile", "/etc/conf.d/cloud-config", mock.Anything, os.FileMode(0o644)).
					Return(errors.New("disk full"))
			},
			wantErr:     true,
			errContains: "failed to write to /etc/conf.d/cloud-config",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			fs := &MockFileSystem{}
			var capturedData []byte
			tc.setupMock(fs, &capturedData)

			a := newTestApplier(fs, nil, nil, nil, nil)
			cfg := &config.CloudConfig{}
			cfg.K3OS.DataSources = tc.dataSources

			err := a.ApplyDataSource(cfg)

			if tc.wantErr {
				require.Error(t, err)
				if tc.errContains != "" {
					assert.Contains(t, err.Error(), tc.errContains)
				}
			} else {
				require.NoError(t, err)
				if tc.checkContent != nil {
					tc.checkContent(t, capturedData)
				}
			}

			fs.AssertExpectations(t)
		})
	}
}
