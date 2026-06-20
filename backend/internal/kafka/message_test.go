package kafka

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/rohit-bagade/notifyx/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTopicForChannel(t *testing.T) {
	cases := []struct {
		name    string
		channel domain.Channel
		want    string
	}{
		{"email", domain.ChannelEmail, TopicEmail},
		{"push", domain.ChannelPush, TopicPush},
		{"sms", domain.ChannelSMS, TopicSMS},
		{"inapp", domain.ChannelInApp, TopicInApp},
		{"unknown channel falls back to DLQ", domain.Channel("unknown"), TopicDLQ},
		{"empty channel falls back to DLQ", domain.Channel(""), TopicDLQ},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, TopicForChannel(tc.channel))
		})
	}
}

func TestTopicConstants(t *testing.T) {
	assert.Equal(t, "notifyx.email", TopicEmail)
	assert.Equal(t, "notifyx.push", TopicPush)
	assert.Equal(t, "notifyx.sms", TopicSMS)
	assert.Equal(t, "notifyx.inapp", TopicInApp)
	assert.Equal(t, "notifyx.dlq", TopicDLQ)
}

func TestMessage_JSONRoundTrip(t *testing.T) {
	notifID := uuid.New()
	delID := uuid.New()
	tenantID := uuid.New()

	msg := Message{
		NotificationID: notifID,
		DeliveryID:     delID,
		TenantID:       tenantID,
		Channel:        domain.ChannelEmail,
		Priority:       domain.PriorityHigh,
		RecipientEmail: "user@example.com",
		Subject:        "Hello",
		Body:           "World",
		Metadata:       map[string]any{"foo": "bar"},
	}

	data, err := json.Marshal(msg)
	require.NoError(t, err)

	var decoded Message
	require.NoError(t, json.Unmarshal(data, &decoded))

	assert.Equal(t, msg.NotificationID, decoded.NotificationID)
	assert.Equal(t, msg.DeliveryID, decoded.DeliveryID)
	assert.Equal(t, msg.TenantID, decoded.TenantID)
	assert.Equal(t, msg.Channel, decoded.Channel)
	assert.Equal(t, msg.Priority, decoded.Priority)
	assert.Equal(t, msg.RecipientEmail, decoded.RecipientEmail)
	assert.Equal(t, msg.Subject, decoded.Subject)
	assert.Equal(t, msg.Body, decoded.Body)
	assert.Equal(t, msg.Metadata["foo"], decoded.Metadata["foo"])
}

func TestMessage_OmitEmptyFields(t *testing.T) {
	msg := Message{
		NotificationID: uuid.New(),
		DeliveryID:     uuid.New(),
		TenantID:       uuid.New(),
		Channel:        domain.ChannelSMS,
		Priority:       domain.PriorityNormal,
		Body:           "body only",
	}

	data, err := json.Marshal(msg)
	require.NoError(t, err)

	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))

	// Optional recipient/subject/metadata/last_error fields should be omitted entirely
	// when empty, per the `omitempty` json tags.
	_, hasRecipientID := m["recipient_id"]
	_, hasRecipientEmail := m["recipient_email"]
	_, hasRecipientPhone := m["recipient_phone"]
	_, hasRecipientToken := m["recipient_token"]
	_, hasSubject := m["subject"]
	_, hasMetadata := m["metadata"]
	_, hasLastError := m["last_error"]

	assert.False(t, hasRecipientID)
	assert.False(t, hasRecipientEmail)
	assert.False(t, hasRecipientPhone)
	assert.False(t, hasRecipientToken)
	assert.False(t, hasSubject)
	assert.False(t, hasMetadata)
	assert.False(t, hasLastError)

	// Body has no omitempty, should always be present even if it were empty.
	assert.Contains(t, m, "body")
}

func TestMessage_LastErrorField(t *testing.T) {
	msg := Message{
		NotificationID: uuid.New(),
		DeliveryID:     uuid.New(),
		TenantID:       uuid.New(),
		Channel:        domain.ChannelPush,
		Priority:       domain.PriorityLow,
		Body:           "retry exhausted",
		LastError:      "provider timeout",
	}

	data, err := json.Marshal(msg)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"last_error":"provider timeout"`)
}
