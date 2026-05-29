package config

// decodeToObj decodes a map into a struct using mapstructure with custom hooks.
func decodeToObj(data interface{}, result interface{}) error {
	_ = data
	_ = result
	panic("not implemented")
}

// normalizeData applies fuzzy name normalization to raw config map data.
//
//nolint:unused // stub for TDD - will be implemented in FEAT-003
func normalizeData(data map[string]interface{}) {
	_ = data
	panic("not implemented")
}

// matchName returns true if mapKey matches fieldName using fuzzy matching rules.
func matchName(mapKey, fieldName string) bool {
	_ = mapKey
	_ = fieldName
	panic("not implemented")
}

// mergeData merges src into dst with override semantics.
func mergeData(dst, src map[string]interface{}) (map[string]interface{}, error) {
	_ = dst
	_ = src
	panic("not implemented")
}
