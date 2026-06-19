package handlers

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// generateAPIKey creates a cryptographically random API key and returns the
// raw key (shown to the caller once) and its SHA-256 hex hash (stored in DB).
//
// Design note: we use SHA-256 (not bcrypt) because API key lookup requires a
// deterministic hash — bcrypt is non-deterministic due to per-call salting,
// making WHERE key_hash = ? impossible. SHA-256 of a 32-byte random key is
// unguessable without rainbow tables (the key space is 2^256).
func generateAPIKey() (rawKey string, keyHash string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", fmt.Errorf("generate random bytes: %w", err)
	}
	raw := "nfx_" + hex.EncodeToString(b)
	hash := sha256.Sum256([]byte(raw))
	return raw, hex.EncodeToString(hash[:]), nil
}

// HashAPIKey returns the SHA-256 hex hash of an API key.
// Used by the auth middleware to hash the incoming key before DB lookup.
func HashAPIKey(key string) string {
	hash := sha256.Sum256([]byte(key))
	return hex.EncodeToString(hash[:])
}
