package push

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// writeTestCredentials generates a throwaway RSA key, creates a service-account-shaped
// JSON payload pointed at the given token endpoint, and returns it as a string.
func writeTestCredentials(t *testing.T, tokenURI string) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	pemKey := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})

	sa := serviceAccount{
		ProjectID:   "test-project",
		PrivateKey:  string(pemKey),
		ClientEmail: "test@test-project.iam.gserviceaccount.com",
		TokenURI:    tokenURI,
	}
	raw, err := json.Marshal(sa)
	if err != nil {
		t.Fatalf("marshal service account: %v", err)
	}

	return string(raw)
}

func TestSend_Success(t *testing.T) {
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse token request: %v", err)
		}
		if got := r.Form.Get("grant_type"); got != "urn:ietf:params:oauth:grant-type:jwt-bearer" {
			t.Errorf("grant_type = %q", got)
		}
		assertion := r.Form.Get("assertion")
		if parts := strings.Split(assertion, "."); len(parts) != 3 {
			t.Errorf("assertion is not a 3-part JWT: %q", assertion)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"access_token": "fake-access-token", "expires_in": 3600})
	}))
	defer tokenSrv.Close()

	credsJSON := writeTestCredentials(t, tokenSrv.URL)

	fcmSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer fake-access-token" {
			t.Errorf("Authorization = %q", got)
		}
		var msg fcmMessage
		if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
			t.Fatalf("decode fcm body: %v", err)
		}
		if msg.Message.Token != "device-token" {
			t.Errorf("token = %q, want %q", msg.Message.Token, "device-token")
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"name": "projects/test-project/messages/abc"})
	}))
	defer fcmSrv.Close()

	origBase := fcmBaseURL
	fcmBaseURL = fcmSrv.URL
	defer func() { fcmBaseURL = origBase }()

	c, err := NewClient(credsJSON)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	name, err := c.Send(context.Background(), "device-token", "title", "body")
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if name != "projects/test-project/messages/abc" {
		t.Errorf("Send() name = %q", name)
	}
}

func TestNewClient_MissingFields(t *testing.T) {
	if _, err := NewClient(`{"project_id":"x"}`); err == nil {
		t.Fatal("NewClient() error = nil, want non-nil for missing private_key/client_email")
	}
}
