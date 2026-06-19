package sms

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSend_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("authorization") != "test-key" {
			t.Errorf("authorization = %q, want %q", q.Get("authorization"), "test-key")
		}
		if q.Get("numbers") != "9999999999" {
			t.Errorf("numbers = %q, want %q", q.Get("numbers"), "9999999999")
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(sendResponse{Return: true, RequestID: "req_123"})
	}))
	defer srv.Close()

	origURL := sendURL
	sendURL = srv.URL
	defer func() { sendURL = origURL }()

	c := NewClient("test-key")
	id, err := c.Send(context.Background(), "9999999999", "hello")
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if id != "req_123" {
		t.Errorf("Send() id = %q, want %q", id, "req_123")
	}
}

func TestSend_LogicalFailure(t *testing.T) {
	// Fast2SMS returns HTTP 200 with return:false on logical failures.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(sendResponse{Return: false, Message: []string{"Invalid Number"}})
	}))
	defer srv.Close()

	origURL := sendURL
	sendURL = srv.URL
	defer func() { sendURL = origURL }()

	c := NewClient("test-key")
	_, err := c.Send(context.Background(), "123", "hello")
	if err == nil {
		t.Fatal("Send() error = nil, want non-nil")
	}
}
