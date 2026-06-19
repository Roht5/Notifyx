package domain

import (
	"time"

	"github.com/google/uuid"
)

// Tenant is a customer of Notifyx — each has isolated data and its own API key.
type Tenant struct {
	ID        uuid.UUID
	Name      string
	CreatedAt time.Time
}

// TenantChannel records which delivery channels a tenant has opted into.
// A tenant can only send via channels where Enabled = true.
type TenantChannel struct {
	ID        uuid.UUID
	TenantID  uuid.UUID
	Channel   Channel
	Enabled   bool
	CreatedAt time.Time
}

// TenantRateLimit stores the per-channel and global rate caps for a tenant.
// MaxPerMin: max notifications per minute on this specific channel.
// GlobalCap: max total notifications per minute across all channels.
type TenantRateLimit struct {
	ID         uuid.UUID
	TenantID   uuid.UUID
	Channel    Channel
	MaxPerMin  int
	GlobalCap  int
	CreatedAt  time.Time
}

// APIKey holds the hashed version of a tenant's API key.
// The raw key is only shown once at creation — we store only the bcrypt hash.
type APIKey struct {
	ID        uuid.UUID
	TenantID  uuid.UUID
	KeyHash   string
	CreatedAt time.Time
}
