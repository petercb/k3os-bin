//go:build linux

package osimpl

import (
	"github.com/u-root/u-root/pkg/rtc"
	"golang.org/x/sys/unix"
)

// RTCClockSyncer implements iface.ClockSyncer by reading the hardware
// RTC and setting the system clock via settimeofday(2).
type RTCClockSyncer struct{}

// SyncRTC reads the hardware clock from /dev/rtc and sets the system
// clock to match (equivalent to hwclock --hctosys --utc).
func (RTCClockSyncer) SyncRTC() error {
	r, err := rtc.OpenRTC()
	if err != nil {
		return err
	}
	defer r.Close() //nolint:errcheck

	t, err := r.Read()
	if err != nil {
		return err
	}

	// Usec is left at zero intentionally: hardware RTCs provide only
	// second-level precision, so sub-second resolution is not available.
	tv := unix.Timeval{Sec: t.Unix()}
	return unix.Settimeofday(&tv)
}
