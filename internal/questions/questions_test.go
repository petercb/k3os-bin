package questions

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pipeStdin creates a pipe, replaces os.Stdin, and returns a writer.
// The cleanup restores the original stdin and closes the read end.
func pipeStdin(t *testing.T) *os.File {
	t.Helper()

	r, w, err := os.Pipe()
	require.NoError(t, err)

	oldStdin := os.Stdin
	os.Stdin = r

	t.Cleanup(func() {
		os.Stdin = oldStdin
		_ = r.Close() //nolint:errcheck // best-effort cleanup in tests
	})

	return w
}

func TestPromptFormattedOptions_SingleOption(t *testing.T) {
	// When only one option is provided, it should return 0 without prompting.
	result, err := PromptFormattedOptions("Pick one:", 0, "only-option")
	require.NoError(t, err)
	assert.Equal(t, 0, result)
}

func TestPromptOptions_SingleOption(t *testing.T) {
	// When only one option is provided, it should return 0 without prompting.
	result, err := PromptOptions("Pick one:", 0, "only-option")
	require.NoError(t, err)
	assert.Equal(t, 0, result)
}

func TestPromptOptions_ValidSelection(t *testing.T) {
	w := pipeStdin(t)

	_, err := w.WriteString("2\n")
	require.NoError(t, err)
	require.NoError(t, w.Close())

	result, err := PromptOptions("Pick:", -1, "opt-a", "opt-b", "opt-c")
	require.NoError(t, err)
	assert.Equal(t, 1, result)
}

func TestPromptBool_Yes(t *testing.T) {
	w := pipeStdin(t)

	_, err := w.WriteString("y\n")
	require.NoError(t, err)
	require.NoError(t, w.Close())

	result, err := PromptBool("Continue?", false)
	require.NoError(t, err)
	assert.True(t, result)
}

func TestPromptBool_No(t *testing.T) {
	w := pipeStdin(t)

	_, err := w.WriteString("n\n")
	require.NoError(t, err)
	require.NoError(t, w.Close())

	result, err := PromptBool("Continue?", true)
	require.NoError(t, err)
	assert.False(t, result)
}

func TestPromptBool_Default(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		def      bool
		expected bool
	}{
		{name: "default true with y", input: "y\n", def: true, expected: true},
		{name: "default false with n", input: "n\n", def: false, expected: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := pipeStdin(t)

			_, err := w.WriteString(tc.input)
			require.NoError(t, err)
			require.NoError(t, w.Close())

			result, err := PromptBool("Continue?", tc.def)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestPrintToTerm_NonTTY(t *testing.T) {
	// In a test environment, stdout is not a TTY, so PrintToTerm should
	// write to stderr. Capture stderr to verify.
	oldStderr := os.Stderr
	r, w, err := os.Pipe()
	require.NoError(t, err)

	os.Stderr = w

	t.Cleanup(func() {
		os.Stderr = oldStderr
	})

	PrintToTerm("hello")
	require.NoError(t, w.Close())

	var buf bytes.Buffer
	_, err = io.Copy(&buf, r)
	require.NoError(t, err)
	require.NoError(t, r.Close())

	assert.Equal(t, "hello", buf.String())
}

func TestPrintlnToTerm_NonTTY(t *testing.T) {
	oldStderr := os.Stderr
	r, w, err := os.Pipe()
	require.NoError(t, err)

	os.Stderr = w

	t.Cleanup(func() {
		os.Stderr = oldStderr
	})

	PrintlnToTerm("world")
	require.NoError(t, w.Close())

	var buf bytes.Buffer
	_, err = io.Copy(&buf, r)
	require.NoError(t, err)
	require.NoError(t, r.Close())

	assert.Equal(t, "world\n", buf.String())
}

func TestPrintfToTerm_NonTTY(t *testing.T) {
	oldStderr := os.Stderr
	r, w, err := os.Pipe()
	require.NoError(t, err)

	os.Stderr = w

	t.Cleanup(func() {
		os.Stderr = oldStderr
	})

	PrintfToTerm("count: %d", 42)
	require.NoError(t, w.Close())

	var buf bytes.Buffer
	_, err = io.Copy(&buf, r)
	require.NoError(t, err)
	require.NoError(t, r.Close())

	assert.Equal(t, "count: 42", buf.String())
}

func TestPrompt_ReturnsUserInput(t *testing.T) {
	w := pipeStdin(t)

	_, err := w.WriteString("my-answer\n")
	require.NoError(t, err)
	require.NoError(t, w.Close())

	result, err := Prompt("Enter: ", "default")
	require.NoError(t, err)
	assert.Equal(t, "my-answer", result)
}

func TestPrompt_UsesDefault(t *testing.T) {
	w := pipeStdin(t)

	// Empty line triggers default
	_, err := w.WriteString("\n")
	require.NoError(t, err)
	require.NoError(t, w.Close())

	result, err := Prompt("Enter: ", "fallback")
	require.NoError(t, err)
	assert.Equal(t, "fallback", result)
}

func TestPrompt_ErrorOnClosedStdin(t *testing.T) {
	r, w, err := os.Pipe()
	require.NoError(t, err)

	oldStdin := os.Stdin
	os.Stdin = r

	t.Cleanup(func() {
		os.Stdin = oldStdin
	})

	// Close both ends so reading returns EOF immediately without any data.
	require.NoError(t, w.Close())
	require.NoError(t, r.Close())

	_, err = Prompt("Enter: ", "")
	assert.Error(t, err)
}

func TestPromptOptional_ReturnsDefault(t *testing.T) {
	w := pipeStdin(t)

	_, err := w.WriteString("\n")
	require.NoError(t, err)
	require.NoError(t, w.Close())

	result, err := PromptOptional("Enter: ", "optional-default")
	require.NoError(t, err)
	assert.Equal(t, "optional-default", result)
}

func TestPromptOptional_Error(t *testing.T) {
	r, w, err := os.Pipe()
	require.NoError(t, err)

	oldStdin := os.Stdin
	os.Stdin = r

	t.Cleanup(func() {
		os.Stdin = oldStdin
	})

	require.NoError(t, w.Close())
	require.NoError(t, r.Close())

	_, err = PromptOptional("Enter: ", "default")
	assert.Error(t, err)
}

func TestPromptFormattedOptions_MultipleOptions(t *testing.T) {
	w := pipeStdin(t)

	_, err := w.WriteString("1\n")
	require.NoError(t, err)
	require.NoError(t, w.Close())

	result, err := PromptFormattedOptions("Pick one:", 0, "first", "second")
	require.NoError(t, err)
	assert.Equal(t, 0, result)
}

func TestPromptOptional_ReturnsUserInput(t *testing.T) {
	w := pipeStdin(t)

	_, err := w.WriteString("user-input\n")
	require.NoError(t, err)
	require.NoError(t, w.Close())

	result, err := PromptOptional("Enter: ", "default")
	require.NoError(t, err)
	assert.Equal(t, "user-input", result)
}
