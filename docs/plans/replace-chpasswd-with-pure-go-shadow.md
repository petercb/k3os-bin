# Replace chpasswd Shell-Outs with Pure Go Shadow Package

## Problem

The codebase shells out to `chpasswd` in three places and directly manipulates
`/etc/shadow` in a fourth. This creates:

- External binary dependency (`chpasswd` must be on PATH)
- Testability issues (tests must mock command execution rather than logic)
- Security surface area (subprocess spawning with passwords on stdin)

## Solution

Create a new `internal/shadow` package that provides:

1. **Password hashing** using SHA-512 crypt (`$6$`) via `github.com/GehirnInc/crypt`
2. **Shadow file read/update** using the existing `iface.FileSystem` abstraction
3. **Pre-hashed password detection** (passwords starting with `$` are used directly)

## Call Sites to Update

| Location | Current Behavior | New Behavior |
|----------|-----------------|--------------|
| `internal/cc/funcs.go` `ApplyPassword()` | Calls `chpasswd` via CommandRunner | Uses `shadow.PasswordSetter.SetPassword()` |
| `internal/command/command.go` `SetPassword()` | Calls `chpasswd` directly | Uses `shadow.Setter` with real FS |
| `internal/cliinstall/ask.go` `AskPassword()` | Calls `chpasswd`, reads shadow back | Uses `shadow.HashPassword()` to produce hash |
| `internal/boot/bootstrap/bootstrap.go` `SetupUsers()` | Appends `rancher:*:::::::` (already pure Go) | No change needed |

## Package Design

```go
// internal/shadow/shadow.go

// PasswordSetter sets a user's password in /etc/shadow.
type PasswordSetter interface {
    SetPassword(fs iface.FileSystem, username, password string) error
}

// Setter implements PasswordSetter using SHA-512 crypt.
type Setter struct{}

// HashPassword hashes a plaintext password using SHA-512 crypt ($6$).
// Returns the full hash string suitable for /etc/shadow.
func HashPassword(plaintext string) (string, error)
```

## Dependencies

- `github.com/GehirnInc/crypt` - provides SHA-512 crypt implementation
- Already uses `golang.org/x/crypto` indirectly (the user referenced
  `golang.org/x/crypto/crypt` which does not exist as a standalone package;
  `GehirnInc/crypt` uses crypto primitives internally)

## Testing Strategy

- TDD approach: write tests first
- Use `iface.FileSystem` mock for shadow file I/O
- Test cases: plaintext hashing, pre-hashed passthrough, user-not-found,
  empty password no-op, read/write error propagation
- No root or real filesystem access required
