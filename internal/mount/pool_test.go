package mount

import (
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPool_Empty(t *testing.T) {
	t.Parallel()

	p := NewPool(func(_ string, _ int) error { return nil })

	require.NotNil(t, p)
	assert.Equal(t, 0, p.Len())
}

func TestPool_Add(t *testing.T) {
	t.Parallel()

	p := NewPool(func(_ string, _ int) error { return nil })

	p.Add(&Point{Source: "proc", Target: "/proc", FSType: "proc"})
	assert.Equal(t, 1, p.Len())

	p.Add(&Point{Source: "tmpfs", Target: "/tmp", FSType: "tmpfs"})
	assert.Equal(t, 2, p.Len())
}

func TestPool_UnmountAll_ReverseOrder(t *testing.T) {
	t.Parallel()

	var order []string
	unmount := func(target string, _ int) error {
		order = append(order, target)
		return nil
	}

	p := NewPool(unmount)
	p.Add(&Point{Target: "/a"})
	p.Add(&Point{Target: "/b"})
	p.Add(&Point{Target: "/c"})

	err := p.UnmountAll(0)
	require.NoError(t, err)
	assert.Equal(t, []string{"/c", "/b", "/a"}, order)
}

func TestPool_UnmountAll_PassesFlags(t *testing.T) {
	t.Parallel()

	var gotFlags []int
	unmount := func(_ string, flags int) error {
		gotFlags = append(gotFlags, flags)
		return nil
	}

	p := NewPool(unmount)
	p.Add(&Point{Target: "/x"})
	p.Add(&Point{Target: "/y"})

	err := p.UnmountAll(42)
	require.NoError(t, err)
	assert.Equal(t, []int{42, 42}, gotFlags)
}

func TestPool_UnmountAll_CollectsErrors(t *testing.T) {
	t.Parallel()

	unmount := func(target string, _ int) error {
		if target == "/b" || target == "/c" {
			return errors.New("fail " + target)
		}
		return nil
	}

	p := NewPool(unmount)
	p.Add(&Point{Target: "/a"})
	p.Add(&Point{Target: "/b"})
	p.Add(&Point{Target: "/c"})

	err := p.UnmountAll(0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fail /c")
	assert.Contains(t, err.Error(), "fail /b")
}

func TestPool_UnmountAll_EmptyPool(t *testing.T) {
	t.Parallel()

	p := NewPool(func(_ string, _ int) error { return errors.New("should not be called") })

	err := p.UnmountAll(0)
	require.NoError(t, err)
}

func TestPool_Len(t *testing.T) {
	t.Parallel()

	p := NewPool(func(_ string, _ int) error { return nil })
	assert.Equal(t, 0, p.Len())

	p.Add(&Point{Target: "/one"})
	assert.Equal(t, 1, p.Len())

	p.Add(&Point{Target: "/two"})
	p.Add(&Point{Target: "/three"})
	assert.Equal(t, 3, p.Len())
}

func TestPool_Entries_ReturnsCopy(t *testing.T) {
	t.Parallel()

	p := NewPool(func(_ string, _ int) error { return nil })
	p.Add(&Point{Target: "/a"})
	p.Add(&Point{Target: "/b"})

	entries := p.Entries()
	assert.Len(t, entries, 2)

	// Modifying the returned slice should not affect the pool.
	entries[0] = &Point{Target: "/modified"}

	poolEntries := p.Entries()
	assert.Len(t, poolEntries, 2)
	assert.Equal(t, "/a", poolEntries[0].Target)
	assert.Equal(t, "/b", poolEntries[1].Target)
}

func TestPool_UnmountAll_ClearsPool(t *testing.T) {
	t.Parallel()

	p := NewPool(func(_ string, _ int) error { return nil })
	p.Add(&Point{Target: "/a"})
	p.Add(&Point{Target: "/b"})

	err := p.UnmountAll(0)
	require.NoError(t, err)
	assert.Equal(t, 0, p.Len(), "pool should be empty after UnmountAll")

	// Second call is a no-op and returns nil.
	err = p.UnmountAll(0)
	require.NoError(t, err)
	assert.Equal(t, 0, p.Len())
}

func TestPool_ConcurrentAdd(t *testing.T) {
	t.Parallel()

	p := NewPool(func(_ string, _ int) error { return nil })

	const goroutines = 50

	var wg sync.WaitGroup

	wg.Add(goroutines)

	for i := range goroutines {
		go func(n int) {
			defer wg.Done()
			p.Add(&Point{Target: "/mnt/" + string(rune('a'+n%26))})
		}(i)
	}

	wg.Wait()
	assert.Equal(t, goroutines, p.Len())
}
