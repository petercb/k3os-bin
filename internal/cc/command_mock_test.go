package cc

import (
	"github.com/petercb/k3os-bin/internal/iface"
	"github.com/stretchr/testify/mock"
)

// Compile-time interface check.
var _ iface.CommandRunner = (*MockCommandRunner)(nil)

// MockCommandRunner is a testable iface.CommandRunner implementation backed by
// testify/mock. All methods delegate to m.Called(...) and return appropriately
// typed values.
type MockCommandRunner struct {
	mock.Mock
}

func (m *MockCommandRunner) Run(name string, args ...string) error {
	callArgs := make([]interface{}, 1+len(args))
	callArgs[0] = name
	for i, a := range args {
		callArgs[i+1] = a
	}
	return m.Called(callArgs...).Error(0)
}

func (m *MockCommandRunner) RunWithStdin(stdin string, name string, args ...string) error {
	callArgs := make([]interface{}, 2+len(args))
	callArgs[0] = stdin
	callArgs[1] = name
	for i, a := range args {
		callArgs[i+2] = a
	}
	return m.Called(callArgs...).Error(0)
}

func (m *MockCommandRunner) RunShell(command string) error {
	return m.Called(command).Error(0)
}

func (m *MockCommandRunner) RunWithEnv(env []string, name string, args ...string) error {
	callArgs := make([]interface{}, 2+len(args))
	callArgs[0] = env
	callArgs[1] = name
	for i, a := range args {
		callArgs[i+2] = a
	}
	return m.Called(callArgs...).Error(0)
}

func (m *MockCommandRunner) RunOutput(name string, args ...string) (string, error) {
	callArgs := make([]interface{}, 1+len(args))
	callArgs[0] = name
	for i, a := range args {
		callArgs[i+1] = a
	}
	ret := m.Called(callArgs...)
	return ret.String(0), ret.Error(1)
}
