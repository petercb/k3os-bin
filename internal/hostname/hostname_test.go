package hostname

import (
	"errors"
	"os"
	"testing"

	"github.com/petercb/k3os-bin/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestSetHostname(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		hostname    string
		setupMock   func(fs *MockFileSystem, hn *MockHostnameSetter)
		wantErr     bool
		errContains string
	}{
		{
			name:     "empty hostname is no-op",
			hostname: "",
			setupMock: func(_ *MockFileSystem, _ *MockHostnameSetter) {
				// No calls expected
			},
			wantErr: false,
		},
		{
			name:     "SetHostname syscall error propagated",
			hostname: "myhost",
			setupMock: func(_ *MockFileSystem, hn *MockHostnameSetter) {
				hn.On("SetHostname", "myhost").Return(errors.New("syscall failed"))
			},
			wantErr:     true,
			errContains: "syscall failed",
		},
		{
			name:     "successful set triggers syncHostname",
			hostname: "myhost",
			setupMock: func(fs *MockFileSystem, hn *MockHostnameSetter) {
				hn.On("SetHostname", "myhost").Return(nil)
				// syncHostname calls Hostname()
				fs.On("Hostname").Return("myhost", nil)
				// writes /etc/hostname
				fs.On("WriteFile", "/etc/hostname", []byte("myhost\n"), os.FileMode(0o644)).Return(nil)
				// opens /etc/hosts
				hostsFile := &MockFile{}
				hostsFile.buf.WriteString("127.0.0.1 localhost\n127.0.1.1 oldhost\n")
				hostsFile.On("Close").Return(nil)
				fs.On("Open", "/etc/hosts").Return(hostsFile, nil)
				// writes updated /etc/hosts
				fs.On("WriteFile", "/etc/hosts", []byte("127.0.0.1 localhost\n127.0.1.1 myhost\n"), os.FileMode(0o600)).Return(nil)
			},
			wantErr: false,
		},
		{
			name:     "syncHostname Hostname returns empty is no-op after SetHostname",
			hostname: "myhost",
			setupMock: func(fs *MockFileSystem, hn *MockHostnameSetter) {
				hn.On("SetHostname", "myhost").Return(nil)
				fs.On("Hostname").Return("", nil)
				// no further calls since hostname is empty
			},
			wantErr: false,
		},
		{
			name:     "syncHostname Hostname error propagated",
			hostname: "myhost",
			setupMock: func(fs *MockFileSystem, hn *MockHostnameSetter) {
				hn.On("SetHostname", "myhost").Return(nil)
				fs.On("Hostname").Return("", errors.New("hostname error"))
			},
			wantErr:     true,
			errContains: "hostname error",
		},
		{
			name:     "syncHostname WriteFile /etc/hostname error propagated",
			hostname: "myhost",
			setupMock: func(fs *MockFileSystem, hn *MockHostnameSetter) {
				hn.On("SetHostname", "myhost").Return(nil)
				fs.On("Hostname").Return("myhost", nil)
				fs.On("WriteFile", "/etc/hostname", []byte("myhost\n"), os.FileMode(0o644)).Return(errors.New("write error"))
			},
			wantErr:     true,
			errContains: "write error",
		},
		{
			name:     "syncHostname Open /etc/hosts error propagated",
			hostname: "myhost",
			setupMock: func(fs *MockFileSystem, hn *MockHostnameSetter) {
				hn.On("SetHostname", "myhost").Return(nil)
				fs.On("Hostname").Return("myhost", nil)
				fs.On("WriteFile", "/etc/hostname", []byte("myhost\n"), os.FileMode(0o644)).Return(nil)
				fs.On("Open", "/etc/hosts").Return(nil, errors.New("open error"))
			},
			wantErr:     true,
			errContains: "open error",
		},
		{
			name:     "hosts file with no 127.0.1.1 line preserves all content",
			hostname: "myhost",
			setupMock: func(fs *MockFileSystem, hn *MockHostnameSetter) {
				hn.On("SetHostname", "myhost").Return(nil)
				fs.On("Hostname").Return("myhost", nil)
				fs.On("WriteFile", "/etc/hostname", []byte("myhost\n"), os.FileMode(0o644)).Return(nil)

				hostsFile := &MockFile{}
				hostsFile.buf.WriteString("127.0.0.1 localhost\n::1 localhost\n")
				hostsFile.On("Close").Return(nil)
				fs.On("Open", "/etc/hosts").Return(hostsFile, nil)

				// All original lines preserved, no 127.0.1.1 line added
				fs.On("WriteFile", "/etc/hosts", []byte("127.0.0.1 localhost\n::1 localhost\n"), os.FileMode(0o600)).Return(nil)
			},
			wantErr: false,
		},
		{
			name:     "hosts file with multiple lines keeps non-matching lines intact",
			hostname: "newhost",
			setupMock: func(fs *MockFileSystem, hn *MockHostnameSetter) {
				hn.On("SetHostname", "newhost").Return(nil)
				fs.On("Hostname").Return("newhost", nil)
				fs.On("WriteFile", "/etc/hostname", []byte("newhost\n"), os.FileMode(0o644)).Return(nil)

				hostsFile := &MockFile{}
				hostsFile.buf.WriteString("127.0.0.1 localhost\n127.0.1.1 oldhost\n10.0.0.1 server\n")
				hostsFile.On("Close").Return(nil)
				fs.On("Open", "/etc/hosts").Return(hostsFile, nil)

				fs.On("WriteFile", "/etc/hosts", []byte("127.0.0.1 localhost\n127.0.1.1 newhost\n10.0.0.1 server\n"), os.FileMode(0o600)).Return(nil)
			},
			wantErr: false,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			fs := &MockFileSystem{}
			hn := &MockHostnameSetter{}
			tc.setupMock(fs, hn)

			cfg := &config.CloudConfig{}
			cfg.Hostname = tc.hostname

			err := SetHostname(cfg, hn, fs)

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

func TestSetHostname_WriteHostsError(t *testing.T) {
	t.Parallel()

	fs := &MockFileSystem{}
	hn := &MockHostnameSetter{}

	hn.On("SetHostname", "myhost").Return(nil)
	fs.On("Hostname").Return("myhost", nil)
	fs.On("WriteFile", "/etc/hostname", []byte("myhost\n"), os.FileMode(0o644)).Return(nil)

	hostsFile := &MockFile{}
	hostsFile.buf.WriteString("127.0.1.1 old\n")
	hostsFile.On("Close").Return(nil)
	fs.On("Open", "/etc/hosts").Return(hostsFile, nil)
	fs.On("WriteFile", "/etc/hosts", mock.Anything, os.FileMode(0o600)).Return(errors.New("hosts write error"))

	cfg := &config.CloudConfig{}
	cfg.Hostname = "myhost"

	err := SetHostname(cfg, hn, fs)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "hosts write error")

	fs.AssertExpectations(t)
	hn.AssertExpectations(t)
}
