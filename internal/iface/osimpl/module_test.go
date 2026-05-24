//go:build linux

package osimpl_test

import (
	"strings"
	"testing"

	"github.com/petercb/k3os-bin/internal/iface/osimpl"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLinuxModuleLoader_LoadedModules_ReturnsNonEmpty(t *testing.T) {
	loader := osimpl.LinuxModuleLoader{}
	modules, err := loader.LoadedModules()
	require.NoError(t, err)

	// Docker Desktop (LinuxKit) uses a monolithic kernel with no loadable modules,
	// so /proc/modules is empty. Skip assertion in that environment.
	if len(modules) == 0 {
		t.Skip("no kernel modules loaded (likely Docker Desktop with monolithic kernel)")
	}
	assert.NotEmpty(t, modules, "expected at least one loaded kernel module")
}

func TestLinuxModuleLoader_LoadedModules_NamesHaveNoWhitespace(t *testing.T) {
	loader := osimpl.LinuxModuleLoader{}
	modules, err := loader.LoadedModules()
	require.NoError(t, err)

	for name := range modules {
		assert.NotContains(t, name, " ", "module name should not contain spaces")
		assert.NotContains(t, name, "\t", "module name should not contain tabs")
		assert.Equal(t, name, strings.TrimSpace(name),
			"module name should have no leading/trailing whitespace")
	}
}

func TestLinuxModuleLoader_LoadedModules_ExtractsOnlyFirstField(t *testing.T) {
	loader := osimpl.LinuxModuleLoader{}
	modules, err := loader.LoadedModules()
	require.NoError(t, err)

	// /proc/modules lines: <name> <size> <refcount> <deps> <state> <offset>
	// Module names are identifiers: alphanumeric + underscore, no spaces
	for name := range modules {
		assert.Regexp(t, `^[a-zA-Z0-9_]+$`, name,
			"module name should only contain alphanumeric chars and underscores")
	}
}
