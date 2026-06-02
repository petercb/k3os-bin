package mount

import (
	"errors"
	"sync"
)

// Point records a single completed mount for later unmounting.
type Point struct {
	Source string
	Target string
	FSType string
	Flags  uintptr
	Data   string
}

// UnmountFunc is the signature for the function used to unmount targets.
type UnmountFunc func(target string, flags int) error

// Pool tracks mounted filesystems and provides ordered teardown.
type Pool struct {
	mu      sync.Mutex
	mounts  []*Point
	unmount UnmountFunc
}

// NewPool creates a Pool that will use the given function for unmounting.
func NewPool(unmount UnmountFunc) *Pool {
	return &Pool{unmount: unmount}
}

// Add records a mount point in the pool.
func (p *Pool) Add(mp *Point) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.mounts = append(p.mounts, mp)
}

// UnmountAll unmounts all tracked mounts in reverse order, collecting errors.
// After completion the pool is cleared, making the call idempotent.
func (p *Pool) UnmountAll(flags int) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	var errs []error

	for i := len(p.mounts) - 1; i >= 0; i-- {
		if err := p.unmount(p.mounts[i].Target, flags); err != nil {
			errs = append(errs, err)
		}
	}

	p.mounts = nil

	return errors.Join(errs...)
}

// Len returns the number of tracked mounts.
func (p *Pool) Len() int {
	p.mu.Lock()
	defer p.mu.Unlock()

	return len(p.mounts)
}

// Entries returns a copy of all tracked mount points.
func (p *Pool) Entries() []*Point {
	p.mu.Lock()
	defer p.mu.Unlock()

	cp := make([]*Point, len(p.mounts))
	copy(cp, p.mounts)

	return cp
}
