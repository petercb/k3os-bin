package config

import (
	"fmt"
	"reflect"
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

	// Check singular -> plural (add "s")
	if normalizedKey+"s" == normalizedField {
		return true
	}
	// Check singular -> plural (add "es")
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

	// Try without underscores for compound words like "datasource" matching "data_sources"
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

// stringToBoolHookFunc converts string "true"/"false" to bool.
func stringToBoolHookFunc() mapstructure.DecodeHookFuncType {
	return func(from reflect.Type, to reflect.Type, data interface{}) (interface{}, error) {
		if from.Kind() != reflect.String || to.Kind() != reflect.Bool {
			return data, nil
		}
		str, _ := data.(string)
		switch strings.ToLower(str) {
		case "true":
			return true, nil
		case "false":
			return false, nil
		default:
			return data, nil
		}
	}
}

// stringToSliceHookFunc converts a single string to []string.
func stringToSliceHookFunc() mapstructure.DecodeHookFuncType {
	return func(from reflect.Type, to reflect.Type, data interface{}) (interface{}, error) {
		if from.Kind() != reflect.String {
			return data, nil
		}
		if to.Kind() != reflect.Slice || to.Elem().Kind() != reflect.String {
			return data, nil
		}
		return []string{data.(string)}, nil
	}
}

// mapToStringMapHookFunc converts map[string]interface{} to map[string]string.
func mapToStringMapHookFunc() mapstructure.DecodeHookFuncType {
	return func(from reflect.Type, to reflect.Type, data interface{}) (interface{}, error) {
		if from.Kind() != reflect.Map || from.Key().Kind() != reflect.String {
			return data, nil
		}
		if to.Kind() != reflect.Map || to.Key().Kind() != reflect.String || to.Elem().Kind() != reflect.String {
			return data, nil
		}
		m, ok := data.(map[string]interface{})
		if !ok {
			return data, nil
		}
		result := make(map[string]string, len(m))
		for k, v := range m {
			result[k] = fmt.Sprint(v)
		}
		return result, nil
	}
}

// decodeToObj decodes a map into a struct using mapstructure with custom hooks.
func decodeToObj(data interface{}, result interface{}) error {
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Result:    result,
		TagName:   "json",
		MatchName: matchName,
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			stringToBoolHookFunc(),
			stringToSliceHookFunc(),
			mapToStringMapHookFunc(),
		),
		WeaklyTypedInput: true,
	})
	if err != nil {
		return err
	}
	return decoder.Decode(data)
}

// normalizeData applies fuzzy name normalization to raw config map data.
// With mapstructure's MatchName function, normalization happens during decode.
// This function exists for any future pre-processing needs.
func normalizeData(_ map[string]interface{}) {
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
