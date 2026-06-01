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

func TestNew_IsProcParser(t *testing.T) {
	t.Parallel()
	p := New()
	_, ok := p.(*procParser)
	assert.True(t, ok, "New() should return a *procParser")
}

func TestNew_ReadsFreshOnEachCall(t *testing.T) {
	t.Parallel()
	// Calling methods multiple times should not panic, and each call
	// reads /proc/cmdline fresh (on the test host it may or may not exist).
	p := New()

	// Multiple calls should all succeed without panic.
	_ = p.Raw()
	_ = p.Raw()
	_ = p.Contains("anything")
	_, _ = p.Flag("anything")
	_ = p.Consoles()
}

func TestNewFromString_IsStaticParser(t *testing.T) {
	t.Parallel()
	p := NewFromString("key=value")
	_, ok := p.(*staticParser)
	assert.True(t, ok, "NewFromString() should return a *staticParser")
}
