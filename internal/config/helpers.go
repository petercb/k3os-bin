package config

import (
	"encoding/json"
	"fmt"
	"unicode"
)

// camelToSnake converts a camelCase or PascalCase string to snake_case.
func camelToSnake(s string) string {
	if s == "" {
		return ""
	}
	var result []rune
	runes := []rune(s)
	for i, r := range runes {
		if i > 0 && unicode.IsUpper(r) {
			prev := runes[i-1]
			if unicode.IsLower(prev) || unicode.IsDigit(prev) {
				result = append(result, '_')
			} else if unicode.IsUpper(prev) && i+1 < len(runes) && unicode.IsLower(runes[i+1]) {
				result = append(result, '_')
			}
		}
		result = append(result, unicode.ToLower(r))
	}
	return string(result)
}

// toString converts any value to its string representation.
func toString(v interface{}) string {
	return fmt.Sprint(v)
}

// encodeToMap converts a struct to map[string]interface{} via JSON round-trip.
func encodeToMap(obj interface{}) (map[string]interface{}, error) {
	data, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	result := map[string]interface{}{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// getValue retrieves a value from a nested map using a sequence of keys.
func getValue(data map[string]interface{}, keys ...string) (interface{}, bool) {
	if len(keys) == 0 {
		return nil, false
	}
	current := interface{}(data)
	for _, key := range keys {
		m, ok := current.(map[string]interface{})
		if !ok {
			return nil, false
		}
		current, ok = m[key]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

// putValue sets a value in a nested map, creating intermediate maps as needed.
func putValue(data map[string]interface{}, val interface{}, keys ...string) {
	if len(keys) == 0 {
		return
	}
	for i := 0; i < len(keys)-1; i++ {
		next, ok := data[keys[i]]
		if !ok {
			next = map[string]interface{}{}
			data[keys[i]] = next
		}
		nextMap, ok := next.(map[string]interface{})
		if !ok {
			nextMap = map[string]interface{}{}
			data[keys[i]] = nextMap
		}
		data = nextMap
	}
	data[keys[len(keys)-1]] = val
}
