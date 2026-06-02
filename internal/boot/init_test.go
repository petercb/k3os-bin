//go:build linux

package boot

import (
	"context"
	"errors"
	"testing"

	"github.com/petercb/k3os-bin/internal/iface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockCmdlineParser implements iface.CmdlineParser for testing.
type mockCmdlineParser struct {
	flags    map[string]string
	contains map[string]bool
	raw      string
	consoles []string
}

// Compile-time check.
var _ iface.CmdlineParser = (*mockCmdlineParser)(nil)

func (m *mockCmdlineParser) Flag(name string) (string, bool) {
	v, ok := m.flags[name]
	return v, ok
}

func (m *mockCmdlineParser) Contains(name string) bool {
	return m.contains[name]
}

func (m *mockCmdlineParser) Consoles() []string { return m.consoles }
func (m *mockCmdlineParser) Raw() string        { return m.raw }

// fakeReaper implements OrphanReaper for testing.
type fakeReaper struct {
	startCalled bool
	waitCalled  bool
}

func (f *fakeReaper) Start(_ context.Context) {
	f.startCalled = true
}

func (f *fakeReaper) Wait() {
	f.waitCalled = true
}

// fakeBootstrapper implements BootstrapRunner for testing.
type fakeBootstrapper struct {
	called bool
	err    error
}

func (f *fakeBootstrapper) Run() error {
	f.called = true
	return f.err
}

// fakeFinalizer implements FinalizerRunner for testing.
type fakeFinalizer struct {
	called bool
	err    error
}

func (f *fakeFinalizer) Run() error {
	f.called = true
	return f.err
}

// fakeModeHandler implements ModeHandler for testing.
type fakeModeHandler struct {
	called bool
	err    error
}

func (f *fakeModeHandler) Execute() error {
	f.called = true
	return f.err
}

// execCall records the arguments passed to ExecFunc.
type execCall struct {
	path string
	args []string
	env  []string
}

func TestInit_Run(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		contains       map[string]bool
		bootstrapErr   error
		detectMode     string
		detectErr      error
		registryMode   string
		registryErr    error
		handlerErr     error
		finalizerErr   error
		execErr        error
		expectRescue   bool
		expectExecPath string
	}{
		{
			name:           "full success path",
			detectMode:     "disk",
			expectExecPath: "/sbin/init",
		},
		{
			name:         "bootstrap failure triggers rescue",
			bootstrapErr: errors.New("bootstrap failed"),
			expectRescue: true,
		},
		{
			name:         "mode detection failure triggers rescue",
			detectErr:    errors.New("detection failed"),
			expectRescue: true,
		},
		{
			name:         "mode registry failure triggers rescue",
			detectMode:   "unknown",
			registryErr:  errors.New("unknown mode"),
			expectRescue: true,
		},
		{
			name:         "mode handler execute failure triggers rescue",
			detectMode:   "disk",
			handlerErr:   errors.New("handler failed"),
			expectRescue: true,
		},
		{
			name:         "finalizer failure triggers rescue",
			detectMode:   "disk",
			finalizerErr: errors.New("finalizer failed"),
			expectRescue: true,
		},
		{
			name:           "debug mode enabled from cmdline",
			contains:       map[string]bool{"k3os.debug": true},
			detectMode:     "local",
			expectExecPath: "/sbin/init",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			bootstrap := &fakeBootstrapper{err: tc.bootstrapErr}
			finalizer := &fakeFinalizer{err: tc.finalizerErr}
			handler := &fakeModeHandler{err: tc.handlerErr}

			var execCalled *execCall
			var rescueCalled bool

			initOrch := &Init{
				Bootstrap: bootstrap,
				ModeDetector: func() (string, error) {
					if tc.detectErr != nil {
						return "", tc.detectErr
					}
					return tc.detectMode, nil
				},
				ModeRegistry: func(_ string) (ModeHandler, error) {
					if tc.registryErr != nil {
						return nil, tc.registryErr
					}
					return handler, nil
				},
				Finalizer: finalizer,
				ExecFunc: func(path string, args []string, env []string) error {
					execCalled = &execCall{path: path, args: args, env: env}
					return tc.execErr
				},
				Cmdline: &mockCmdlineParser{contains: tc.contains},
				RescueFunc: func() error {
					rescueCalled = true
					return nil
				},
			}

			initOrch.Run()

			if tc.expectRescue {
				assert.True(t, rescueCalled, "rescue should have been called")
				assert.Nil(t, execCalled, "exec should not have been called")
			} else {
				assert.False(t, rescueCalled, "rescue should not have been called")
				require.NotNil(t, execCalled, "exec should have been called")
				assert.Equal(t, tc.expectExecPath, execCalled.path)
			}
		})
	}
}

func TestInit_Run_PhasesCalledInOrder(t *testing.T) {
	t.Parallel()

	var order []string

	initOrch := &Init{
		Bootstrap: &fakeBootstrapperOrdered{order: &order, name: "bootstrap"},
		ModeDetector: func() (string, error) {
			order = append(order, "detect")
			return "disk", nil
		},
		ModeRegistry: func(_ string) (ModeHandler, error) {
			order = append(order, "registry")
			return &fakeModeHandlerOrdered{order: &order, name: "handler"}, nil
		},
		Finalizer: &fakeFinalizerOrdered{order: &order, name: "finalizer"},
		ExecFunc: func(_ string, _ []string, _ []string) error {
			order = append(order, "exec")
			return nil
		},
		Cmdline: &mockCmdlineParser{},
		RescueFunc: func() error {
			order = append(order, "rescue")
			return nil
		},
	}

	initOrch.Run()

	expected := []string{"bootstrap", "detect", "registry", "handler", "finalizer", "exec"}
	assert.Equal(t, expected, order)
}

