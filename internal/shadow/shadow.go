// Package shadow provides pure-Go password hashing and /etc/shadow file manipulation.
package shadow

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/go-crypt/crypt/algorithm/shacrypt"
	"github.com/petercb/k3os-bin/internal/iface"
)

// shadowPath is the path to the shadow password file.
const shadowPath = "/etc/shadow"

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
//
// The write is atomic: contents are written to a temporary file in the same
// directory and then renamed over the target, so a crash cannot leave a
// truncated shadow file.
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

	data, err := fs.ReadFile(shadowPath)
	if err != nil {
		return fmt.Errorf("reading %s: %w", shadowPath, err)
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
		return fmt.Errorf("user %s not found in %s", username, shadowPath)
	}

	output := strings.Join(lines, "\n")

	// Atomic write: create temp file in same directory, write, then rename.
	dir := filepath.Dir(shadowPath)
	tmp, err := fs.CreateTemp(dir, "shadow.*")
	if err != nil {
		return fmt.Errorf("creating temp file for %s: %w", shadowPath, err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write([]byte(output)); err != nil {
		_ = tmp.Close()
		_ = fs.Remove(tmpName)
		return fmt.Errorf("writing temp shadow file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = fs.Remove(tmpName)
		return fmt.Errorf("closing temp shadow file: %w", err)
	}

	if err := fs.Chmod(tmpName, 0o640); err != nil {
		_ = fs.Remove(tmpName)
		return fmt.Errorf("setting permissions on temp shadow file: %w", err)
	}

	if err := fs.Rename(tmpName, shadowPath); err != nil {
		_ = fs.Remove(tmpName)
		return fmt.Errorf("renaming temp file to %s: %w", shadowPath, err)
	}

	return nil
}

// HashPassword hashes a plaintext password using SHA-512 crypt ($6$),
// compatible with /etc/shadow format.
func HashPassword(plaintext string) (string, error) {
	hasher, err := shacrypt.New(
		shacrypt.WithVariant(shacrypt.VariantSHA512),
	)
	if err != nil {
		return "", fmt.Errorf("creating SHA-512 crypt hasher: %w", err)
	}

	digest, err := hasher.Hash(plaintext)
	if err != nil {
		return "", fmt.Errorf("generating SHA-512 crypt hash: %w", err)
	}

	return digest.Encode(), nil
}
