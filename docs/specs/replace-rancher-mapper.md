# Migration: Replace rancher/mapper with mapstructure/v2 and mergo

## Summary

Removed the `github.com/rancher/mapper` dependency (and its transitive dependencies
`rancher/wrangler`, `ghodss/yaml`, `gopkg.in/yaml.v2`, all k8s.io replace directives)
in favor of two well-maintained, lightweight libraries:

- `github.com/go-viper/mapstructure/v2` for struct decoding with custom hooks
- `dario.cat/mergo` for deep map merging with override semantics

## Replacement Mapping

| Old (rancher/mapper) | New | Purpose |
|---|---|---|
| `mapper.NewSchemas().MustImport()` | `mapstructure.NewDecoder` with `MatchName` | Schema-based field discovery |
| `FuzzyNames` mapper | `matchName` function | Fuzzy field name matching |
| `NewToBool()` mapper | `stringToBoolHookFunc()` | String "true"/"false" to bool |
| `NewToSlice()` mapper | `stringToSliceHookFunc()` | Single string to []string |
| `NewToMap()` mapper | `mapToStringMapHookFunc()` | map[string]interface{} to map[string]string |
| `convert.ToObj(data, &result)` | `decodeToObj(data, &result)` | Map to struct conversion |
| `convert.EncodeToMap(&cfg)` | YAML round-trip (marshal/unmarshal) | Struct to map conversion |
| `convert.ToYAMLKey(k)` | `camelToSnake(k)` | camelCase to snake_case |
| `merge2.UpdateMerge(...)` | `mergeData(dst, src)` using mergo | Deep map merge with override |
| `values.GetValue` / `values.PutValue` | Local `getValue` / `putValue` | Nested map access/mutation |

## Design Decisions

1. **YAML round-trip for struct-to-map**: Since all structs already have both `json` and
   `yaml` tags, using `yaml.Marshal` followed by `yaml.Unmarshal` into
   `map[string]interface{}` produces correctly-keyed maps (snake_case) without needing
   a separate key-conversion step.

2. **Fuzzy matching via `matchName`**: The `mapstructure` library supports a custom
   `MatchName` function. Our implementation normalizes both the map key and struct field
   name to lowercase snake_case, then checks for exact match, singular-to-plural
   variants, special aliases (password/pass to passphrase), and compound-word matching
   (removing underscores for comparison).

3. **mergo with override**: `mergo.Merge(&dst, src, mergo.WithOverride)` provides the
   same last-writer-wins semantics that `rancher/mapper`'s `UpdateMerge` provided,
   with recursive map merging.

4. **Simplified write.go**: Since structs have `yaml:"..."` tags with the correct
   snake_case names, `yaml.Marshal` on the struct directly produces the desired YAML
   output without any intermediate map conversion or key transformation.

5. **Removed all k8s.io replace directives**: These were only needed as transitive
   dependencies of `rancher/wrangler` (pulled in by `rancher/mapper`). With mapper
   removed, all nine k8s.io replace directives are no longer needed.
