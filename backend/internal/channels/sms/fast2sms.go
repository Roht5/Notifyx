// Package sms wraps the Fast2SMS "quick" SMS REST route. No SDK exists for Go —
// the API is a single GET with query params.
package sms

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// sendURL is a var (not const) so tests can point it at an httptest server.
var sendURL = "https://www.fast2sms.com/dev/bulkV2"

const requestTimeout = 10 * time.Second

// Sender is the seam used by the sms consumer so it can be unit-tested against a
// mock instead of a real Fast2SMS HTTP call.
type Sender interface {
	Send(ctx context.Context, phone, message string) (string, error)
}

type Client struct {
	apiKey string
	http   *http.Client
}

var _ Sender = (*Client)(nil)

func NewClient(apiKey string) *Client {
	return &Client{
		apiKey: apiKey,
		http:   &http.Client{Timeout: requestTimeout},
	}
}

type sendResponse struct {
	Return    bool     `json:"return"`
	RequestID string   `json:"request_id"`
	Message   []string `json:"message"`
}

// Send delivers one SMS via Fast2SMS's "q" (quick transactional) route and returns
// the provider's request ID on success.
func (c *Client) Send(ctx context.Context, phone, message string) (string, error) {
	q := url.Values{}
	q.Set("authorization", c.apiKey)
	q.Set("message", message)
	q.Set("language", "english")
	q.Set("route", "q")
	q.Set("numbers", phone)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sendURL+"?"+q.Encode(), nil)
	if err != nil {
		return "", fmt.Errorf("build fast2sms request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("fast2sms request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read fast2sms response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("fast2sms error (%d): %s", resp.StatusCode, string(respBody))
	}

	var sr sendResponse
	if err := json.Unmarshal(respBody, &sr); err != nil {
		return "", fmt.Errorf("unmarshal fast2sms response: %w", err)
	}
	if !sr.Return {
		// Fast2SMS returns HTTP 200 even on logical failures (bad number, no balance) —
		// the actual outcome is in the "return" field, not the status code.
		reason := "unknown error"
		if len(sr.Message) > 0 {
			reason = sr.Message[0]
		}
		return "", fmt.Errorf("fast2sms rejected message: %s", reason)
	}
	return sr.RequestID, nil
}