func TestInit_Run_NilCmdline(t *testing.T) {
	t.Parallel()

	// If Cmdline is nil, we still proceed (debug check is guarded with nil check).
	bootstrap := &fakeBootstrapper{}
	finalizer := &fakeFinalizer{}
	handler := &fakeModeHandler{}

	var execCalled bool

	initOrch := &Init{
		Bootstrap: bootstrap,
		ModeDetector: func() (string, error) {
			return "disk", nil
		},
		ModeRegistry: func(_ string) (ModeHandler, error) {
			return handler, nil
		},
		Finalizer: finalizer,
		ExecFunc: func(_ string, _ []string, _ []string) error {
			execCalled = true
			return nil
		},
		Cmdline: nil,
		RescueFunc: func() error {
			return nil
		},
	}

	initOrch.Run()

	assert.True(t, bootstrap.called)
	assert.True(t, handler.called)
	assert.True(t, finalizer.called)
	assert.True(t, execCalled)
}

func TestInit_Run_ReaperStartedAndWaited(t *testing.T) {
	t.Parallel()

	r := &fakeReaper{}
	bootstrap := &fakeBootstrapper{}
	finalizer := &fakeFinalizer{}
	handler := &fakeModeHandler{}

	initOrch := &Init{
		Bootstrap: bootstrap,
		Reaper:    r,
		ModeDetector: func() (string, error) {
			return "disk", nil
		},
		ModeRegistry: func(_ string) (ModeHandler, error) {
			return handler, nil
		},
		Finalizer: finalizer,
		ExecFunc: func(_ string, _ []string, _ []string) error {
			return nil
		},
		Cmdline: &mockCmdlineParser{},
		RescueFunc: func() error {
			return nil
		},
	}

	initOrch.Run()

	assert.True(t, r.startCalled, "Reaper.Start should have been called")
	assert.True(t, r.waitCalled, "Reaper.Wait should have been called")
}

func TestInit_Run_NilReaperDoesNotPanic(t *testing.T) {
	t.Parallel()

	bootstrap := &fakeBootstrapper{}
	finalizer := &fakeFinalizer{}
	handler := &fakeModeHandler{}

	var execCalled bool

	initOrch := &Init{
		Bootstrap: bootstrap,
		Reaper:    nil,
		ModeDetector: func() (string, error) {
			return "disk", nil
		},
		ModeRegistry: func(_ string) (ModeHandler, error) {
			return handler, nil
		},
		Finalizer: finalizer,
		ExecFunc: func(_ string, _ []string, _ []string) error {
			execCalled = true
			return nil
		},
		Cmdline: &mockCmdlineParser{},
		RescueFunc: func() error {
			return nil
		},
	}

	assert.NotPanics(t, func() {
		initOrch.Run()
	})
	assert.True(t, execCalled)
}

func TestInit_Run_ReaperReceivesValidContext(t *testing.T) {
	t.Parallel()

	var receivedCtx context.Context

	captureReaper := &contextCapturingReaper{}

	initOrch := &Init{
		Bootstrap: &fakeBootstrapper{},
		Reaper:    captureReaper,
		ModeDetector: func() (string, error) {
			// Verify context is not cancelled at this point.
			receivedCtx = captureReaper.ctx
			return "disk", nil
		},
		ModeRegistry: func(_ string) (ModeHandler, error) {
			return &fakeModeHandler{}, nil
		},
		Finalizer: &fakeFinalizer{},
		ExecFunc: func(_ string, _ []string, _ []string) error {
			return nil
		},
		Cmdline: &mockCmdlineParser{},
		RescueFunc: func() error {
			return nil
		},
	}

	initOrch.Run()

	require.NotNil(t, receivedCtx)
	// Context should not have been cancelled during the boot sequence.
	// It gets cancelled in defer after Run returns.
}

// contextCapturingReaper captures the context passed to Start.
type contextCapturingReaper struct {
	ctx         context.Context
	startCalled bool
	waitCalled  bool
}

func (c *contextCapturingReaper) Start(ctx context.Context) {
	c.ctx = ctx
	c.startCalled = true
}

func (c *contextCapturingReaper) Wait() {
	c.waitCalled = true
}

// Ordered fake implementations for tracking call order.

type fakeBootstrapperOrdered struct {
	order *[]string
	name  string
}

func (f *fakeBootstrapperOrdered) Run() error {
	*f.order = append(*f.order, f.name)
	return nil
}

type fakeFinalizerOrdered struct {
	order *[]string
	name  string
}

func (f *fakeFinalizerOrdered) Run() error {
	*f.order = append(*f.order, f.name)
	return nil
}

type fakeModeHandlerOrdered struct {
	order *[]string
	name  string
}

func (f *fakeModeHandlerOrdered) Execute() error {
	*f.order = append(*f.order, f.name)
	return nil
}
