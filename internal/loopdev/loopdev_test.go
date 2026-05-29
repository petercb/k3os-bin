//go:build linux

package loopdev

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockSyscaller is a test double for the syscaller interface.
type mockSyscaller struct {
	openCalls  []openCall
	openIdx    int
	closeCalls []int

	ioctlRetIntFn      func(fd int, req uint) (int, error)
	ioctlSetIntFn      func(fd int, req uint, val int) error
	ioctlGetStatus64Fn func(fd int, info *loopInfo64) error
	ioctlSetStatus64Fn func(fd int, info *loopInfo64) error
}

type openCall struct {
	path string
	fd   int
	err  error
}

func (m *mockSyscaller) Open(path string, _ int, _ uint32) (int, error) {
	if m.openIdx >= len(m.openCalls) {
		return -1, errors.New("unexpected Open call")
	}
	call := m.openCalls[m.openIdx]
	m.openIdx++
	if call.path != "" && call.path != path {
		return -1, errors.New("unexpected path: " + path)
	}
	return call.fd, call.err
}

func (m *mockSyscaller) Close(fd int) error {
	m.closeCalls = append(m.closeCalls, fd)
	return nil
}

func (m *mockSyscaller) IoctlRetInt(fd int, req uint) (int, error) {
	if m.ioctlRetIntFn != nil {
		return m.ioctlRetIntFn(fd, req)
	}
	return 0, nil
}

func (m *mockSyscaller) IoctlSetInt(fd int, req uint, val int) error {
	if m.ioctlSetIntFn != nil {
		return m.ioctlSetIntFn(fd, req, val)
	}
	return nil
}

func (m *mockSyscaller) IoctlLoopGetStatus64(fd int, info *loopInfo64) error {
	if m.ioctlGetStatus64Fn != nil {
		return m.ioctlGetStatus64Fn(fd, info)
	}
	return nil
}

func (m *mockSyscaller) IoctlLoopSetStatus64(fd int, info *loopInfo64) error {
	if m.ioctlSetStatus64Fn != nil {
		return m.ioctlSetStatus64Fn(fd, info)
	}
	return nil
}

func TestAttach_HappyPath(t *testing.T) {
	t.Parallel()

	var capturedInfo *loopInfo64
	var setFdVal int
	mock := &mockSyscaller{
		openCalls: []openCall{
			{path: "/test/backing.img", fd: 10, err: nil},    // backing file
			{path: loopControlPath, fd: 11, err: nil},        // loop-control
			{path: loopDevicePrefix + "3", fd: 12, err: nil}, // loop device
		},
		ioctlRetIntFn: func(fd int, req uint) (int, error) {
			if req == loopCtlGetFree {
				assert.Equal(t, 11, fd)
				return 3, nil
			}
			return 0, nil
		},
		ioctlSetIntFn: func(fd int, req uint, val int) error {
			if req == loopSetFd {
				assert.Equal(t, 12, fd)
				setFdVal = val
			}
			return nil
		},
		ioctlSetStatus64Fn: func(fd int, info *loopInfo64) error {
			assert.Equal(t, 12, fd)
			capturedInfo = info
			return nil
		},
	}

	a := newAttacherWith(mock)
	dev, err := a.Attach("/test/backing.img", 512, true)
	require.NoError(t, err)
	require.NotNil(t, dev)

	assert.Equal(t, "/dev/loop3", dev.Path())
	assert.Equal(t, 10, setFdVal) // backing fd passed to LOOP_SET_FD
	require.NotNil(t, capturedInfo)
	assert.Equal(t, uint64(512), capturedInfo.Offset)
	assert.Equal(t, uint32(flagReadOnly), capturedInfo.Flags)
}

func TestAttach_BackingFileOpenError(t *testing.T) {
	t.Parallel()

	mock := &mockSyscaller{
		openCalls: []openCall{
			{path: "/nonexistent", fd: -1, err: errors.New("no such file")},
		},
	}

	a := newAttacherWith(mock)
	dev, err := a.Attach("/nonexistent", 0, true)
	require.Error(t, err)
	assert.Nil(t, dev)
	assert.Contains(t, err.Error(), "opening backing file")
}

func TestAttach_LoopControlOpenError(t *testing.T) {
	t.Parallel()

	mock := &mockSyscaller{
		openCalls: []openCall{
			{path: "/test/file", fd: 10, err: nil},
			{path: loopControlPath, fd: -1, err: errors.New("permission denied")},
		},
	}

	a := newAttacherWith(mock)
	dev, err := a.Attach("/test/file", 0, false)
	require.Error(t, err)
	assert.Nil(t, dev)
	assert.Contains(t, err.Error(), "opening /dev/loop-control")
}

func TestAttach_LoopCtlGetFreeError(t *testing.T) {
	t.Parallel()

	mock := &mockSyscaller{
		openCalls: []openCall{
			{path: "/test/file", fd: 10, err: nil},
			{path: loopControlPath, fd: 11, err: nil},
		},
		ioctlRetIntFn: func(_ int, req uint) (int, error) {
			if req == loopCtlGetFree {
				return 0, errors.New("no free loop device")
			}
			return 0, nil
		},
	}

	a := newAttacherWith(mock)
	dev, err := a.Attach("/test/file", 0, false)
	require.Error(t, err)
	assert.Nil(t, dev)
	assert.Contains(t, err.Error(), "LOOP_CTL_GET_FREE")
}

