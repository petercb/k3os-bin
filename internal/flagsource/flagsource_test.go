package flagsource

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/petercb/k3os-bin/internal/cmdline"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCmdlineSource(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		raw      string
		flagName string
		want     []string
	}{
		{
			name:     "flag present with single arg",
			raw:      "uroot.uinitargs=--verbose quiet",
			flagName: "uroot.uinitargs",
			want:     []string{"--verbose"},
		},
		{
			name:     "flag missing",
			raw:      "root=/dev/sda1 quiet",
			flagName: "uroot.uinitargs",
			want:     nil,
		},
		{
			name:     "flag present but empty value",
			raw:      "uroot.uinitargs= quiet",
			flagName: "uroot.uinitargs",
			want:     nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			parser := cmdline.NewFromString(tc.raw)
			src := &CmdlineSource{
				Parser:   parser,
				FlagName: tc.flagName,
			}

			got, err := src.Args()
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestCmdlineSource_ShlexSplitting(t *testing.T) {
	t.Parallel()
	// Use a mock parser to verify shlex splitting behavior directly,
	// independent of the kernel cmdline quoting semantics.
	parser := &mockCmdlineParser{flags: map[string]string{
		"uroot.uinitargs": "-v --foobar",
	}}
	src := &CmdlineSource{
		Parser:   parser,
		FlagName: "uroot.uinitargs",
	}

	got, err := src.Args()
	require.NoError(t, err)
	assert.Equal(t, []string{"-v", "--foobar"}, got)
}

func TestFileSource(t *testing.T) {
	t.Parallel()

	t.Run("valid file with Go-quoted lines", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := filepath.Join(dir, "flags")
		content := strconv.Quote("-v") + "\n" +
			strconv.Quote("--config=/etc/app.conf") + "\n" +
			strconv.Quote("--timeout=30s") + "\n"
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

		src := &FileSource{Path: path}
		got, err := src.Args()
		require.NoError(t, err)
		assert.Equal(t, []string{"-v", "--config=/etc/app.conf", "--timeout=30s"}, got)
	})

	t.Run("missing file returns empty slice no error", func(t *testing.T) {
		t.Parallel()
		src := &FileSource{Path: "/nonexistent/path/flags"}
		got, err := src.Args()
		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("empty file returns empty slice", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := filepath.Join(dir, "flags")
		require.NoError(t, os.WriteFile(path, []byte(""), 0o644))

		src := &FileSource{Path: path}
		got, err := src.Args()
		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("blank lines and comments are skipped", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := filepath.Join(dir, "flags")
		content := "# This is a comment\n" +
			"\n" +
			strconv.Quote("--flag1") + "\n" +
			"   \n" +
			"  # Another comment\n" +
			strconv.Quote("--flag2") + "\n"
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

		src := &FileSource{Path: path}
		got, err := src.Args()
		require.NoError(t, err)
		assert.Equal(t, []string{"--flag1", "--flag2"}, got)
	})

	t.Run("malformed quote returns error", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := filepath.Join(dir, "flags")
		content := strconv.Quote("--good") + "\n" +
			"not-a-quoted-string\n"
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

		src := &FileSource{Path: path}
		_, err := src.Args()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "line 2")
	})
}

func TestEnvSource(t *testing.T) {
	t.Run("env var set with args", func(t *testing.T) {
		t.Setenv("TEST_FLAGSOURCE_ARGS", "-v --debug")

		src := &EnvSource{Name: "TEST_FLAGSOURCE_ARGS"}
		got, err := src.Args()
		require.NoError(t, err)
		assert.Equal(t, []string{"-v", "--debug"}, got)
	})

	t.Run("env var unset returns empty slice no error", func(t *testing.T) {
		src := &EnvSource{Name: "TEST_FLAGSOURCE_UNSET_VAR_XYZ"}
		got, err := src.Args()
		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("env var set but empty returns empty slice", func(t *testing.T) {
		t.Setenv("TEST_FLAGSOURCE_EMPTY", "")

		src := &EnvSource{Name: "TEST_FLAGSOURCE_EMPTY"}
		got, err := src.Args()
		require.NoError(t, err)
		assert.Empty(t, got)
	})
}

func TestMerge(t *testing.T) {
	t.Parallel()

	t.Run("multiple sources combined in order", func(t *testing.T) {
		t.Parallel()
		s1 := &staticSource{args: []string{"a", "b"}}
		s2 := &staticSource{args: []string{"c"}}
		s3 := &staticSource{args: []string{"d", "e"}}

		got, err := Merge(s1, s2, s3)
		require.NoError(t, err)
		assert.Equal(t, []string{"a", "b", "c", "d", "e"}, got)
	})

	t.Run("source returning error stops merge", func(t *testing.T) {
		t.Parallel()
		s1 := &staticSource{args: []string{"a"}}
		s2 := &errorSource{err: assert.AnError}
		s3 := &staticSource{args: []string{"should not appear"}}

		_, err := Merge(s1, s2, s3)
		require.ErrorIs(t, err, assert.AnError)
	})

	t.Run("all empty sources return empty slice", func(t *testing.T) {
		t.Parallel()
		s1 := &staticSource{}
		s2 := &staticSource{}

		got, err := Merge(s1, s2)
		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("single source works", func(t *testing.T) {
		t.Parallel()
		s1 := &staticSource{args: []string{"only"}}

		got, err := Merge(s1)
		require.NoError(t, err)
		assert.Equal(t, []string{"only"}, got)
	})

	t.Run("no sources returns empty slice", func(t *testing.T) {
		t.Parallel()

		got, err := Merge()
		require.NoError(t, err)
		assert.Empty(t, got)
	})
}

// staticSource is a test helper that returns a fixed args slice.
type staticSource struct {
	args []string
}

func (s *staticSource) Args() ([]string, error) {
	return s.args, nil
}

// errorSource is a test helper that always returns an error.
type errorSource struct {
	err error
}

func (s *errorSource) Args() ([]string, error) {
	return nil, s.err
}

// mockCmdlineParser is a test helper that returns pre-configured flag values.
type mockCmdlineParser struct {
	flags map[string]string
}

func (m *mockCmdlineParser) Flag(name string) (string, bool) {
	v, ok := m.flags[name]
	return v, ok
}

func (m *mockCmdlineParser) Contains(name string) bool {
	_, ok := m.flags[name]
	return ok
}

func (m *mockCmdlineParser) Consoles() []string { return nil }
func (m *mockCmdlineParser) Raw() string        { return "" }
