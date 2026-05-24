package cc

import (
	"github.com/petercb/k3os-bin/internal/iface"
	"github.com/stretchr/testify/mock"
)

// Compile-time interface check.
var _ iface.ModuleLoader = (*MockModuleLoader)(nil)

// MockModuleLoader is a testable iface.ModuleLoader implementation backed by
// testify/mock. All methods delegate to m.Called(...) and return appropriately
// typed values. LoadedModules guards against nil before type-asserting to
// avoid panics when the mock is configured to return an error with a nil value.
type MockModuleLoader struct {
	mock.Mock
}

func (m *MockModuleLoader) LoadedModules() (map[string]bool, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]bool), args.Error(1)
}

func (m *MockModuleLoader) LoadModule(name string, params string) error {
	return m.Called(name, params).Error(0)
}
