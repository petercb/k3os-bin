package config

import (
	"strings"

	"dario.cat/mergo"
	"github.com/go-viper/mapstructure/v2"
)

// matchName returns true if mapKey matches fieldName using fuzzy matching rules.
func matchName(mapKey, fieldName string) bool {
	normalizedKey := strings.ToLower(camelToSnake(mapKey))
	normalizedField := strings.ToLower(camelToSnake(fieldName))

	if normalizedKey == normalizedField {
		return true
	}

	// Singular -> plural
	if normalizedKey+"s" == normalizedField {
		return true
	}
	if normalizedKey+"es" == normalizedField {
		return true
	}

	// Special aliases
	aliases := map[string]string{
		"pass":     "passphrase",
		"password": "passphrase",
	}
	if target, ok := aliases[normalizedKey]; ok {
		if normalizedField == target || normalizedField == target+"s" {
			return true
		}
	}

	// Without underscores for compound words
	keyNoSep := strings.ReplaceAll(normalizedKey, "_", "")
	fieldNoSep := strings.ReplaceAll(normalizedField, "_", "")
	if keyNoSep == fieldNoSep {
		return true
	}
	if keyNoSep+"s" == fieldNoSep {
		return true
	}
	if keyNoSep+"es" == fieldNoSep {
		return true
	}

	return false
}

// decodeToObj decodes a map into a struct using mapstructure with weak type coercion.
func decodeToObj(data interface{}, result interface{}) error {
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Result:           result,
		TagName:          "json",
		MatchName:        matchName,
		WeaklyTypedInput: true,
	})
	if err != nil {
		return err
	}
	return decoder.Decode(data)
}

// normalizeData converts all map keys to lowercase snake_case for consistent merging.
func normalizeData(data map[string]interface{}) {
	if data == nil {
		return
	}
	for key, val := range data {
		if subMap, ok := val.(map[string]interface{}); ok {
			normalizeData(subMap)
		}
		normalized := strings.ToLower(camelToSnake(key))
		if normalized != key {
			delete(data, key)
			data[normalized] = val
		}
	}
}

// mergeData merges src into dst with override semantics.
func mergeData(dst, src map[string]interface{}) (map[string]interface{}, error) {
	if src == nil {
		return dst, nil
	}
	if dst == nil {
		dst = make(map[string]interface{})
	}
	if err := mergo.Merge(&dst, src, mergo.WithOverride); err != nil {
		return nil, err
	}
	return dst, nil
}
