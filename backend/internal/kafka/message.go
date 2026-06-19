package kafka

import (
	"github.com/google/uuid"
	"github.com/rohit-bagade/notifyx/internal/domain"
)

// Topic names — must match the topics created in Confluent Cloud.
const (
	TopicEmail = "notifyx.email"
	TopicPush  = "notifyx.push"
	TopicSMS   = "notifyx.sms"
	TopicInApp = "notifyx.inapp"
	TopicDLQ   = "notifyx.dlq"
)

// TopicForChannel returns the Kafka topic that handles the given channel.
func TopicForChannel(ch domain.Channel) string {
	switch ch {
	case domain.ChannelEmail:
		return TopicEmail
	case domain.ChannelPush:
		return TopicPush
	case domain.ChannelSMS:
		return TopicSMS
	case domain.ChannelInApp:
		return TopicInApp
	default:
		return TopicDLQ
	}
}

// Message is the JSON payload published to every per-channel Kafka topic.
// Consumers decode this to perform delivery and update DB status.
type Message struct {
	NotificationID uuid.UUID       `json:"notification_id"`
	DeliveryID     uuid.UUID       `json:"delivery_id"`
	TenantID       uuid.UUID       `json:"tenant_id"`
	Channel        domain.Channel  `json:"channel"`
	Priority       domain.Priority `json:"priority"`
	RecipientID    string          `json:"recipient_id,omitempty"`
	RecipientEmail string          `json:"recipient_email,omitempty"`
	RecipientPhone string          `json:"recipient_phone,omitempty"`
	RecipientToken string          `json:"recipient_token,omitempty"`
	Subject        string          `json:"subject,omitempty"`
	Body           string          `json:"body"`
	Metadata       map[string]any  `json:"metadata,omitempty"`
	// LastError carries the final handler error when a message exhausts retries and
	// is republished to the DLQ — lets the DLQ consumer record the real root cause
	// instead of a generic "exhausted delivery retries" string.
	LastError string `json:"last_error,omitempty"`
}
