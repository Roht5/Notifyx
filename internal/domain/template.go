package domain

import (
	"time"

	"github.com/google/uuid"
)

// NotificationTemplate is a reusable message body (and optional subject for email).
// Body supports {{variable}} placeholders that are substituted at send time.
type NotificationTemplate struct {
	ID        uuid.UUID
	TenantID  uuid.UUID
	Name      string
	Channel   Channel
	Subject   string // only used for email; empty for other channels
	Body      string
	CreatedAt time.Time
}
