//go:build linux

// Package reaper implements orphan process reaping for PID 1.
// When a process's parent exits, the orphaned child is reparented to PID 1.
// Without active reaping, these children become zombies when they exit.
package reaper

import (
	"context"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// pollInterval is the time between WNOHANG polls.
const pollInterval = 100 * time.Millisecond

// Reaper reaps orphaned child processes using Wait4 in a background goroutine.
type Reaper struct {
	once    sync.Once
	started atomic.Bool
	done    chan struct{}
}

// New creates a new Reaper instance.
func New() *Reaper {
	return &Reaper{
		done: make(chan struct{}),
	}
}

// Start spawns a goroutine that reaps zombie children until ctx is cancelled.
func (r *Reaper) Start(ctx context.Context) {
	r.once.Do(func() {
		r.started.Store(true)
		go r.loop(ctx)
	})
}

// Wait blocks until the reaper goroutine exits. If Start was never called,
// Wait returns immediately.
func (r *Reaper) Wait() {
	if !r.started.Load() {
		return
	}
	<-r.done
}

// loop polls for zombie children using WNOHANG and exits when ctx is done.
func (r *Reaper) loop(ctx context.Context) {
	defer close(r.done)

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			r.reapAll()
			return
		case <-ticker.C:
			r.reapAll()
		}
	}
}

// reapAll reaps all available zombie children without blocking.
func (r *Reaper) reapAll() {
	for {
		var status syscall.WaitStatus
		pid, err := syscall.Wait4(-1, &status, syscall.WNOHANG, nil)
		if pid <= 0 || err != nil {
			return
		}
	}
}
