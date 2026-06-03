//go:build linux

// Package klog provides early boot logging backed by /dev/kmsg (the kernel
// ring buffer). It configures a slog.TextHandler that writes structured log
// lines through a kmsg.Writer, ensuring all messages appear in dmesg output.
// If /dev/kmsg is unavailable, it falls back transparently to os.Stderr.
package klog

import (
	"log/slog"
	"os"

	"github.com/petercb/k3os-bin/internal/mount"
	"github.com/siderolabs/go-kmsg"
	"golang.org/x/sys/unix"
)

// openKmsg is the function used to open /dev/kmsg; override in tests.
var openKmsg = defaultOpenKmsg

// ensureKmsg ensures /dev/kmsg is available by mounting devtmpfs if needed.
// During early boot (enter-root reexec, post-chroot), /dev may not have
// devtmpfs mounted yet, so /dev/kmsg may be missing.
// Override in tests.
var ensureKmsg = defaultEnsureKmsg

// Package-level vars for test injection.
var (
	statKmsgFn   = func() error { _, err := os.Stat("/dev/kmsg"); return err }
	mkdirAllFn   = os.MkdirAll
	forceMountFn = mount.ForceMount
)

func defaultOpenKmsg() (*os.File, error) {
	return os.OpenFile("/dev/kmsg", os.O_WRONLY|unix.O_CLOEXEC|unix.O_NONBLOCK|unix.O_NOCTTY, 0o666)
}

func defaultEnsureKmsg() error {
	// If /dev/kmsg already exists, the device is available -- skip mounting.
	if statKmsgFn() == nil {
		return nil
	}
	if err := mkdirAllFn("/dev", 0o755); err != nil {
		return err
	}
	return forceMountFn("none", "/dev", "devtmpfs", "nosuid,noexec")
}

// EarlyLogger holds the state of a kmsg-backed slog handler established
// during early boot.
type EarlyLogger struct {
	file     *os.File
	levelVar *slog.LevelVar
}

// Setup opens /dev/kmsg, wraps it with a kmsg.Writer for line splitting and
// truncation, creates a slog.TextHandler targeting that writer, and sets it as
// the slog default. If /dev/kmsg cannot be opened, it falls back to os.Stderr.
// Returns an EarlyLogger that allows changing the log level and closing the
// underlying file.
func Setup() *EarlyLogger {
	levelVar := &slog.LevelVar{}
	levelVar.Set(slog.LevelInfo)

	// Ensure the /dev/kmsg device node exists. If this fails, we proceed
	// anyway -- the existing stderr fallback handles the case where
	// /dev/kmsg still cannot be opened.
	_ = ensureKmsg()

	f, err := openKmsg()
	if err != nil {
		// Fall back to stderr when /dev/kmsg is not available (e.g. containers,
		// non-PID-1 contexts, or tests).
		handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: levelVar,
		})
		slog.SetDefault(slog.New(handler))
		slog.Warn("klog: failed to open /dev/kmsg, falling back to stderr", "error", err)
		return &EarlyLogger{file: nil, levelVar: levelVar}
	}

	writer := &kmsg.Writer{KmsgWriter: f}
	handler := slog.NewTextHandler(writer, &slog.HandlerOptions{
		Level: levelVar,
	})
	slog.SetDefault(slog.New(handler))

	return &EarlyLogger{file: f, levelVar: levelVar}
}

// SetDebug sets the log level to Debug.
func (l *EarlyLogger) SetDebug() {
	l.levelVar.Set(slog.LevelDebug)
}

// Level returns the shared LevelVar, allowing callers to adjust or inspect the
// current log level.
func (l *EarlyLogger) Level() *slog.LevelVar {
	return l.levelVar
}

// Close closes the underlying /dev/kmsg file if it was opened. Safe to call
// multiple times or when the file is nil.
func (l *EarlyLogger) Close() {
	if l.file != nil {
		_ = l.file.Close()
	}
}
