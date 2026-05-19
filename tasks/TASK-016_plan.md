# TASK-016 Execution Plan: Fix flaky TestFuzzyNames test in internal/config

## Objective

Refactor the `TestFuzzyNames` test to resolve the flaky assertion caused by Go's non-deterministic map iteration order, ensuring the test suite is deterministic and reliable.

## Context

The `TestFuzzyNames` test in `internal/config/rename_test.go` verifies that different config keys (like `"pass"` and `"password"`) correctly map to a canonical key (`"passphrase"`). Currently, the test provides a single input map with both `"pass"` and `"password"`. Since `FuzzyNames.ToInternal` iterates over the input map, and map iteration order in Go is randomized, it is non-deterministic which key is evaluated last. The test asserts that the resulting value in the map is `"my-password"` (which comes from `"password"`), but roughly half the time it evaluates `"pass"` last, resulting in `"my-pass"`, causing the test to fail.

## Implementation Steps

### 1. Refactor `TestFuzzyNames`

* Modify `TestFuzzyNames` in `internal/config/rename_test.go`.
* Separate the testing of `"pass"` and `"password"` so they don't overwrite each other non-deterministically.
* **Approach:**
  * Initialize the `schema` and `FuzzyNames` instance as before.
  * Create separate input data maps for each test case or use `t.Run` with table-driven tests.
  * Assert that `"pass"` maps to `"passphrase"` and its value is retained.
  * Assert that `"password"` maps to `"passphrase"` and its value is retained.
  * Keep the assertions for `"ssh_authorized_key"` and `"environment"` in a deterministic manner (they don't conflict, so they can be grouped together or separated).

### 2. Verify Fix (TDD / Local Testing)

* Run the specific test package with `-count=10` to guarantee the flakiness is resolved:

    ```bash
    go test ./internal/config/... -run TestFuzzyNames -count=10
    ```

* Ensure all 10 runs pass reliably.

### 3. Final Validation

* Run the full test suite (`go test -race -covermode=atomic -failfast ./...`) inside the devcontainer to ensure nothing else is broken.
* Check that code lints cleanly (no new issues introduced).
