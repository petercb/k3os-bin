//go:build linux

package boot

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
		cmdline        string
		cmdlineErr     error
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
			cmdline:        "root=/dev/sda1 quiet",
			detectMode:     "disk",
			expectExecPath: "/sbin/init",
		},
		{
			name:         "bootstrap failure triggers rescue",
			cmdline:      "root=/dev/sda1",
			bootstrapErr: errors.New("bootstrap failed"),
			expectRescue: true,
		},
		{
			name:         "mode detection failure triggers rescue",
			cmdline:      "root=/dev/sda1",
			detectErr:    errors.New("detection failed"),
			expectRescue: true,
		},
		{
			name:         "mode registry failure triggers rescue",
			cmdline:      "root=/dev/sda1",
			detectMode:   "unknown",
			registryErr:  errors.New("unknown mode"),
			expectRescue: true,
		},
		{
			name:         "mode handler execute failure triggers rescue",
			cmdline:      "root=/dev/sda1",
			detectMode:   "disk",
			handlerErr:   errors.New("handler failed"),
			expectRescue: true,
		},
		{
			name:         "finalizer failure triggers rescue",
			cmdline:      "root=/dev/sda1",
			detectMode:   "disk",
			finalizerErr: errors.New("finalizer failed"),
			expectRescue: true,
		},
		{
			name:           "debug mode enabled from cmdline",
			cmdline:        "root=/dev/sda1 k3os.debug quiet",
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

			init := &Init{
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
				CmdlineReader: func() (string, error) {
					if tc.cmdlineErr != nil {
						return "", tc.cmdlineErr
					}
					return tc.cmdline, nil
				},
				RescueFunc: func() error {
					rescueCalled = true
					return nil
				},
			}

			init.Run()

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

	init := &Init{
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
		CmdlineReader: func() (string, error) {
			return "root=/dev/sda1", nil
		},
		RescueFunc: func() error {
			order = append(order, "rescue")
			return nil
		},
	}

	init.Run()

	expected := []string{"bootstrap", "detect", "registry", "handler", "finalizer", "exec"}
	assert.Equal(t, expected, order)
}

func TestInit_Run_CmdlineReaderError(t *testing.T) {
	t.Parallel()

	// If cmdline can't be read, we still proceed (debug check is best-effort).
	bootstrap := &fakeBootstrapper{}
	finalizer := &fakeFinalizer{}
	handler := &fakeModeHandler{}

	var execCalled bool

	init := &Init{
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
		CmdlineReader: func() (string, error) {
			return "", errors.New("cannot read cmdline")
		},
		RescueFunc: func() error {
			return nil
		},
	}

	init.Run()

	assert.True(t, bootstrap.called)
	assert.True(t, handler.called)
	assert.True(t, finalizer.called)
	assert.True(t, execCalled)
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
