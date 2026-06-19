package domain

import (
	"time"

	"github.com/google/uuid"
)

// NotificationTemplate is a reusable message body (and optional subject for email).
// Body supports {{variable}} placeholders that are substituted at send time.
type NotificationTemplate struct {
	ID        uuid.UUID `json:"id"`
	TenantID  uuid.UUID `json:"tenant_id"`
	Name      string    `json:"name"`
	Channel   Channel   `json:"channel"`
	Subject   string    `json:"subject,omitempty"` // only used for email; empty for other channels
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
