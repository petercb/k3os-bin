//go:build linux

package testmode

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeStat returns a fake FileInfo for testing.
func fakeStat(_ string) (os.FileInfo, error) {
	return fakeFileInfo{}, nil
}

// fakeFileInfo implements os.FileInfo for testing.
type fakeFileInfo struct{}

func (fakeFileInfo) Name() string       { return "fake" }
func (fakeFileInfo) Size() int64        { return 0 }
func (fakeFileInfo) Mode() os.FileMode  { return 0 }
func (fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (fakeFileInfo) IsDir() bool        { return false }
func (fakeFileInfo) Sys() any           { return nil }

func TestVerifier_Run_AllPassing(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	var rebootCalled bool

	v := &Verifier{
		StatFunc: fakeStat,
		ReadFileFunc: func(name string) ([]byte, error) {
			switch name {
			case "/etc/passwd":
				return []byte("root:x:0:0:root:/root:/bin/sh\nrancher:x:1000:1000::/home/rancher:/bin/bash\n"), nil
			case "/run/k3os/mode":
				return []byte("disk\n"), nil
			default:
				return nil, errors.New("not found")
			}
		},
		HostnameFunc: func() (string, error) {
			return "k3os-node", nil
		},
		RebootFunc: func() error {
			rebootCalled = true
			return nil
		},
		Output: &buf,
	}

	err := v.Run()
	require.NoError(t, err)
	assert.True(t, rebootCalled, "RebootFunc should be called")

	output := buf.String()
	assert.Contains(t, output, ResultStart)
	assert.Contains(t, output, ResultEnd)

	// Extract JSON between delimiters.
	jsonStr := extractJSON(t, output)
	var results Results
	require.NoError(t, json.Unmarshal([]byte(jsonStr), &results))

	assert.True(t, results.Passed)
	assert.Equal(t, "8/8 checks passed", results.Summary)
	assert.Len(t, results.Phases, 4)

	// Verify each phase passed.
	for _, phase := range results.Phases {
		assert.True(t, phase.Passed, "phase %s should pass", phase.Name)
		for _, check := range phase.Checks {
			assert.True(t, check.Passed, "check %s should pass", check.Name)
		}
	}
}

func TestVerifier_Run_SomeFailing(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	var rebootCalled bool

	v := &Verifier{
		StatFunc: func(name string) (os.FileInfo, error) {
			switch name {
			case "/proc/self/status":
				return nil, errors.New("not found")
			case "/run/k3os":
				return fakeFileInfo{}, nil
			default:
				return nil, errors.New("not found")
			}
		},
		ReadFileFunc: func(name string) ([]byte, error) {
			switch name {
			case "/etc/passwd":
				return []byte("root:x:0:0:root:/root:/bin/sh\nrancher:x:1000:1000::/home/rancher:/bin/bash\n"), nil
			case "/run/k3os/mode":
				return nil, errors.New("file not found")
			default:
				return nil, errors.New("not found")
			}
		},
		HostnameFunc: func() (string, error) {
			return "k3os-node", nil
		},
		RebootFunc: func() error {
			rebootCalled = true
			return nil
		},
		Output: &buf,
	}

	err := v.Run()
	require.NoError(t, err)
	assert.True(t, rebootCalled)

	jsonStr := extractJSON(t, buf.String())
	var results Results
	require.NoError(t, json.Unmarshal([]byte(jsonStr), &results))

	assert.False(t, results.Passed)
	assert.Equal(t, "4/8 checks passed", results.Summary)

	// Bootstrap phase: proc_mounted fails, etc_populated passes.
	bootstrap := results.Phases[0]
	assert.Equal(t, "bootstrap", bootstrap.Name)
	assert.False(t, bootstrap.Passed)
	assert.False(t, bootstrap.Checks[0].Passed) // proc_mounted
	assert.True(t, bootstrap.Checks[1].Passed)  // etc_populated

	// Mode detection phase: run_k3os_exists passes, mode_file fails.
	modeDetection := results.Phases[1]
	assert.Equal(t, "mode_detection", modeDetection.Name)
	assert.False(t, modeDetection.Passed)
	assert.True(t, modeDetection.Checks[0].Passed)  // run_k3os_exists
	assert.False(t, modeDetection.Checks[1].Passed) // mode_file

	// Finalization phase: hostname passes.
	finalization := results.Phases[3]
	assert.Equal(t, "finalization", finalization.Name)
	assert.True(t, finalization.Passed)
}

func TestVerifier_Run_InvalidMode(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	v := &Verifier{
		StatFunc: fakeStat,
		ReadFileFunc: func(name string) ([]byte, error) {
			switch name {
			case "/etc/passwd":
				return []byte("rancher:x:1000:1000::/home/rancher:/bin/bash\n"), nil
			case "/run/k3os/mode":
				return []byte("invalid_mode\n"), nil
			default:
				return nil, errors.New("not found")
			}
		},
		HostnameFunc: func() (string, error) {
			return "host1", nil
		},
		RebootFunc: func() error { return nil },
		Output:     &buf,
	}

	err := v.Run()
	require.NoError(t, err)

	jsonStr := extractJSON(t, buf.String())
	var results Results
	require.NoError(t, json.Unmarshal([]byte(jsonStr), &results))

	assert.False(t, results.Passed)

	// Find mode_file check.
	modePhase := results.Phases[1]
	assert.Equal(t, "mode_detection", modePhase.Name)
	modeFileCheck := modePhase.Checks[1]
	assert.Equal(t, "mode_file", modeFileCheck.Name)
	assert.False(t, modeFileCheck.Passed)
	assert.Contains(t, modeFileCheck.Detail, "invalid mode")
}

func TestVerifier_Run_EmptyHostname(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	v := &Verifier{
		StatFunc: fakeStat,
		ReadFileFunc: func(name string) ([]byte, error) {
			switch name {
			case "/etc/passwd":
				return []byte("rancher:x:1000:1000::/home/rancher:/bin/bash\n"), nil
			case "/run/k3os/mode":
				return []byte("live\n"), nil
			default:
				return nil, errors.New("not found")
			}
		},
		HostnameFunc: func() (string, error) {
			return "", nil
		},
		RebootFunc: func() error { return nil },
		Output:     &buf,
	}

	err := v.Run()
	require.NoError(t, err)

	jsonStr := extractJSON(t, buf.String())
	var results Results
	require.NoError(t, json.Unmarshal([]byte(jsonStr), &results))

	assert.False(t, results.Passed)
	finalization := results.Phases[3]
	assert.False(t, finalization.Passed)
	assert.Equal(t, "hostname_set", finalization.Checks[0].Name)
	assert.False(t, finalization.Checks[0].Passed)
	assert.Contains(t, finalization.Checks[0].Detail, "empty")
}

func TestVerifier_Run_ExpectedModeMismatch(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	v := &Verifier{
		StatFunc: fakeStat,
		ReadFileFunc: func(name string) ([]byte, error) {
			switch name {
			case "/etc/passwd":
				return []byte("rancher:x:1000:1000::/home/rancher:/bin/bash\n"), nil
			case "/run/k3os/mode":
				return []byte("local\n"), nil
			case "/proc/cmdline":
				return []byte("console=ttyS0 k3os.test_mode k3os.test_expected_mode=disk k3os.debug\n"), nil
			default:
				return nil, errors.New("not found")
			}
		},
		HostnameFunc: func() (string, error) {
			return "k3os-node", nil
		},
		RebootFunc: func() error { return nil },
		Output:     &buf,
	}

	err := v.Run()
	require.NoError(t, err)

	jsonStr := extractJSON(t, buf.String())
	var results Results
	require.NoError(t, json.Unmarshal([]byte(jsonStr), &results))

	assert.False(t, results.Passed)

	// Find the expected_mode check in the mode_detection phase.
	modePhase := results.Phases[1]
	assert.Equal(t, "mode_detection", modePhase.Name)
	assert.False(t, modePhase.Passed)

	var expectedCheck *Check
	for i := range modePhase.Checks {
		if modePhase.Checks[i].Name == "expected_mode" {
			expectedCheck = &modePhase.Checks[i]
			break
		}
	}
	require.NotNil(t, expectedCheck)
	assert.False(t, expectedCheck.Passed)
	assert.Contains(t, expectedCheck.Detail, `expected mode "disk" but got "local"`)
}

func TestVerifier_Run_ExpectedModeMatches(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	v := &Verifier{
		StatFunc: fakeStat,
		ReadFileFunc: func(name string) ([]byte, error) {
			switch name {
			case "/etc/passwd":
				return []byte("rancher:x:1000:1000::/home/rancher:/bin/bash\n"), nil
			case "/run/k3os/mode":
				return []byte("disk\n"), nil
			case "/proc/cmdline":
				return []byte("console=ttyS0 k3os.test_mode k3os.test_expected_mode=disk k3os.debug\n"), nil
			default:
				return nil, errors.New("not found")
			}
		},
		HostnameFunc: func() (string, error) {
			return "k3os-node", nil
		},
		RebootFunc: func() error { return nil },
		Output:     &buf,
	}

	err := v.Run()
	require.NoError(t, err)

	jsonStr := extractJSON(t, buf.String())
	var results Results
	require.NoError(t, json.Unmarshal([]byte(jsonStr), &results))

	assert.True(t, results.Passed)

	// Find the expected_mode check.
	modePhase := results.Phases[1]
	var expectedCheck *Check
	for i := range modePhase.Checks {
		if modePhase.Checks[i].Name == "expected_mode" {
			expectedCheck = &modePhase.Checks[i]
			break
		}
	}
	require.NotNil(t, expectedCheck)
	assert.True(t, expectedCheck.Passed)
	assert.Contains(t, expectedCheck.Detail, `mode matches expected: "disk"`)
}

func TestVerifier_Run_OutputFormat(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	v := &Verifier{
		StatFunc: fakeStat,
		ReadFileFunc: func(name string) ([]byte, error) {
			switch name {
			case "/etc/passwd":
				return []byte("rancher:x:1000:1000::/home/rancher:/bin/bash\n"), nil
			case "/run/k3os/mode":
				return []byte("local\n"), nil
			default:
				return nil, errors.New("not found")
			}
		},
		HostnameFunc: func() (string, error) {
			return "test-host", nil
		},
		RebootFunc: func() error { return nil },
		Output:     &buf,
	}

	err := v.Run()
	require.NoError(t, err)

	output := buf.String()

	// Verify delimiters are present and in correct order.
	startIdx := strings.Index(output, ResultStart)
	endIdx := strings.Index(output, ResultEnd)
	require.Greater(t, startIdx, -1, "start delimiter should be present")
	require.Greater(t, endIdx, -1, "end delimiter should be present")
	require.Greater(t, endIdx, startIdx, "end delimiter should come after start")

	// Verify JSON is valid between delimiters.
	jsonStr := extractJSON(t, output)
	assert.True(t, json.Valid([]byte(jsonStr)), "output between delimiters should be valid JSON")
}

func TestVerifier_Run_RebootFuncCalled(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	var rebootCalled bool

	v := &Verifier{
		StatFunc: fakeStat,
		ReadFileFunc: func(name string) ([]byte, error) {
			switch name {
			case "/etc/passwd":
				return []byte("rancher:x:1000:1000::/home/rancher:/bin/bash\n"), nil
			case "/run/k3os/mode":
				return []byte("disk\n"), nil
			default:
				return nil, errors.New("not found")
			}
		},
		HostnameFunc: func() (string, error) {
			return "node", nil
		},
		RebootFunc: func() error {
			rebootCalled = true
			return nil
		},
		Output: &buf,
	}

	err := v.Run()
	require.NoError(t, err)
	assert.True(t, rebootCalled, "RebootFunc must be called after output")
}

func TestVerifier_Run_RebootFuncError(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	v := &Verifier{
		StatFunc: fakeStat,
		ReadFileFunc: func(name string) ([]byte, error) {
			switch name {
			case "/etc/passwd":
				return []byte("rancher:x:1000:1000::/home/rancher:/bin/bash\n"), nil
			case "/run/k3os/mode":
				return []byte("disk\n"), nil
			default:
				return nil, errors.New("not found")
			}
		},
		HostnameFunc: func() (string, error) {
			return "node", nil
		},
		RebootFunc: func() error {
			return errors.New("reboot failed")
		},
		Output: &buf,
	}

	err := v.Run()
	assert.EqualError(t, err, "reboot failed")

	// Output should still have been written before the error.
	assert.Contains(t, buf.String(), ResultStart)
	assert.Contains(t, buf.String(), ResultEnd)
}

// extractJSON extracts the JSON string between the result delimiters.
func extractJSON(t *testing.T, output string) string {
	t.Helper()
	startIdx := strings.Index(output, ResultStart)
	endIdx := strings.Index(output, ResultEnd)
	require.Greater(t, startIdx, -1, "start delimiter not found")
	require.Greater(t, endIdx, startIdx, "end delimiter not found after start")

	between := output[startIdx+len(ResultStart) : endIdx]
	return strings.TrimSpace(between)
}
