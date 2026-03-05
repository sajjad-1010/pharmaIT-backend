package pagination

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestEncodeDecode(t *testing.T) {
	ts := time.Now().UTC().Truncate(time.Nanosecond)
	id := uuid.New()

	cursor := Encode(ts, id)
	decoded, err := Decode(cursor)
	if err != nil {
		t.Fatalf("Decode returned error: %v", err)
	}

	if !decoded.Timestamp.Equal(ts) {
		t.Fatalf("timestamp mismatch: got %v want %v", decoded.Timestamp, ts)
	}
	if decoded.ID != id {
		t.Fatalf("id mismatch: got %v want %v", decoded.ID, id)
	}
}

func TestDecodeInvalid(t *testing.T) {
	if _, err := Decode("not-base64"); err == nil {
		t.Fatal("expected error for invalid cursor")
	}
}

