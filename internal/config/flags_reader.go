package config

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

// flagsFile is the path to the optional flags configuration file.
// It can be overridden in tests.
var flagsFile = "/run/k3os/config.flags"

// readFlagsFile reads key=value pairs from the flags file and returns
// a nested map using the same dot-notation semantics as cmdline parsing.
// Missing file is not an error (returns nil, nil).
// The file format is one key=value per line. Lines starting with '#' are
// comments. Values may be Go-quoted (strconv.Unquote) or plain strings.
func readFlagsFile() (map[string]interface{}, error) {
	f, err := os.Open(flagsFile)
	if os.IsNotExist(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	defer f.Close() //nolint:errcheck // best-effort close on read-only file

	data := map[string]interface{}{}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value := splitFlagLine(line)
		if key == "" {
			continue
		}

		keys := strings.Split(key, ".")

		existing, ok := getValue(data, keys...)
		if ok {
			switch v := existing.(type) {
			case string:
				putValue(data, []string{v, value}, keys...)
			case []string:
				putValue(data, append(v, value), keys...)
			}
		} else {
			putValue(data, value, keys...)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	if len(data) == 0 {
		return nil, nil
	}

	return data, nil
}

// splitFlagLine splits a line on the first '=' into key and value.
// If there is no '=', the key is the whole line and value is "true".
// Values are unquoted using strconv.Unquote if they appear Go-quoted.
func splitFlagLine(line string) (string, string) {
	parts := strings.SplitN(line, "=", 2)
	if len(parts) == 1 {
		return strings.TrimSpace(parts[0]), "true"
	}

	key := strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])

	// Attempt Go unquoting for quoted values.
	if unquoted, err := strconv.Unquote(value); err == nil {
		value = unquoted
	}

	return key, value
}
