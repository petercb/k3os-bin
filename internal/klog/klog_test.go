//go:build linux

package klog

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetup_Success(t *testing.T) {
	// Create a pipe to simulate /dev/kmsg.
	r, w, err := os.Pipe()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = r.Close()
		_ = w.Close()
	})

	orig := openKmsg
	openKmsg = func() (*os.File, error) { return w, nil }
	t.Cleanup(func() { openKmsg = orig })

	logger := Setup()
	require.NotNil(t, logger)
	t.Cleanup(func() { logger.Close() })

	// Verify slog default is configured and writes to our pipe.
	slog.Info("test message", "key", "value")

	// Close the write end so Read doesn't block.
	_ = w.Close()

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	assert.Contains(t, output, "test message")
	assert.Contains(t, output, "key=value")
}

func TestSetup_OpenFails_FallsBackToStderr(t *testing.T) {
	orig := openKmsg
	openKmsg = func() (*os.File, error) {
		return nil, errors.New("no such device")
	}
	t.Cleanup(func() { openKmsg = orig })

	logger := Setup()
	require.NotNil(t, logger)
	t.Cleanup(func() { logger.Close() })

	// The file should be nil (no /dev/kmsg was opened).
	assert.Nil(t, logger.file)

	// slog should still be functional (writing to stderr fallback).
	// Just verify it doesn't panic.
	slog.Info("fallback message")
}

func TestSetDebug(t *testing.T) {
	orig := openKmsg
	openKmsg = func() (*os.File, error) {
		return nil, errors.New("no device")
	}
	t.Cleanup(func() { openKmsg = orig })

	logger := Setup()
	require.NotNil(t, logger)
	t.Cleanup(func() { logger.Close() })

	// Default level is Info.
	assert.Equal(t, slog.LevelInfo, logger.Level().Level())

	// After SetDebug, level should be Debug.
	logger.SetDebug()
	assert.Equal(t, slog.LevelDebug, logger.Level().Level())
}

func TestLevel_ReturnsLevelVar(t *testing.T) {
	orig := openKmsg
	openKmsg = func() (*os.File, error) {
		return nil, errors.New("no device")
	}
	t.Cleanup(func() { openKmsg = orig })

	logger := Setup()
	require.NotNil(t, logger)
	t.Cleanup(func() { logger.Close() })

	lv := logger.Level()
	require.NotNil(t, lv)
	assert.Equal(t, slog.LevelInfo, lv.Level())

	// Changing level via the returned LevelVar is reflected.
	lv.Set(slog.LevelWarn)
	assert.Equal(t, slog.LevelWarn, logger.Level().Level())
}

func TestClose_ClosesFile(t *testing.T) {
	r, w, err := os.Pipe()
	require.NoError(t, err)
	_ = r.Close()

	orig := openKmsg
	openKmsg = func() (*os.File, error) { return w, nil }
	t.Cleanup(func() { openKmsg = orig })

	logger := Setup()
	require.NotNil(t, logger)

	// Close should not error on first call.
	logger.Close()

	// Writing to the closed file should fail.
	_, writeErr := w.Write([]byte("test"))
	assert.Error(t, writeErr)
}

func TestClose_NilFile_NoPanic(t *testing.T) {
	orig := openKmsg
	openKmsg = func() (*os.File, error) {
		return nil, errors.New("no device")
	}
	t.Cleanup(func() { openKmsg = orig })

	logger := Setup()
	require.NotNil(t, logger)

	// Close with nil file should not panic.
	assert.NotPanics(t, func() { logger.Close() })
}

func TestSetup_WritesStructuredOutput(t *testing.T) {
	r, w, err := os.Pipe()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = r.Close()
		_ = w.Close()
	})

	orig := openKmsg
	openKmsg = func() (*os.File, error) { return w, nil }
	t.Cleanup(func() { openKmsg = orig })

	logger := Setup()
	require.NotNil(t, logger)

	slog.Warn("resource limit failed", "resource", "RLIMIT_NOFILE", "error", "operation not permitted")

	_ = w.Close()

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	assert.Contains(t, output, "level=WARN")
	assert.Contains(t, output, "resource limit failed")
	assert.Contains(t, output, "resource=RLIMIT_NOFILE")
}

func TestSetup_KmsgWriterSplitsLines(t *testing.T) {
	r, w, err := os.Pipe()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = r.Close()
		_ = w.Close()
	})

	orig := openKmsg
	openKmsg = func() (*os.File, error) { return w, nil }
	t.Cleanup(func() { openKmsg = orig })

	logger := Setup()
	require.NotNil(t, logger)

	// Write a long message that would be truncated by kmsg.Writer's MaxLineLength.
	longVal := strings.Repeat("x", 1500)
	slog.Info("long", "data", longVal)

	_ = w.Close()

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	// The kmsg.Writer truncates lines longer than MaxLineLength (976) with "..."
	assert.Contains(t, output, "...")
}
