package domain

import (
	"time"

	"github.com/google/uuid"
)

// Channel identifies which delivery provider handles a notification.
type Channel string

const (
	ChannelEmail Channel = "email"
	ChannelPush  Channel = "push"
	ChannelSMS   Channel = "sms"
	ChannelInApp Channel = "inapp"
)

// Status tracks where a notification is in its lifecycle.
type Status string

const (
	StatusPending           Status = "pending"
	StatusQueued            Status = "queued"
	StatusQueuedRateLimited Status = "queued_rate_limited"
	StatusDelivered         Status = "delivered"
	StatusFailed            Status = "failed"
)

// Priority controls how urgently a notification should be processed.
type Priority string

const (
	PriorityCritical Priority = "critical"
	PriorityHigh     Priority = "high"
	PriorityNormal   Priority = "normal"
	PriorityLow      Priority = "low"
)

// Notification is the central entity — one row in the notifications table.
type Notification struct {
	ID             uuid.UUID      `json:"id"`
	TenantID       uuid.UUID      `json:"tenant_id"`
	Channel        Channel        `json:"channel"`
	Priority       Priority       `json:"priority"`
	Status         Status         `json:"status"`
	RecipientID    string         `json:"recipient_id,omitempty"`
	RecipientEmail string         `json:"recipient_email,omitempty"`
	RecipientPhone string         `json:"recipient_phone,omitempty"`
	RecipientToken string         `json:"recipient_token,omitempty"`
	TemplateID     *uuid.UUID     `json:"template_id,omitempty"` // nullable — nil when no template used
	Subject        string         `json:"subject,omitempty"`
	Body           string         `json:"body"`
	Metadata       map[string]any `json:"metadata,omitempty"`
	IdempotencyKey string         `json:"idempotency_key,omitempty"`
	ScheduledAt    *time.Time     `json:"scheduled_at,omitempty"` // nullable — nil for immediate sends
	CreatedAt      time.Time      `json:"created_at"`
	ExpiresAt      time.Time      `json:"expires_at"`
}

// NotificationDelivery tracks each delivery attempt for a notification.
type NotificationDelivery struct {
	ID             uuid.UUID  `json:"id"`
	NotificationID uuid.UUID  `json:"notification_id"`
	Channel        Channel    `json:"channel"`
	Status         Status     `json:"status"`
	Attempts       int        `json:"attempts"`
	ErrorMessage   string     `json:"error_message,omitempty"`
	DeliveredAt    *time.Time `json:"delivered_at,omitempty"` // nil until successfully delivered
	CreatedAt      time.Time  `json:"created_at"`
}

// DLQMessage represents a notification that exhausted all retry attempts.
type DLQMessage struct {
	ID             uuid.UUID
	NotificationID uuid.UUID
	Channel        Channel
	Error          string
	Attempts       int
	LastTriedAt    time.Time
	CreatedAt      time.Time
}
