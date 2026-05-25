package ssh

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindUserHomeDir(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		input       string
		username    string
		wantUID     int
		wantGID     int
		wantHomeDir string
		wantErr     bool
	}{
		{
			name:        "valid single user",
			input:       "rancher:x:1000:1000::/home/rancher:/bin/sh\n",
			username:    "rancher",
			wantUID:     1000,
			wantGID:     1000,
			wantHomeDir: "/home/rancher",
			wantErr:     false,
		},
		{
			name:        "multiple users finds correct one",
			input:       "root:x:0:0:root:/root:/bin/bash\nrancher:x:1000:1000::/home/rancher:/bin/sh\nnobody:x:65534:65534::/nonexistent:/usr/sbin/nologin\n",
			username:    "rancher",
			wantUID:     1000,
			wantGID:     1000,
			wantHomeDir: "/home/rancher",
			wantErr:     false,
		},
		{
			name:        "user not found returns zero values",
			input:       "root:x:0:0:root:/root:/bin/bash\n",
			username:    "rancher",
			wantUID:     0,
			wantGID:     0,
			wantHomeDir: "",
			wantErr:     false,
		},
		{
			name:        "malformed line fewer than 6 fields",
			input:       "rancher:x:1000\n",
			username:    "rancher",
			wantUID:     0,
			wantGID:     0,
			wantHomeDir: "",
			wantErr:     false,
		},
		{
			name:        "non-numeric uid returns error",
			input:       "rancher:x:abc:1000::/home/rancher:/bin/sh\n",
			username:    "rancher",
			wantUID:     -1,
			wantGID:     -1,
			wantHomeDir: "",
			wantErr:     true,
		},
		{
			name:        "non-numeric gid returns error",
			input:       "rancher:x:1000:abc::/home/rancher:/bin/sh\n",
			username:    "rancher",
			wantUID:     -1,
			wantGID:     -1,
			wantHomeDir: "",
			wantErr:     true,
		},
		{
			name:        "empty input returns zero values",
			input:       "",
			username:    "rancher",
			wantUID:     0,
			wantGID:     0,
			wantHomeDir: "",
			wantErr:     false,
		},
		{
			name:        "username prefix match - rancher2 matches rancher due to HasPrefix",
			input:       "rancher2:x:2000:2000::/home/rancher2:/bin/sh\n",
			username:    "rancher",
			wantUID:     2000,
			wantGID:     2000,
			wantHomeDir: "/home/rancher2",
			wantErr:     false,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			uid, gid, homeDir, err := findUserHomeDir([]byte(tc.input), tc.username)

			if tc.wantErr {
				require.Error(t, err)
				assert.Equal(t, tc.wantUID, uid)
				assert.Equal(t, tc.wantGID, gid)
				assert.Equal(t, tc.wantHomeDir, homeDir)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.wantUID, uid)
				assert.Equal(t, tc.wantGID, gid)
				assert.Equal(t, tc.wantHomeDir, homeDir)
			}
		})
	}
}
