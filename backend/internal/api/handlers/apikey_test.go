package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateAPIKey(t *testing.T) {
	raw, hash, err := generateAPIKey()

	assert.NoError(t, err)
	assert.Contains(t, raw, "nfx_")
	assert.Len(t, hash, 64) // sha256 hex digest length
	assert.Equal(t, HashAPIKey(raw), hash)
}

func TestGenerateAPIKey_Unique(t *testing.T) {
	raw1, hash1, err1 := generateAPIKey()
	raw2, hash2, err2 := generateAPIKey()

	assert.NoError(t, err1)
	assert.NoError(t, err2)
	assert.NotEqual(t, raw1, raw2)
	assert.NotEqual(t, hash1, hash2)
}

func TestHashAPIKey_Deterministic(t *testing.T) {
	key := "nfx_somekeyvalue"

	h1 := HashAPIKey(key)
	h2 := HashAPIKey(key)

	assert.Equal(t, h1, h2)
	assert.Len(t, h1, 64)
}

func TestHashAPIKey_DifferentInputsDifferentHashes(t *testing.T) {
	h1 := HashAPIKey("key-one")
	h2 := HashAPIKey("key-two")

	assert.NotEqual(t, h1, h2)
}
