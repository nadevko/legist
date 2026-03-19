package api

import (
	"strings"

	"github.com/google/uuid"
)

// newID generates a short prefixed ID.
// Example: file_a1b2c3d4e5f6, doc_a1b2c3d4e5f6
func newID(prefix string) string {
	raw := strings.ReplaceAll(uuid.NewString(), "-", "")
	return prefix + "_" + raw[:12]
}
