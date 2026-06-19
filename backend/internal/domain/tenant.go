package domain

import (
	"time"

	"github.com/google/uuid"
)

// Tenant is a customer of Notifyx — each has isolated data and its own API key.
// GlobalRateCap is the tenant-wide sliding-window cap across all channels combined —
// it lives here (not on TenantRateLimit) because it's one value per tenant, not one per
// channel.
type Tenant struct {
	ID            uuid.UUID `json:"id"`
	Name          string    `json:"name"`
	GlobalRateCap int       `json:"global_rate_cap"`
	CreatedAt     time.Time `json:"created_at"`
}

// TenantChannel records which delivery channels a tenant has opted into.
// A tenant can only send via channels where Enabled = true.
type TenantChannel struct {
	ID        uuid.UUID `json:"id"`
	TenantID  uuid.UUID `json:"tenant_id"`
	Channel   Channel   `json:"channel"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
}

// TenantRateLimit stores the per-channel rate cap for a tenant.
// MaxPerMin: max notifications per minute on this specific channel.
// The tenant-wide global cap lives on Tenant.GlobalRateCap instead — see its doc comment.
type TenantRateLimit struct {
	ID        uuid.UUID `json:"id"`
	TenantID  uuid.UUID `json:"tenant_id"`
	Channel   Channel   `json:"channel"`
	MaxPerMin int       `json:"max_per_min"`
	CreatedAt time.Time `json:"created_at"`
}

// APIKey holds the hashed version of a tenant's API key.
// The raw key is only shown once at creation — we store only the SHA-256 hash
// (deterministic, unlike bcrypt, so WHERE key_hash = ? lookups work — see apikey.go).
// KeyHash is deliberately excluded from JSON: nothing should ever return it over the API.
type APIKey struct {
	ID        uuid.UUID `json:"id"`
	TenantID  uuid.UUID `json:"tenant_id"`
	KeyHash   string    `json:"-"`
	CreatedAt time.Time `json:"created_at"`
}
