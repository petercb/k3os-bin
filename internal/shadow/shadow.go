// Package shadow provides pure-Go password hashing and /etc/shadow file manipulation.
package shadow

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/GehirnInc/crypt/sha512_crypt"
	"github.com/petercb/k3os-bin/internal/iface"
)

// PasswordSetter sets a user's password in /etc/shadow.
type PasswordSetter interface {
	SetPassword(fs iface.FileSystem, username string, password string) error
}

// Setter implements PasswordSetter using SHA-512 crypt for hashing.
type Setter struct{}

// SetPassword updates the password hash for username in /etc/shadow.
// Pre-hashed passwords (starting with '$') are used directly.
// Plaintext passwords are hashed using SHA-512 crypt before writing.
// An empty password is a no-op.
func (s Setter) SetPassword(fs iface.FileSystem, username string, password string) error {
	if password == "" {
		return nil
	}

	hash := password
	if !strings.HasPrefix(password, "$") {
		var err error
		hash, err = HashPassword(password)
		if err != nil {
			return fmt.Errorf("hashing password for %s: %w", username, err)
		}
	}

	data, err := fs.ReadFile("/etc/shadow")
	if err != nil {
		return fmt.Errorf("reading /etc/shadow: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	found := false
	for i, line := range lines {
		fields := strings.SplitN(line, ":", 3)
		if len(fields) >= 2 && fields[0] == username {
			// Replace the hash field (field index 1)
			rest := ""
			if len(fields) == 3 {
				rest = ":" + fields[2]
			}
			lines[i] = fields[0] + ":" + hash + rest
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("user %s not found in /etc/shadow", username)
	}

	output := strings.Join(lines, "\n")
	if err := fs.WriteFile("/etc/shadow", []byte(output), 0o640); err != nil {
		return fmt.Errorf("writing /etc/shadow: %w", err)
	}

	return nil
}

// HashPassword hashes a plaintext password using SHA-512 crypt ($6$),
// compatible with /etc/shadow format.
func HashPassword(plaintext string) (string, error) {
	salt, err := generateSalt()
	if err != nil {
		return "", fmt.Errorf("generating salt: %w", err)
	}

	crypt := sha512_crypt.New()
	hash, err := crypt.Generate([]byte(plaintext), []byte("$6$"+salt))
	if err != nil {
		return "", fmt.Errorf("generating SHA-512 crypt hash: %w", err)
	}

	return hash, nil
}

// generateSalt produces a random 16-byte base64-encoded salt string.
func generateSalt() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	// Use raw encoding (no padding) for cleaner salt values
	return base64.RawStdEncoding.EncodeToString(b), nil
}
