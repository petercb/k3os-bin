//go:build linux

package virt

import (
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDMIDetector_Detect(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		files    map[string]string
		readErr  error
		expected []string
	}{
		{
			name: "KVM/QEMU detected via sys_vendor",
			files: map[string]string{
				"sys_vendor":   "QEMU",
				"product_name": "Standard PC (Q35 + ICH9, 2009)",
				"board_vendor": "QEMU",
				"bios_vendor":  "SeaBIOS",
			},
			expected: []string{"kvm"},
		},
		{
			name: "VMware detected via sys_vendor",
			files: map[string]string{
				"sys_vendor":   "VMware, Inc.",
				"product_name": "VMware Virtual Platform",
				"board_vendor": "Intel Corporation",
				"bios_vendor":  "Phoenix Technologies LTD",
			},
			expected: []string{"vmware"},
		},
		{
			name: "Hyper-V detected via sys_vendor and product_name",
			files: map[string]string{
				"sys_vendor":   "Microsoft Corporation",
				"product_name": "Virtual Machine",
				"board_vendor": "Microsoft Corporation",
				"bios_vendor":  "American Megatrends Inc.",
			},
			expected: []string{"hyperv"},
		},
		{
			name: "VirtualBox detected via board_vendor innotek",
			files: map[string]string{
				"sys_vendor":   "innotek GmbH",
				"product_name": "VirtualBox",
				"board_vendor": "Oracle Corporation",
				"bios_vendor":  "innotek GmbH",
			},
			expected: []string{"virtualbox"},
		},
		{
			name: "VirtualBox detected via Oracle sys_vendor",
			files: map[string]string{
				"sys_vendor":   "Oracle Corporation",
				"product_name": "VirtualBox",
				"board_vendor": "Oracle Corporation",
				"bios_vendor":  "innotek GmbH",
			},
			expected: []string{"virtualbox"},
		},
		{
			name: "no hypervisor detected returns nil",
			files: map[string]string{
				"sys_vendor":   "Dell Inc.",
				"product_name": "PowerEdge R740",
				"board_vendor": "Dell Inc.",
				"bios_vendor":  "Dell Inc.",
			},
			expected: nil,
		},
		{
			name:     "file read errors are non-fatal",
			files:    nil,
			readErr:  os.ErrNotExist,
			expected: nil,
		},
		{
			name:     "generic read error is non-fatal",
			files:    nil,
			readErr:  errors.New("permission denied"),
			expected: nil,
		},
		{
			name: "empty files return nil",
			files: map[string]string{
				"sys_vendor":   "",
				"product_name": "",
				"board_vendor": "",
				"bios_vendor":  "",
			},
			expected: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			d := &DMIDetector{
				BasePath: "/fake/dmi/",
				ReadFile: func(name string) ([]byte, error) {
					if tc.readErr != nil {
						return nil, tc.readErr
					}
					for filename, content := range tc.files {
						if name == "/fake/dmi/"+filename {
							return []byte(content), nil
						}
					}
					return nil, os.ErrNotExist
				},
			}

			result, err := d.Detect()
			require.NoError(t, err)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestNewDMIDetector(t *testing.T) {
	t.Parallel()

	d := NewDMIDetector()
	assert.Equal(t, "/sys/class/dmi/id/", d.BasePath)
	assert.NotNil(t, d.ReadFile)
}
