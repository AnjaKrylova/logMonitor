package api

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"logmonitor/internal/storage"
)

func encodeCursor(p storage.Position) string {
	data, _ := json.Marshal(p)
	return base64.RawURLEncoding.EncodeToString(data)
}

func decodeCursor(s string) (storage.Position, error) {
	ds, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return storage.Position{}, fmt.Errorf("invalid cursor: %w", err)
	}
	var p storage.Position
	err = json.Unmarshal(ds, &p)
	if err != nil {
		return storage.Position{}, fmt.Errorf("invalid cursor: %w", err)
	}
	return p, nil
}
