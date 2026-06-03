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

func TestSetup_EnsureKmsgSucceeds_ThenOpenSucceeds(t *testing.T) {
	// Create a pipe to simulate /dev/kmsg.
	r, w, err := os.Pipe()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = r.Close()
		_ = w.Close()
	})

	// Record call order to prove ensureKmsg runs before openKmsg.
	var callOrder []string

	origOpen := openKmsg
	openKmsg = func() (*os.File, error) {
		callOrder = append(callOrder, "open")
		return w, nil
	}
	t.Cleanup(func() { openKmsg = origOpen })

	origEnsure := ensureKmsg
	ensureKmsg = func() error {
		callOrder = append(callOrder, "ensure")
		return nil
	}
	t.Cleanup(func() { ensureKmsg = origEnsure })

	logger := Setup()
	require.NotNil(t, logger)
	t.Cleanup(func() { logger.Close() })

	// Verify ensureKmsg is called before openKmsg.
	require.Equal(t, []string{"ensure", "open"}, callOrder)

	// Logger should have successfully opened the file.
	assert.NotNil(t, logger.file)
}

func TestSetup_EnsureKmsgFails_FallbackToStderr(t *testing.T) {
	origOpen := openKmsg
	openKmsg = func() (*os.File, error) {
		return nil, errors.New("no such file or directory")
	}
	t.Cleanup(func() { openKmsg = origOpen })

	origEnsure := ensureKmsg
	ensureKmsg = func() error {
		return errors.New("operation not permitted")
	}
	t.Cleanup(func() { ensureKmsg = origEnsure })

	logger := Setup()
	require.NotNil(t, logger)
	t.Cleanup(func() { logger.Close() })

	// When ensureKmsg fails and openKmsg also fails, fallback to stderr.
	assert.Nil(t, logger.file)

	// slog should still be functional (writing to stderr fallback).
	assert.NotPanics(t, func() { slog.Info("fallback after ensure failure") })
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

func TestDefaultEnsureKmsg_SkipsWhenKmsgExists(t *testing.T) {
	origStatKmsg := statKmsgFn
	origMkdirAll := mkdirAllFn
	origForceMount := forceMountFn
	t.Cleanup(func() {
		statKmsgFn = origStatKmsg
		mkdirAllFn = origMkdirAll
		forceMountFn = origForceMount
	})

	// /dev/kmsg already exists.
	statKmsgFn = func() error { return nil }

	mountCalled := false
	forceMountFn = func(_, _, _, _ string) error {
		mountCalled = true
		return nil
	}

	err := defaultEnsureKmsg()
	require.NoError(t, err)
	assert.False(t, mountCalled, "mount should not be called when /dev/kmsg already exists")
}

func TestDefaultEnsureKmsg_MountsDevtmpfs(t *testing.T) {
	origStatKmsg := statKmsgFn
	origMkdirAll := mkdirAllFn
	origForceMount := forceMountFn
	t.Cleanup(func() {
		statKmsgFn = origStatKmsg
		mkdirAllFn = origMkdirAll
		forceMountFn = origForceMount
	})

	// /dev/kmsg does not exist.
	statKmsgFn = func() error { return errors.New("no such file or directory") }

	mkdirCalled := false
	mkdirAllFn = func(path string, perm os.FileMode) error {
		mkdirCalled = true
		assert.Equal(t, "/dev", path)
		assert.Equal(t, os.FileMode(0o755), perm)
		return nil
	}

	var mountDevice, mountTarget, mountType, mountOpts string
	forceMountFn = func(device, target, mType, options string) error {
		mountDevice = device
		mountTarget = target
		mountType = mType
		mountOpts = options
		return nil
	}

	err := defaultEnsureKmsg()
	require.NoError(t, err)
	assert.True(t, mkdirCalled, "mkdirAll should be called")
	assert.Equal(t, "none", mountDevice)
	assert.Equal(t, "/dev", mountTarget)
	assert.Equal(t, "devtmpfs", mountType)
	assert.Equal(t, "nosuid,noexec", mountOpts)
}

func TestDefaultEnsureKmsg_MkdirAllFails_ReturnsError(t *testing.T) {
	origStatKmsg := statKmsgFn
	origMkdirAll := mkdirAllFn
	origForceMount := forceMountFn
	t.Cleanup(func() {
		statKmsgFn = origStatKmsg
		mkdirAllFn = origMkdirAll
		forceMountFn = origForceMount
	})

	// /dev/kmsg does not exist.
	statKmsgFn = func() error { return errors.New("no such file or directory") }

	mkdirErr := errors.New("mkdir /dev: read-only file system")
	mkdirAllFn = func(_ string, _ os.FileMode) error {
		return mkdirErr
	}

	forceMountFn = func(_, _, _, _ string) error {
		t.Fatal("mount should not be called when mkdirAll fails")
		return nil
	}

	err := defaultEnsureKmsg()
	require.Error(t, err)
	assert.Equal(t, mkdirErr, err)
}

func TestDefaultEnsureKmsg_MountFails_ReturnsError(t *testing.T) {
	origStatKmsg := statKmsgFn
	origMkdirAll := mkdirAllFn
	origForceMount := forceMountFn
	t.Cleanup(func() {
		statKmsgFn = origStatKmsg
		mkdirAllFn = origMkdirAll
		forceMountFn = origForceMount
	})

	// /dev/kmsg does not exist.
	statKmsgFn = func() error { return errors.New("no such file or directory") }

	mkdirAllFn = func(_ string, _ os.FileMode) error {
		return nil
	}

	mountErr := errors.New("mount failed: no devtmpfs support")
	forceMountFn = func(_, _, _, _ string) error {
		return mountErr
	}

	err := defaultEnsureKmsg()
	require.Error(t, err)
	assert.Equal(t, mountErr, err)
}
