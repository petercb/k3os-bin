package cmdline

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFromString_FlagPresent(t *testing.T) {
	t.Parallel()
	p := NewFromString("root=/dev/sda1 quiet loglevel=3")

	val, ok := p.Flag("root")
	require.True(t, ok)
	assert.Equal(t, "/dev/sda1", val)
}

func TestNewFromString_FlagMissing(t *testing.T) {
	t.Parallel()
	p := NewFromString("root=/dev/sda1 quiet")

	val, ok := p.Flag("nosuchflag")
	assert.False(t, ok)
	assert.Empty(t, val)
}

func TestNewFromString_FlagDashUnderscoreEquivalence(t *testing.T) {
	t.Parallel()
	p := NewFromString("k3os-debug=true")

	val, ok := p.Flag("k3os_debug")
	require.True(t, ok)
	assert.Equal(t, "true", val)

	val2, ok2 := p.Flag("k3os-debug")
	require.True(t, ok2)
	assert.Equal(t, "true", val2)
}

func TestNewFromString_ContainsPresent(t *testing.T) {
	t.Parallel()
	p := NewFromString("root=/dev/sda1 quiet loglevel=3")

	assert.True(t, p.Contains("quiet"))
}

func TestNewFromString_ContainsMissing(t *testing.T) {
	t.Parallel()
	p := NewFromString("root=/dev/sda1 quiet")

	assert.False(t, p.Contains("verbose"))
}

func TestNewFromString_ContainsBooleanFlag(t *testing.T) {
	t.Parallel()
	p := NewFromString("ro quiet debug")

	assert.True(t, p.Contains("ro"))
	assert.True(t, p.Contains("quiet"))
	assert.True(t, p.Contains("debug"))
}

func TestNewFromString_Consoles(t *testing.T) {
	t.Parallel()
	p := NewFromString("console=ttyS0,115200n8 root=/dev/sda1 console=tty0")

	consoles := p.Consoles()
	assert.Equal(t, []string{"ttyS0", "tty0"}, consoles)
}

func TestNewFromString_ConsolesEmpty(t *testing.T) {
	t.Parallel()
	p := NewFromString("root=/dev/sda1 quiet")

	consoles := p.Consoles()
	assert.Empty(t, consoles)
}

func TestNewFromString_Raw(t *testing.T) {
	t.Parallel()
	raw := "root=/dev/sda1 quiet loglevel=3"
	p := NewFromString(raw)

	assert.Equal(t, raw, p.Raw())
}

func TestNewFromString_EmptyString(t *testing.T) {
	t.Parallel()
	p := NewFromString("")

	assert.Empty(t, p.Raw())
	assert.Empty(t, p.Consoles())
	assert.False(t, p.Contains("anything"))

	val, ok := p.Flag("anything")
	assert.False(t, ok)
	assert.Empty(t, val)
}

func TestNew_ReturnsNonNil(t *testing.T) {
	t.Parallel()
	p := New()
	require.NotNil(t, p)
}

func TestNew_LazyInit_DoesNotReadProcAtConstruction(t *testing.T) {
	t.Parallel()
	// New() should return a valid parser without accessing /proc/cmdline.
	// The internal cl field should be nil until the first method call.
	p := New()
	require.NotNil(t, p)

	// Cast to concrete type to inspect the internal state.
	pp, ok := p.(*parser)
	require.True(t, ok)
	assert.Nil(t, pp.cl, "cl should be nil before first method call (lazy init)")
}

func TestNew_LazyInit_LoadsOnFirstMethodCall(t *testing.T) {
	t.Parallel()
	// Calling any method should trigger the load.
	// On the test host /proc/cmdline may or may not exist, but the parser
	// should not panic regardless.
	p := New()

	// Calling Raw() triggers initialization.
	_ = p.Raw()

	pp, ok := p.(*parser)
	require.True(t, ok)
	assert.NotNil(t, pp.cl, "cl should be set after first method call")
}

func TestNewFromString_EagerInit(t *testing.T) {
	t.Parallel()
	// NewFromString should eagerly set the cl field.
	p := NewFromString("key=value")

	pp, ok := p.(*parser)
	require.True(t, ok)
	assert.NotNil(t, pp.cl, "cl should be set immediately for NewFromString")
}
