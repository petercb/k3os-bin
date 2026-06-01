//go:build linux

package loopdev

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockDeviceFinder is a test double for the deviceFinder interface.
type mockDeviceFinder struct {
	path string
	err  error
}

func (m *mockDeviceFinder) FindDevice() (string, error) {
	return m.path, m.err
}

// mockSyscaller is a test double for the syscaller interface.
type mockSyscaller struct {
	openCalls  []openCall
	openIdx    int
	closeMu    sync.Mutex
	closeCalls []int

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
	m.closeMu.Lock()
	m.closeCalls = append(m.closeCalls, fd)
	m.closeMu.Unlock()
	return nil
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
			{path: "/test/backing.img", fd: 10, err: nil}, // backing file
			{path: "/dev/loop3", fd: 12, err: nil},        // loop device
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
	df := &mockDeviceFinder{path: "/dev/loop3", err: nil}

	a := newAttacherWith(df, mock)
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
	df := &mockDeviceFinder{path: "/dev/loop0", err: nil}

	a := newAttacherWith(df, mock)
	dev, err := a.Attach("/nonexistent", 0, true)
	require.Error(t, err)
	assert.Nil(t, dev)
	assert.Contains(t, err.Error(), "opening backing file")
}

func TestAttach_DeviceFinderError(t *testing.T) {
	t.Parallel()

	mock := &mockSyscaller{
		openCalls: []openCall{
			{path: "/test/file", fd: 10, err: nil}, // backing file opens ok
		},
	}
	df := &mockDeviceFinder{path: "", err: errors.New("no free loop device")}

	a := newAttacherWith(df, mock)
	dev, err := a.Attach("/test/file", 0, false)
	require.Error(t, err)
	assert.Nil(t, dev)
	assert.Contains(t, err.Error(), "finding free loop device")
}

func TestAttach_LoopDeviceOpenError(t *testing.T) {
	t.Parallel()

	mock := &mockSyscaller{
		openCalls: []openCall{
			{path: "/test/file", fd: 10, err: nil},
			{path: "/dev/loop0", fd: -1, err: errors.New("permission denied")},
		},
	}
	df := &mockDeviceFinder{path: "/dev/loop0", err: nil}

	a := newAttacherWith(df, mock)
	dev, err := a.Attach("/test/file", 0, false)
	require.Error(t, err)
	assert.Nil(t, dev)
	assert.Contains(t, err.Error(), "opening loop device")
}

func TestAttach_SetFdError(t *testing.T) {
	t.Parallel()

	mock := &mockSyscaller{
		openCalls: []openCall{
			{path: "/test/file", fd: 10, err: nil},
			{path: "/dev/loop0", fd: 12, err: nil},
		},
		ioctlSetIntFn: func(_ int, req uint, _ int) error {
			if req == loopSetFd {
				return errors.New("set fd failed")
			}
			return nil
		},
	}
	df := &mockDeviceFinder{path: "/dev/loop0", err: nil}

	a := newAttacherWith(df, mock)
	dev, err := a.Attach("/test/file", 0, true)
	require.Error(t, err)
	assert.Nil(t, dev)
	assert.Contains(t, err.Error(), "LOOP_SET_FD")
	assert.Contains(t, mock.closeCalls, 12, "loop device fd should be closed on SET_FD failure")
}

func TestAttach_SetStatus64Failure_CleansUp(t *testing.T) {
	t.Parallel()

	var clrFdCalled bool
	mock := &mockSyscaller{
		openCalls: []openCall{
			{path: "/test/backing.img", fd: 10, err: nil},
			{path: "/dev/loop0", fd: 12, err: nil},
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
	df := &mockDeviceFinder{path: "/dev/loop0", err: nil}

	a := newAttacherWith(df, mock)
	dev, err := a.Attach("/test/backing.img", 0, true)
	require.Error(t, err)
	assert.Nil(t, dev)
	assert.Contains(t, err.Error(), "LOOP_SET_STATUS64")
	assert.True(t, clrFdCalled, "LOOP_CLR_FD should be called during cleanup")
	assert.Contains(t, mock.closeCalls, 12, "loop device fd should be closed during cleanup")
}

func TestAttach_ReadWrite(t *testing.T) {
	t.Parallel()

	var capturedInfo *loopInfo64
	mock := &mockSyscaller{
		openCalls: []openCall{
			{path: "/test/rw.img", fd: 10, err: nil},
			{path: "/dev/loop0", fd: 12, err: nil},
		},
		ioctlSetStatus64Fn: func(_ int, info *loopInfo64) error {
			capturedInfo = info
			return nil
		},
	}
	df := &mockDeviceFinder{path: "/dev/loop0", err: nil}

	a := newAttacherWith(df, mock)
	dev, err := a.Attach("/test/rw.img", 0, false)
	require.NoError(t, err)
	require.NotNil(t, dev)

	assert.Equal(t, "/dev/loop0", dev.Path())
	require.NotNil(t, capturedInfo)
	assert.Equal(t, uint32(0), capturedInfo.Flags) // no read-only flag
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
	// closed is reverted to false on failure so retry is possible
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

func TestDetach_ConcurrentCalls(t *testing.T) {
	t.Parallel()

	var ioctlCalls atomic.Int32
	var closeCalls atomic.Int32

	mock := &mockSyscaller{
		ioctlSetIntFn: func(_ int, req uint, _ int) error {
			if req == loopClrFd {
				ioctlCalls.Add(1)
			}
			return nil
		},
	}
	// Override Close to count calls atomically.
	countingMock := &concurrentMockSyscaller{
		mockSyscaller: mock,
		closeCalls:    &closeCalls,
	}

	d := &Device{path: "/dev/loop0", fd: 5, sc: countingMock}

	const goroutines = 10
	var wg sync.WaitGroup
	wg.Add(goroutines)
	errs := make([]error, goroutines)

	for i := range goroutines {
		go func(idx int) {
			defer wg.Done()
			errs[idx] = d.Detach()
		}(i)
	}
	wg.Wait()

	// Exactly one goroutine should have issued LOOP_CLR_FD.
	assert.Equal(t, int32(1), ioctlCalls.Load(), "LOOP_CLR_FD should be called exactly once")
	// Exactly one goroutine should have closed the fd.
	assert.Equal(t, int32(1), closeCalls.Load(), "Close should be called exactly once")
	// All calls should succeed (no errors, no panics).
	for _, err := range errs {
		require.NoError(t, err)
	}
	assert.True(t, d.closed.Load())
}

// concurrentMockSyscaller wraps mockSyscaller with atomic close counting for concurrent tests.
type concurrentMockSyscaller struct {
	mockSyscaller *mockSyscaller
	closeCalls    *atomic.Int32
}

func (c *concurrentMockSyscaller) Open(path string, flags int, perm uint32) (int, error) {
	return c.mockSyscaller.Open(path, flags, perm)
}

func (c *concurrentMockSyscaller) Close(_ int) error {
	c.closeCalls.Add(1)
	return nil
}

func (c *concurrentMockSyscaller) IoctlSetInt(fd int, req uint, val int) error {
	return c.mockSyscaller.IoctlSetInt(fd, req, val)
}

func (c *concurrentMockSyscaller) IoctlLoopGetStatus64(fd int, info *loopInfo64) error {
	return c.mockSyscaller.IoctlLoopGetStatus64(fd, info)
}

func (c *concurrentMockSyscaller) IoctlLoopSetStatus64(fd int, info *loopInfo64) error {
	return c.mockSyscaller.IoctlLoopSetStatus64(fd, info)
}
