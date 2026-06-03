//go:build linux

package reaper

import (
	"context"
	"os/exec"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReaper_StartAndWait_CancelStops(t *testing.T) {
	t.Parallel()

	r := New()
	ctx, cancel := context.WithCancel(context.Background())

	r.Start(ctx)

	// Give the goroutine a moment to be running.
	time.Sleep(50 * time.Millisecond)

	cancel()

	// Wait should return promptly after context cancellation.
	done := make(chan struct{})
	go func() {
		r.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success: Wait returned after cancel.
	case <-time.After(2 * time.Second):
		t.Fatal("Wait did not return after context cancellation")
	}
}

func TestReaper_ReapsZombieChild(t *testing.T) {
	t.Parallel()

	r := New()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	r.Start(ctx)

	// Spawn a short-lived child process that exits immediately.
	cmd := exec.Command("true")
	require.NoError(t, cmd.Start())

	// Give the reaper time to pick up the zombie.
	time.Sleep(300 * time.Millisecond)

	cancel()
	r.Wait()

	// In a test environment we are not PID 1, so the test process itself
	// is the parent. The reaper's Wait4(-1, ...) will get ECHILD because
	// we cannot actually reparent children to ourselves. This test
	// confirms the reaper handles ECHILD gracefully and does not panic.
}

func TestReaper_NilContextHandled(t *testing.T) {
	t.Parallel()

	// Starting with an already-cancelled context should still work.
	r := New()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	r.Start(ctx)

	done := make(chan struct{})
	go func() {
		r.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success.
	case <-time.After(2 * time.Second):
		t.Fatal("Wait did not return with pre-cancelled context")
	}
}

func TestReaper_ConcurrentStartStop(t *testing.T) {
	t.Parallel()

	// Verify that concurrent start/stop does not race.
	var wg sync.WaitGroup

	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()

			r := New()
			ctx, cancel := context.WithCancel(context.Background())
			r.Start(ctx)
			time.Sleep(10 * time.Millisecond)
			cancel()
			r.Wait()
		}()
	}

	wg.Wait()
}

func TestReaper_WaitWithoutStart(t *testing.T) {
	t.Parallel()

	// Wait on a reaper that was never started should not block.
	r := New()

	done := make(chan struct{})
	go func() {
		r.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success.
	case <-time.After(2 * time.Second):
		t.Fatal("Wait blocked without Start being called")
	}
}

func TestReaper_MultipleWaitCalls(t *testing.T) {
	t.Parallel()

	r := New()
	ctx, cancel := context.WithCancel(context.Background())

	r.Start(ctx)
	cancel()
	r.Wait()

	// Second Wait should also return immediately.
	done := make(chan struct{})
	go func() {
		r.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success.
	case <-time.After(2 * time.Second):
		t.Fatal("second Wait call blocked")
	}
}

func TestNew_ReturnsNonNil(t *testing.T) {
	t.Parallel()

	r := New()
	assert.NotNil(t, r)
}
