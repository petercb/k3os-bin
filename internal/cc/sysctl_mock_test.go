package cc

import (
	"github.com/petercb/k3os-bin/internal/iface"
	"github.com/stretchr/testify/mock"
)

// Compile-time interface check.
var _ iface.SysctlApplier = (*MockSysctlApplier)(nil)

// MockSysctlApplier is a testable iface.SysctlApplier implementation backed by
// testify/mock. All methods delegate to m.Called(...) and return appropriately
// typed values.
type MockSysctlApplier struct {
	mock.Mock
}

func (m *MockSysctlApplier) Set(key string, value string) error {
	return m.Called(key, value).Error(0)
}
