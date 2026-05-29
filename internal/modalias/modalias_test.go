package modalias

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLookup(t *testing.T) {
	tests := []struct {
		name     string
		aliases  map[string]string
		input    string
		expected string
	}{
		{
			name: "star wildcard matches",
			aliases: map[string]string{
				"usb:v*p*": "usb_storage",
			},
			input:    "usb:v1234p5678",
			expected: "usb_storage",
		},
		{
			name: "question mark single-char wildcard",
			aliases: map[string]string{
				"pci:d000?sv*": "pci_driver",
			},
			input:    "pci:d000Asv1234",
			expected: "pci_driver",
		},
		{
			name: "character class matching",
			aliases: map[string]string{
				"usb:v[0-9]*": "usb_class",
			},
			input:    "usb:v1ABC",
			expected: "usb_class",
		},
		{
			name: "no match returns original name",
			aliases: map[string]string{
				"usb:v*p*": "usb_storage",
			},
			input:    "pci:d0001sv1234",
			expected: "pci:d0001sv1234",
		},
		{
			name:     "empty aliases returns original name",
			aliases:  map[string]string{},
			input:    "usb:v1234p5678",
			expected: "usb:v1234p5678",
		},
		{
			name: "malformed pattern skipped gracefully",
			aliases: map[string]string{
				"usb:v[abc": "bad_module",
			},
			input:    "usb:vABC",
			expected: "usb:vABC",
		},
		{
			name: "multiple patterns only one matches",
			aliases: map[string]string{
				"pci:d*":    "pci_driver",
				"usb:v*p*":  "usb_storage",
				"acpi:ABC*": "acpi_module",
			},
			input:    "usb:v1234p5678",
			expected: "usb_storage",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mod := ModuleAliases{aliases: tc.aliases}
			result := mod.Lookup(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}
