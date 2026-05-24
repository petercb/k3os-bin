package cc

import (
	"github.com/petercb/k3os-bin/internal/iface"
	"github.com/stretchr/testify/mock"
)

// Compile-time interface check.
var _ iface.HostnameSetter = (*MockHostnameSetter)(nil)

// MockHostnameSetter is a testable iface.HostnameSetter implementation backed by
// testify/mock. All methods delegate to m.Called(...) and return appropriately
// typed values.
type MockHostnameSetter struct {
	mock.Mock
}

func (m *MockHostnameSetter) SetHostname(name string) error {
	return m.Called(name).Error(0)
}
