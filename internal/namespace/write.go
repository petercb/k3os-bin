//go:build linux

package namespace

import (
	"fmt"
	"os"
)

// Write creates a file with the given content.
type Write struct {
	Path    string
	Content string
}

// Create calls os.WriteFile to write the content to the path.
func (w Write) Create() error {
	if err := os.WriteFile(w.Path, []byte(w.Content), 0o600); err != nil {
		return fmt.Errorf("write %s: %w", w.Path, err)
	}
	return nil
}

func (w Write) String() string {
	return fmt.Sprintf("write{%s}", w.Path)
}
