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
	ID             uuid.UUID
	TenantID       uuid.UUID
	Channel        Channel
	Priority       Priority
	Status         Status
	RecipientID    string
	RecipientEmail string
	RecipientPhone string
	RecipientToken string
	TemplateID     *uuid.UUID // nullable — nil when no template used
	Subject        string
	Body           string
	Metadata       map[string]any
	IdempotencyKey string
	ScheduledAt    *time.Time // nullable — nil for immediate sends
	CreatedAt      time.Time
	ExpiresAt      time.Time
}

// NotificationDelivery tracks each delivery attempt for a notification.
type NotificationDelivery struct {
	ID             uuid.UUID
	NotificationID uuid.UUID
	Channel        Channel
	Status         Status
	Attempts       int
	ErrorMessage   string
	DeliveredAt    *time.Time // nil until successfully delivered
	CreatedAt      time.Time
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
