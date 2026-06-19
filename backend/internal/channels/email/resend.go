// Package email wraps the Resend REST API. No official Resend Go SDK is used —
// the API is a single simple JSON POST, not worth a dependency for.
package email

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// sendURL is a var (not const) so tests can point it at an httptest server.
var sendURL = "https://api.resend.com/emails"

// requestTimeout bounds a single Resend call so a hung provider can't stall the
// consumer's retry loop indefinitely — handleWithRetry's own backoff handles spacing.
const requestTimeout = 10 * time.Second

type Client struct {
	apiKey    string
	fromEmail string
	http      *http.Client
}

func NewClient(apiKey, fromEmail string) *Client {
	return &Client{
		apiKey:    apiKey,
		fromEmail: fromEmail,
		http:      &http.Client{Timeout: requestTimeout},
	}
}

type sendRequest struct {
	From    string `json:"from"`
	To      string `json:"to"`
	Subject string `json:"subject"`
	HTML    string `json:"html"`
}

type sendResponse struct {
	ID string `json:"id"`
}

type errorResponse struct {
	Message string `json:"message"`
}

// Send delivers one email and returns Resend's message ID on success.
func (c *Client) Send(ctx context.Context, to, subject, body string) (string, error) {
	payload, err := json.Marshal(sendRequest{From: c.fromEmail, To: to, Subject: subject, HTML: body})
	if err != nil {
		return "", fmt.Errorf("marshal resend request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sendURL, bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("build resend request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("resend request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read resend response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var apiErr errorResponse
		if jsonErr := json.Unmarshal(respBody, &apiErr); jsonErr == nil && apiErr.Message != "" {
			return "", fmt.Errorf("resend error (%d): %s", resp.StatusCode, apiErr.Message)
		}
		return "", fmt.Errorf("resend error (%d): %s", resp.StatusCode, string(respBody))
	}

	var sr sendResponse
	if err := json.Unmarshal(respBody, &sr); err != nil {
		return "", fmt.Errorf("unmarshal resend response: %w", err)
	}
	return sr.ID, nil
}
