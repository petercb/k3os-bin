// Package flagsource implements the u-root pattern of merging kernel cmdline
// args + flags file + environment into a single args slice. It provides
// composable, testable building blocks for argument resolution from multiple
// sources.
package flagsource

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/petercb/k3os-bin/internal/iface"
	"github.com/u-root/u-root/pkg/shlex"
)

// Source provides command-line arguments from a single origin.
type Source interface {
	Args() ([]string, error)
}

// CmdlineSource extracts a named flag from the kernel cmdline and splits its
// value into arguments using shell-like parsing (shlex.Argv).
type CmdlineSource struct {
	Parser   iface.CmdlineParser
	FlagName string
}

// Args returns the arguments parsed from the kernel cmdline flag value.
// If the flag is not present or empty, it returns nil and no error.
func (s *CmdlineSource) Args() ([]string, error) {
	val, ok := s.Parser.Flag(s.FlagName)
	if !ok || val == "" {
		return nil, nil
	}

	args := shlex.Argv(val)
	if len(args) == 0 {
		return nil, nil
	}

	return args, nil
}

// FileSource reads a flags file in the uflag format: each non-empty,
// non-comment line contains a single Go-quoted string (strconv.Unquote).
// A missing file is not an error (the file is optional).
type FileSource struct {
	Path string
}

// Args returns the arguments parsed from the flags file.
// If the file does not exist, it returns nil and no error.
func (s *FileSource) Args() ([]string, error) {
	f, err := os.Open(s.Path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}

		return nil, err
	}
	defer f.Close() //nolint:errcheck // read-only file

	var args []string

	scanner := bufio.NewScanner(f)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		unquoted, err := strconv.Unquote(line)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNum, err)
		}

		args = append(args, unquoted)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return args, nil
}

// EnvSource reads an environment variable and splits its value into arguments
// using shell-like parsing (shlex.Argv).
type EnvSource struct {
	Name string
}

// Args returns the arguments parsed from the environment variable value.
// If the variable is not set or empty, it returns nil and no error.
func (s *EnvSource) Args() ([]string, error) {
	val, ok := os.LookupEnv(s.Name)
	if !ok || val == "" {
		return nil, nil
	}

	args := shlex.Argv(val)
	if len(args) == 0 {
		return nil, nil
	}

	return args, nil
}

// Merge concatenates the arguments from all sources in order.
// If any source returns an error, Merge stops and returns that error.
func Merge(sources ...Source) ([]string, error) {
	var merged []string

	for _, src := range sources {
		args, err := src.Args()
		if err != nil {
			return nil, err
		}

		merged = append(merged, args...)
	}

	return merged, nil
}
