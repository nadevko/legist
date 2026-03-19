package api

import (
	"strings"

	"github.com/google/uuid"
)

// newID генерирует короткий ID с префиксом типа объекта.
// Пример: file_a1b2c3d4, user_e5f6g7h8
func newID(prefix string) string {
	raw := strings.ReplaceAll(uuid.NewString(), "-", "")
	return prefix + "_" + raw[:12]
}
