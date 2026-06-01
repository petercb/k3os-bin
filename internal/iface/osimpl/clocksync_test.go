//go:build linux

package osimpl

import (
	"testing"

	"github.com/petercb/k3os-bin/internal/iface"
	"github.com/stretchr/testify/assert"
)

// Compile-time interface check.
var _ iface.ClockSyncer = (*RTCClockSyncer)(nil)

func TestRTCClockSyncer_SyncRTC_NoDevice(t *testing.T) {
	t.Parallel()

	// In a test/CI environment without /dev/rtc, SyncRTC should return
	// an error without panicking.
	cs := RTCClockSyncer{}
	err := cs.SyncRTC()
	// We expect an error since /dev/rtc is unlikely to exist in CI.
	assert.Error(t, err)
}
