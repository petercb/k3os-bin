package cliinstall

import (
	"os"
	"testing"

	"github.com/petercb/k3os-bin/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockBlockProber is a test implementation of iface.BlockProber.
type mockBlockProber struct {
	disks []string
	err   error
}

func (m *mockBlockProber) FindByLabel(_ string) (string, error) {
	return "", nil
}

func (m *mockBlockProber) ListDisks() ([]string, error) {
	return m.disks, m.err
}

func (m *mockBlockProber) ProbeFS(_ string) string {
	return ""
}

func TestAskInstallDevice_AlreadySet(t *testing.T) {
	t.Parallel()

	cfg := &config.CloudConfig{}
	cfg.K3OS.Install = &config.Install{Device: "/dev/sda"}

	err := AskInstallDevice(cfg)
	require.NoError(t, err)
	assert.Equal(t, "/dev/sda", cfg.K3OS.Install.Device)
}

func TestAskInstallDevice_ListDisksError(t *testing.T) {
	orig := blockProber
	t.Cleanup(func() { blockProber = orig })

	blockProber = &mockBlockProber{err: os.ErrNotExist}

	cfg := &config.CloudConfig{}
	cfg.K3OS.Install = &config.Install{}

	err := AskInstallDevice(cfg)
	require.Error(t, err)
}

func TestAskInstallDevice_SingleDisk(t *testing.T) {
	orig := blockProber
	t.Cleanup(func() { blockProber = orig })

	blockProber = &mockBlockProber{disks: []string{"sda"}}

	cfg := &config.CloudConfig{}
	cfg.K3OS.Install = &config.Install{}

	// PromptFormattedOptions returns 0 immediately when there's only 1 option
	err := AskInstallDevice(cfg)
	require.NoError(t, err)
	assert.Equal(t, "/dev/sda", cfg.K3OS.Install.Device)
}

func TestAskInstallDevice_UsesBlockProber(t *testing.T) {
	orig := blockProber
	t.Cleanup(func() { blockProber = orig })

	// With a single disk, PromptFormattedOptions returns immediately (index 0)
	blockProber = &mockBlockProber{disks: []string{"nvme0n1"}}

	cfg := &config.CloudConfig{}
	cfg.K3OS.Install = &config.Install{}

	err := AskInstallDevice(cfg)
	require.NoError(t, err)
	assert.Equal(t, "/dev/nvme0n1", cfg.K3OS.Install.Device)
}
