package pagination

import (
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Cursor struct {
	Timestamp time.Time
	ID        uuid.UUID
}

type Result[T any] struct {
	Items      []T    `json:"items"`
	NextCursor *string `json:"next_cursor"`
	HasMore    bool   `json:"has_more"`
}

func Encode(ts time.Time, id uuid.UUID) string {
	raw := ts.UTC().Format(time.RFC3339Nano) + "|" + id.String()
	return base64.StdEncoding.EncodeToString([]byte(raw))
}

func Decode(cursor string) (Cursor, error) {
	raw, err := base64.StdEncoding.DecodeString(cursor)
	if err != nil {
		return Cursor{}, fmt.Errorf("invalid cursor encoding: %w", err)
	}

	parts := strings.Split(string(raw), "|")
	if len(parts) != 2 {
		return Cursor{}, fmt.Errorf("invalid cursor payload")
	}

	ts, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return Cursor{}, fmt.Errorf("invalid cursor timestamp: %w", err)
	}

	id, err := uuid.Parse(parts[1])
	if err != nil {
		return Cursor{}, fmt.Errorf("invalid cursor id: %w", err)
	}

	return Cursor{
		Timestamp: ts,
		ID:        id,
	}, nil
}

func BuildResult[T any](items []T, limit int, getCursor func(T) (time.Time, uuid.UUID)) Result[T] {
	result := Result[T]{
		Items:   items,
		HasMore: len(items) == limit,
	}

	if len(items) == limit {
		ts, id := getCursor(items[len(items)-1])
		next := Encode(ts, id)
		result.NextCursor = &next
	}

	return result
}

