---
inclusion: auto
description: Project-specific patterns, preferences, and lessons learned over time (user-editable)
---

# Lessons Learned

This file captures project-specific patterns, coding preferences, common pitfalls, and architectural decisions that emerge during development. It serves as a workaround for continuous learning by allowing you to document patterns manually.

**How to use this file:**
1. The `extract-patterns` hook will suggest patterns after agent sessions
2. Review suggestions and add genuinely useful patterns below
3. Edit this file directly to capture team conventions
4. Keep it focused on project-specific insights, not general best practices

---

## Project-Specific Patterns

*Document patterns unique to this project that the team should follow.*

### Example: API Error Handling
```typescript
// Always use our custom ApiError class for consistent error responses
throw new ApiError(404, 'Resource not found', { resourceId });
```

---

## Code Style Preferences

*Document team preferences that go beyond standard linting rules.*

### Example: Import Organization
```typescript
// Group imports: external, internal, types
import { useState } from 'react';
import { Button } from '@/components/ui';
import type { User } from '@/types';
```

---

## Kiro Hooks

### `install.sh` is additive-only — it won't update existing installations
The installer skips any file that already exists in the target (`if [ ! -f ... ]`). Running it against a folder that already has `.kiro/` will not overwrite or update hooks, agents, or steering files. To push updates to an existing project, manually copy the changed files or remove the target files first before re-running the installer.

### README.md mirrors hook configurations — keep them in sync
The hooks table and Example 5 in README.md document the action type (`runCommand` vs `askAgent`) and behavior of each hook. When changing a hook's `then.type` or behavior, update both the hook file and the corresponding README entries to avoid misleading documentation.

### Prefer `askAgent` over `runCommand` for file-event hooks
`runCommand` hooks on `fileEdited` or `fileCreated` events spawn a new terminal session every time they fire, creating friction. Use `askAgent` instead so the agent handles the task inline. Reserve `runCommand` for `userTriggered` hooks where a manual, isolated terminal run is intentional (e.g., `quality-gate`).

---

## Common Pitfalls

*Document mistakes that have been made and how to avoid them.*

### Docker Desktop LinuxKit has a monolithic kernel
Docker Desktop's LinuxKit kernel has no loadable modules — `/proc/modules` is empty. Integration tests that assert on loaded modules must use `t.Skip()` when the map is empty, with a clear skip message. Tests still validate on real Linux hosts (CI) where modules are present.

### Integration tests writing to `/proc/sys/` need `--privileged`
The `docker run` command for Linux-only integration tests that write to `/proc/sys/` paths (sysctl tests) requires the `--privileged` flag. Without it, `/proc/sys` is mounted read-only and writes fail with permission errors. Document this in test execution commands:
```bash
docker run --rm --privileged -v "$(pwd)":/app -w /app golang:1.21.9-bookworm \
  go test -v ./internal/iface/osimpl/...
```

### `moby/sys/reexec` matches `os.Args[0]` exactly — register full paths
The `github.com/moby/sys/reexec` package (v0.1.0) uses `os.Args[0]` verbatim as the map key in `Init()`. Unlike the old `github.com/moby/moby/pkg/reexec` which used `filepath.Base(os.Args[0])`, the new package does NOT strip directory prefixes. When the kernel boots the binary as `/init` or systemd invokes `/sbin/init`, the registration must use the full path (`"/init"`, `"/sbin/init"`), not just the basename (`"init"`). Basename-only registrations work for programmatic reexec (where the caller controls argv[0]) but fail for kernel/init-system invocations.

### Pre-existing `pault.ag/go/modprobe` typecheck on macOS is expected
Running `golangci-lint run ./...` on macOS produces typecheck errors for `unix.FinitModule`/`unix.DeleteModule` (Linux-only syscalls in the modprobe dependency). This is not a blocker — it's a known limitation of linting Linux-only code on Darwin. Scoped lint (`golangci-lint run ./internal/iface/osimpl/...`) passes cleanly.

### Pre-commit `go-mod-tidy` hook captures dependency changes automatically
The pre-commit hook runs `go mod tidy` on every commit. When a migration removes a dependency from source files, the `go.mod`/`go.sum` cleanup happens automatically during the commit that removes the last import. A separate "remove dependency" commit may be empty — check `git status` before creating it. Plan for this in task specs to avoid no-op commits.

### Extract package-level variables for testability of hardcoded paths
Linux-only code that reads from `/proc/*` paths can be made testable by extracting the path into a `var` (e.g., `var procFilesystemsPath = "/proc/filesystems"`). Tests override the variable to point at a temp file with controlled content, using `t.Cleanup` to restore the original. This avoids needing root, Linux, or Docker for unit tests of pure logic.

---

## Architecture Decisions

*Document key architectural decisions and their rationale.*

### Example: State Management
- **Decision**: Use Zustand for global state, React Context for component trees
- **Rationale**: Zustand provides better performance and simpler API than Redux
- **Trade-offs**: Less ecosystem tooling than Redux, but sufficient for our needs

---

## Notes

- Keep entries concise and actionable
- Remove patterns that are no longer relevant
- Update patterns as the project evolves
- Focus on what's unique to this project
