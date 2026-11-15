package api

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"

	"logmonitor/internal/storage"
)

func TestCursor_RoundTrip(t *testing.T) {
	p := storage.Position{
		Timestamp: time.Date(2025, 9, 8, 11, 45, 0, 0, time.UTC),
		ID:        uuid.NewString(),
	}
	ep := encodeCursor(p)
	dp, err := decodeCursor(ep)
	if err != nil {
		t.Fatalf("Failed to decode cursor: %v", err)
	}
	if diff := cmp.Diff(p, dp); diff != "" {
		t.Errorf("round trip mismatch (-want +got):\n%s", diff)
	}
}

func TestCursor_DecodeInvalid(t *testing.T) {
	_, err := decodeCursor("not-a-cursor!")
	if err == nil {
		t.Fatalf("Expected error for invalid cursor, got nil")
	}
}
