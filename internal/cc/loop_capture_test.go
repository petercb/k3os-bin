package cc

import (
	"fmt"
	"testing"

	"github.com/petercb/k3os-bin/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRunApplies_ClosureIsolation verifies that runApplies invokes each
// applier independently without closure variable sharing bugs.
//
// Under Go 1.22+ loop variable semantics, each iteration of a for-range
// loop creates a new variable. This test documents the expected behavior:
// every applier receives its own invocation and operates on independent
// state, regardless of whether the loop variable is captured by reference
// or by value.
func TestRunApplies_ClosureIsolation(t *testing.T) {
	t.Parallel()

	a := newTestApplier(nil, nil, nil, nil, nil)

	const count = 5
	invoked := make([]int, 0, count)

	// Build appliers using a loop -- the classic closure-capture scenario.
	// Under Go <1.22 without explicit re-binding, all closures would
	// capture the same loop variable and record the final index. Under
	// Go 1.22+ each iteration has its own variable. The runApplies
	// implementation iterates the slice by value, so this is safe
	// regardless of Go version, but documenting the expectation is
	// valuable as a regression guard.
	appliers := make([]applier, count)
	for i := 0; i < count; i++ {
		idx := i // explicit re-bind (safe under all Go versions)
		appliers[idx] = func(_ *config.CloudConfig) error {
			invoked = append(invoked, idx)
			return nil
		}
	}

	err := a.runApplies(&config.CloudConfig{}, appliers...)
	require.NoError(t, err)

	// Each applier must have been called exactly once, in order.
	expected := []int{0, 1, 2, 3, 4}
	assert.Equal(t, expected, invoked, "each applier should be called independently in order")
}

// TestRunApplies_ClosureIsolation_WithErrors verifies that even when some
// appliers return errors, every applier is still invoked with independent
// state (no shared closure variable bugs).
func TestRunApplies_ClosureIsolation_WithErrors(t *testing.T) {
	t.Parallel()

	a := newTestApplier(nil, nil, nil, nil, nil)

	const count = 4
	invoked := make([]int, 0, count)

	appliers := make([]applier, count)
	for i := 0; i < count; i++ {
		idx := i
		appliers[idx] = func(_ *config.CloudConfig) error {
			invoked = append(invoked, idx)
			if idx%2 == 1 { // odd indices return errors
				return fmt.Errorf("applier %d failed", idx)
			}
			return nil
		}
	}

	err := a.runApplies(&config.CloudConfig{}, appliers...)
	require.Error(t, err)

	// All appliers must have been called, even those after an error.
	expected := []int{0, 1, 2, 3}
	assert.Equal(t, expected, invoked, "all appliers should be invoked regardless of errors")
}
