package mode

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/petercb/k3os-bin/internal/system"
)

// Get reads the current k3OS operational mode from the state file.
func Get(prefix ...string) (string, error) {
	bytes, err := os.ReadFile(filepath.Join(filepath.Join(prefix...), system.StatePath("mode")))
	if os.IsNotExist(err) {
		return "", nil
	} else if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(bytes)), nil
}
