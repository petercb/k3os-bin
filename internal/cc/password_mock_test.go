package cc

import (
	"github.com/petercb/k3os-bin/internal/iface"
	"github.com/petercb/k3os-bin/internal/shadow"
	"github.com/stretchr/testify/mock"
)

// Compile-time interface check.
var _ shadow.PasswordSetter = (*MockPasswordSetter)(nil)

// MockPasswordSetter is a testable shadow.PasswordSetter implementation backed
// by testify/mock.
type MockPasswordSetter struct {
	mock.Mock
}

func (m *MockPasswordSetter) SetPassword(fs iface.FileSystem, username string, password string) error {
	return m.Called(fs, username, password).Error(0)
}
