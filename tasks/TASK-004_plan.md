# TASK-004 Execution Plan: Add unit tests for `internal/config` (read, merge)

## Objective

Test the configuration reading and merging logic within the `internal/config` package (`read.go`), aiming to achieve ≥60% test coverage for the file. The tests should verify that configurations are loaded correctly from various sources, merged with the correct priority, and that the kernel command line parameters are parsed properly.

## Scope

The tests will cover the following functions and components from `internal/config/read.go`:

- `readCmdline()`
- `readFile()`
- `readLocalConfigs()`
- `merge()`
- `readersToObject()`
- `mapToEnv()` and `ToEnv()`

## Implementation Steps

### 1. Test Fixtures & Setup

- Create temporary directories using `t.TempDir()` for dynamic file tests (like `config.d` scanning).
- Create static fixture files in `internal/config/testdata/` for complex multi-source merge scenarios (e.g., `system_config.yaml`, `local_config.yaml`, and `cmdline.txt`).

### 2. Test `mapToEnv` & `ToEnv`

- **Scenario:** Flat map conversion.
  - Verify keys are converted to uppercase (e.g., `hostname` -> `HOSTNAME`).
- **Scenario:** Nested map conversion.
  - Verify nested structures are flattened with underscore separators (e.g., `k3os.wifi.network` -> `K3OS_WIFI_NETWORK`).
- **Scenario:** Array and boolean values.
  - Verify non-string types format appropriately into string environment variables.

### 3. Test `readCmdline`

- **Scenario:** Standard key-value pairs.
  - Verify `k3os.hostname=myhost` sets the nested structure correctly.
- **Scenario:** Quoted values.
  - Verify keys with quoted values (`k3os.password="pass"`) trim quotes correctly.
- **Scenario:** List values.
  - Verify repeating a key appends to a list instead of overwriting.
- **Edge Cases:** Missing `/proc/cmdline` file (should handle gracefully without error), and malformed formats.

### 4. Test `readFile`

- **Scenario:** Existing valid YAML file.
  - Verify the YAML parses correctly into a map.
- **Edge Case:** Non-existent file.
  - Verify it returns a `nil` map and `nil` error (graceful fallback).
- **Edge Case:** Malformed YAML.
  - Verify it returns a parsing error.

### 5. Test `readLocalConfigs`

- **Scenario:** Valid `config.d` directory.
  - Use `t.TempDir()` to simulate a `config.d` folder containing multiple `.yaml` files.
  - Verify the function returns the correct number of reader functions.
- **Edge Cases:** Empty directory, and non-existent directory.

### 6. Test `merge`

- **Scenario:** Multi-source merge priority.
  - Pass multiple reader functions simulating `System < Local < config.d` configs.
  - Verify that later readers correctly override values from earlier readers.
- **Scenario:** Schema type coercion during merge.
  - Ensure the schemas (`NewToMap`, `NewToSlice`, `FuzzyNames`) are applied during the merge step.

### 7. Test `readersToObject`

- **Scenario:** Full pipeline execution.
  - Pass readers that return map data and verify the final `CloudConfig` object is correctly populated.

### 8. Final Validation (TDD Check)

- Run `go test -race -coverprofile=coverage.out ./internal/config/...` inside the dev container.
- Use `go tool cover -func=coverage.out` to verify that coverage for `read.go` is ≥60%.
- Ensure tests pass consistently on multiple runs to prevent flakiness.
