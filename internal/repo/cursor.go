package repo

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gofrs/uuid/v5"
)

// EncodeCursor кодирует позицию для пагинации (created_at,id) в page_token
// Если данные пустые  вернет ""
func EncodeCursor(createdAt time.Time, id string) string {
	if createdAt.IsZero() || strings.TrimSpace(id) == "" {
		return ""
	}

	raw := fmt.Sprintf("%d|%s", createdAt.UTC().UnixNano(), strings.TrimSpace(id))
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

// DecodeCursor раскодирует page_token обратно в (created_at,id)
// Если token пустой - возвращает (zero,"",nil)
func DecodeCursor(token string) (time.Time, string, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return time.Time{}, "", nil
	}

	b, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return time.Time{}, "", fmt.Errorf("invalid page_token (base64): %w", ErrInvalidInput)
	}

	parts := strings.SplitN(string(b), "|", 2)
	if len(parts) != 2 {
		return time.Time{}, "", fmt.Errorf("invalid page_token (format): %w", ErrInvalidInput)
	}

	nsStr := strings.TrimSpace(parts[0])
	idStr := strings.TrimSpace(parts[1])

	ns, err := strconv.ParseInt(nsStr, 10, 64)
	if err != nil || ns <= 0 {
		return time.Time{}, "", fmt.Errorf("invalid page_token (time): %w", ErrInvalidInput)
	}

	// UUID валидируем сразу, чтобы дальше не ловить 22P02 от Postgres
	if idStr == "" {
		return time.Time{}, "", fmt.Errorf("invalid page_token (id empty): %w", ErrInvalidInput)
	}
	if _, err := uuid.FromString(idStr); err != nil {
		return time.Time{}, "", fmt.Errorf("invalid page_token (id uuid): %w", ErrInvalidInput)
	}

	return time.Unix(0, ns).UTC(), idStr, nil
}
