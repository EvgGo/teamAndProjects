package repo

import (
	"fmt"
	"strings"

	"github.com/gofrs/uuid/v5"
)

// parseUUID валидирует UUID-строку и возвращает тип uuid.UUID (gofrs)
// Возвращает ErrInvalidInput, если строка пустая/битая
func parseUUID(s string) (uuid.UUID, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return uuid.Nil, fmt.Errorf("uuid is empty: %w", ErrInvalidInput)
	}

	id, err := uuid.FromString(s)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid uuid %q: %w", s, ErrInvalidInput)
	}

	return id, nil
}
