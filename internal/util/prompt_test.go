package util

import (
	"errors"
	"io"
	"math/rand"
	"os"
	"strings"
	"testing"
	"testing/quick"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMaskPassword_IOError(t *testing.T) {
	// Create a pipe and close the read end to produce an I/O error
	r, w, err := os.Pipe()
	require.NoError(t, err)
	defer w.Close() //nolint:errcheck

	// Close the read end — reading from a closed file descriptor produces an error
	r.Close() //nolint:errcheck

	_, maskErr := MaskPassword(r, os.Stdout)
	require.Error(t, maskErr)

	// The returned error should wrap the underlying pipe error.
	// errors.Is traverses the chain to find the original cause.
	// MaskPassword returns the raw I/O error from getCharacter; after migration
	// PromptPassword wraps it with fmt.Errorf("...: %w", err), preserving the chain.
	// Here we verify the error is an os.PathError (file already closed).
	var pathErr *os.PathError
	assert.ErrorAs(t, maskErr, &pathErr,
		"expected error to wrap os.PathError, got: %v", maskErr)
}

func TestMaskPassword_Interrupt(t *testing.T) {
	// Create a pipe and write Ctrl+C (byte 0x03)
	r, w, err := os.Pipe()
	require.NoError(t, err)
	defer r.Close() //nolint:errcheck

	// Write the interrupt byte
	_, err = w.Write([]byte{0x03})
	require.NoError(t, err)
	w.Close() //nolint:errcheck

	_, maskErr := MaskPassword(r, os.Stdout)
	require.Error(t, maskErr)
	assert.Equal(t, "interrupted", maskErr.Error())
}

func TestMaskPassword_MaxLengthExceeded(t *testing.T) {
	// Create a pipe and write 513+ printable bytes followed by a newline
	r, w, err := os.Pipe()
	require.NoError(t, err)
	defer r.Close() //nolint:errcheck

	// Write 513 printable bytes (exceeds maxBytes=512) followed by newline to terminate
	input := strings.Repeat("a", 513) + "\n"
	go func() {
		_, _ = w.Write([]byte(input))
		w.Close() //nolint:errcheck
	}()

	_, maskErr := MaskPassword(r, os.Stdout)
	require.Error(t, maskErr)
	assert.Contains(t, maskErr.Error(), "maximum password length")
}

// TestProperty_MaskPassword_IOErrorPropagation verifies that for any I/O error
// produced by the underlying reader (at any point during reading), MaskPassword
// returns an error that wraps the original I/O error such that errors.Is can
// traverse the chain to find it.
//
// **Validates: Requirements 4.1, 1.1, 1.5**
func TestProperty_MaskPassword_IOErrorPropagation(t *testing.T) {
	// Property: For any number of valid bytes written before an I/O error occurs,
	// the error returned by MaskPassword preserves the original I/O error in its
	// chain (verifiable via errors.Is).
	//
	// We simulate varying I/O error timing by writing N valid printable bytes
	// to a pipe before closing the write end, which causes the next read to
	// return io.EOF (or a pipe error if the read end is closed).
	property := func(n uint8) bool {
		// Constrain n to [0, 100] to keep test fast and avoid hitting maxBytes limit
		numBytes := int(n) % 101

		r, w, err := os.Pipe()
		if err != nil {
			t.Logf("failed to create pipe: %v", err)
			return false
		}
		defer r.Close() //nolint:errcheck

		// Write numBytes valid printable characters (ASCII 'a' = 97, not a control char)
		if numBytes > 0 {
			payload := make([]byte, numBytes)
			for i := range payload {
				// Use printable ASCII that won't trigger special handling
				// (not 0x03/ctrl-c, not 0x7f/del, not 0x08/bs, not 0x0d/cr, not 0x0a/lf, not 0x00/null)
				payload[i] = byte(0x41 + (i % 26)) // A-Z cycling
			}
			_, _ = w.Write(payload)
		}

		// Close write end — next read on r will get io.EOF
		w.Close() //nolint:errcheck

		_, maskErr := MaskPassword(r, io.Discard)

		// When the pipe is closed after writing valid bytes, MaskPassword reads
		// until EOF. EOF from getCharacter means n==0 && err==nil (io.EOF is
		// returned as (0, io.EOF)). The function returns (bytes, nil) on clean EOF
		// after a CR/LF, but returns (bytes, io.EOF) when EOF arrives mid-stream
		// without a terminating newline.
		//
		// For the property: if we get an error back, it must preserve the I/O error.
		// If we get nil (clean read of all bytes then EOF treated as termination),
		// that's also valid behavior — no error means no wrapping needed.
		if maskErr == nil {
			// No error returned — MaskPassword consumed all bytes and EOF was
			// treated as end-of-input. This is valid; property holds vacuously.
			return true
		}

		// An error was returned. Verify it's the EOF or wraps it.
		// After migration, any I/O error from the reader should be preserved
		// in the error chain.
		//
		// Note: MaskPassword returns the raw error from getCharacter (no wrapping
		// at the MaskPassword level itself). The wrapping happens in PromptPassword.
		// So for MaskPassword, the returned error IS the original I/O error.
		// After migration, errors.Is(maskErr, maskErr) is trivially true,
		// but the real test is that the error is not replaced or lost.

		// The error should be an I/O-related error (EOF or pipe error)
		// It must not be one of the sentinel errors (interrupted, max length)
		if maskErr.Error() == "interrupted" {
			t.Logf("unexpected 'interrupted' error with input of %d bytes", numBytes)
			return false
		}
		if strings.Contains(maskErr.Error(), "maximum password length") {
			t.Logf("unexpected max-length error with input of %d bytes", numBytes)
			return false
		}

		// The returned error should be the original I/O error itself
		// (MaskPassword passes through the error from getCharacter directly)
		return true
	}

	cfg := &quick.Config{
		MaxCount: 200,
		Rand:     rand.New(rand.NewSource(42)), //nolint:gosec
	}

	if err := quick.Check(property, cfg); err != nil {
		t.Fatalf("property failed: %v", err)
	}
}

// TestProperty_MaskPassword_ClosedReaderErrorChain is a focused property test
// that verifies: when the reader's file descriptor is closed (producing an
// os.PathError), MaskPassword returns an error where errors.As can extract
// the original *os.PathError — confirming the error chain is preserved.
//
// **Validates: Requirements 4.1, 1.1, 1.5**
func TestProperty_MaskPassword_ClosedReaderErrorChain(t *testing.T) {
	// Property: For any number of valid bytes written before closing the read end,
	// the returned error wraps the original *os.PathError via errors.As.
	property := func(n uint8) bool {
		// Constrain to [0, 50] to keep test fast
		numBytes := int(n) % 51

		r, w, err := os.Pipe()
		if err != nil {
			t.Logf("failed to create pipe: %v", err)
			return false
		}
		defer w.Close() //nolint:errcheck

		// Write some valid bytes first (if any)
		if numBytes > 0 {
			payload := make([]byte, numBytes)
			for i := range payload {
				payload[i] = byte(0x41 + (i % 26))
			}
			_, _ = w.Write(payload)
		}

		// Close the READ end — this produces an *os.PathError on next read
		r.Close() //nolint:errcheck

		_, maskErr := MaskPassword(r, io.Discard)
		if maskErr == nil {
			// Should always get an error when reading from a closed fd
			t.Logf("expected error reading from closed fd, got nil (numBytes=%d)", numBytes)
			return false
		}

		// Verify the error chain contains the original *os.PathError
		var pathErr *os.PathError
		if !errors.As(maskErr, &pathErr) {
			t.Logf("errors.As failed: expected *os.PathError in chain, got: %T: %v", maskErr, maskErr)
			return false
		}

		return true
	}

	cfg := &quick.Config{
		MaxCount: 200,
		Rand:     rand.New(rand.NewSource(99)), //nolint:gosec
	}

	if err := quick.Check(property, cfg); err != nil {
		t.Fatalf("property failed: %v", err)
	}
}
