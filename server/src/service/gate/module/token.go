package module

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

func newID(prefix string, bytes int) (string, error) {
	buf := make([]byte, bytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate random bytes: %w", err)
	}
	return prefix + "_" + hex.EncodeToString(buf), nil
}