func TestDetach_HappyPath(t *testing.T) {
	t.Parallel()

	mock := &mockSyscaller{
		ioctlSetIntFn: func(_ int, req uint, _ int) error {
			if req == loopClrFd {
				return nil
			}
			return nil
		},
	}

	d := &Device{path: "/dev/loop0", fd: 5, sc: mock}
	err := d.Detach()
	require.NoError(t, err)
	assert.True(t, d.closed.Load())
	assert.Contains(t, mock.closeCalls, 5)
}

func TestDetach_Error(t *testing.T) {
	t.Parallel()

	mock := &mockSyscaller{
		ioctlSetIntFn: func(_ int, req uint, _ int) error {
			if req == loopClrFd {
				return errors.New("device busy")
			}
			return nil
		},
	}

	d := &Device{path: "/dev/loop1", fd: 6, sc: mock}
	err := d.Detach()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "LOOP_CLR_FD")
	assert.False(t, d.closed.Load())
}

func TestDetach_AlreadyClosed(t *testing.T) {
	t.Parallel()

	d := &Device{path: "/dev/loop0", fd: 5, sc: &mockSyscaller{}}
	d.closed.Store(true)
	err := d.Detach()
	require.NoError(t, err)
}

func TestSetAutoclear_HappyPath(t *testing.T) {
	t.Parallel()

	var setFlags uint32
	mock := &mockSyscaller{
		ioctlGetStatus64Fn: func(_ int, info *loopInfo64) error {
			info.Flags = flagReadOnly // simulate existing flags
			return nil
		},
		ioctlSetStatus64Fn: func(_ int, info *loopInfo64) error {
			setFlags = info.Flags
			return nil
		},
	}

	d := &Device{path: "/dev/loop2", fd: 7, sc: mock}
	err := d.SetAutoclear()
	require.NoError(t, err)
	assert.Equal(t, uint32(flagReadOnly|flagAutoclear), setFlags)
}

func TestSetAutoclear_GetStatusError(t *testing.T) {
	t.Parallel()

	mock := &mockSyscaller{
		ioctlGetStatus64Fn: func(_ int, _ *loopInfo64) error {
			return errors.New("get failed")
		},
	}

	d := &Device{path: "/dev/loop3", fd: 8, sc: mock}
	err := d.SetAutoclear()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "LOOP_GET_STATUS64")
}

func TestAttach_ReadWrite(t *testing.T) {
	t.Parallel()

	var capturedInfo *loopInfo64
	mock := &mockSyscaller{
		openCalls: []openCall{
			{path: "/test/rw.img", fd: 10, err: nil},
			{path: loopControlPath, fd: 11, err: nil},
			{path: loopDevicePrefix + "0", fd: 12, err: nil},
		},
		ioctlRetIntFn: func(_ int, req uint) (int, error) {
			if req == loopCtlGetFree {
				return 0, nil
			}
			return 0, nil
		},
		ioctlSetStatus64Fn: func(_ int, info *loopInfo64) error {
			capturedInfo = info
			return nil
		},
	}

	a := newAttacherWith(mock)
	dev, err := a.Attach("/test/rw.img", 0, false)
	require.NoError(t, err)
	require.NotNil(t, dev)

	assert.Equal(t, "/dev/loop0", dev.Path())
	require.NotNil(t, capturedInfo)
	assert.Equal(t, uint32(0), capturedInfo.Flags) // no read-only flag
}

func TestSetAutoclear_AfterDetach(t *testing.T) {
	t.Parallel()

	mock := &mockSyscaller{
		ioctlSetIntFn: func(_ int, _ uint, _ int) error {
			return nil
		},
	}

	d := &Device{path: "/dev/loop0", fd: 5, sc: mock}

	// Detach first.
	err := d.Detach()
	require.NoError(t, err)
	assert.True(t, d.closed.Load())

	// SetAutoclear after Detach should return nil (no error, no panic).
	err = d.SetAutoclear()
	require.NoError(t, err)
}

func TestAttach_SetStatus64Failure_CleansUp(t *testing.T) {
	t.Parallel()

	var clrFdCalled bool
	mock := &mockSyscaller{
		openCalls: []openCall{
			{path: "/test/backing.img", fd: 10, err: nil},
			{path: loopControlPath, fd: 11, err: nil},
			{path: loopDevicePrefix + "0", fd: 12, err: nil},
		},
		ioctlRetIntFn: func(_ int, req uint) (int, error) {
			if req == loopCtlGetFree {
				return 0, nil
			}
			return 0, nil
		},
		ioctlSetIntFn: func(fd int, req uint, _ int) error {
			if req == loopClrFd {
				clrFdCalled = true
				assert.Equal(t, 12, fd)
			}
			return nil
		},
		ioctlSetStatus64Fn: func(_ int, _ *loopInfo64) error {
			return errors.New("status64 failed")
		},
	}

	a := newAttacherWith(mock)
	dev, err := a.Attach("/test/backing.img", 0, true)
	require.Error(t, err)
	assert.Nil(t, dev)
	assert.Contains(t, err.Error(), "LOOP_SET_STATUS64")
	assert.True(t, clrFdCalled, "LOOP_CLR_FD should be called during cleanup")
	assert.Contains(t, mock.closeCalls, 12, "loop device fd should be closed during cleanup")
}
