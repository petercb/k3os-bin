package config

import (
	"errors"
	"os"
	"strings"
)

// readCmdline reads and parses the kernel command line from cmdlineFile.
// It produces a nested map where dot-separated keys become nested maps
// and repeated keys accumulate into string slices.
func readCmdline() (map[string]interface{}, error) {
	content, err := os.ReadFile(cmdlineFile)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	data := parseCmdlineArgs(string(content))
	if len(data) == 0 {
		return nil, nil
	}

	return data, nil
}

// parseCmdlineArgs parses a raw kernel command-line string into a nested map.
// Tokens are split on whitespace (respecting quoted fields). Each token is
// split on the first '=' to produce key/value pairs. Keys containing '.' are
// split into nested maps. Repeated keys are accumulated into string slices.
// Tokens without '=' are treated as boolean flags with the value "true".
func parseCmdlineArgs(raw string) map[string]interface{} {
	data := map[string]interface{}{}

	for _, token := range splitCmdlineTokens(raw) {
		if len(token) == 0 {
			continue
		}

		key, value := splitKeyValue(token)
		keys := strings.Split(strings.Trim(key, `"`), ".")

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

	return data
}

// splitCmdlineTokens splits a raw command-line string on whitespace while
// respecting ASCII double-quoted regions. Only ASCII '"' (0x22) is treated as
// a quote character, matching u-root's doParse tokenization and the old regex
// behavior.
func splitCmdlineTokens(raw string) []string {
	lastQuote := rune(0)
	return strings.FieldsFunc(raw, func(c rune) bool {
		switch {
		case c == lastQuote:
			lastQuote = rune(0)
			return false
		case lastQuote != rune(0):
			return false
		case c == '"':
			lastQuote = c
			return false
		default:
			return c == ' ' || c == '\t' || c == '\n' || c == '\r'
		}
	})
}

// splitKeyValue splits a token on the first '=' into key and value.
// If there is no '=', the entire token is the key and value is "true".
// Surrounding quotes are stripped from the value.
func splitKeyValue(token string) (string, string) {
	parts := strings.SplitN(token, "=", 2)
	if len(parts) == 1 {
		return parts[0], "true"
	}
	return parts[0], strings.Trim(parts[1], `"`)
}
