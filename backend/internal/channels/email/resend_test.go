package email

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSend_Success(t *testing.T) {
	var gotReq sendRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("Authorization header = %q, want %q", got, "Bearer test-key")
		}
		if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(sendResponse{ID: "msg_123"})
	}))
	defer srv.Close()

	origURL := sendURL
	sendURL = srv.URL
	defer func() { sendURL = origURL }()

	c := NewClient("test-key", "from@example.com")
	id, err := c.Send(context.Background(), "to@example.com", "hi", "<p>body</p>")
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if id != "msg_123" {
		t.Errorf("Send() id = %q, want %q", id, "msg_123")
	}
	if gotReq.From != "from@example.com" || gotReq.To != "to@example.com" || gotReq.Subject != "hi" || gotReq.HTML != "<p>body</p>" {
		t.Errorf("request body = %+v, unexpected fields", gotReq)
	}
}

func TestSend_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(errorResponse{Message: "invalid recipient"})
	}))
	defer srv.Close()

	origURL := sendURL
	sendURL = srv.URL
	defer func() { sendURL = origURL }()

	c := NewClient("test-key", "from@example.com")
	_, err := c.Send(context.Background(), "bad", "hi", "body")
	if err == nil {
		t.Fatal("Send() error = nil, want non-nil")
	}
}
