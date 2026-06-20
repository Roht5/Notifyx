// Package push wraps the Firebase Cloud Messaging HTTP v1 API. Rather than pulling in
// the firebase.google.com/go/v4 Admin SDK (which drags in the full Google Cloud client
// stack for a single REST call), this implements the service-account JWT bearer flow
// directly against stdlib crypto/x509 + net/http, then calls FCM's send endpoint.
package push

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"
)

const (
	tokenURL        = "https://oauth2.googleapis.com/token"
	fcmScope        = "https://www.googleapis.com/auth/firebase.messaging"
	requestTimeout  = 10 * time.Second
	tokenExpirySkew = 60 * time.Second // refresh slightly before the token actually expires
)

// fcmBaseURL is a var (not const) so tests can point it at an httptest server.
var fcmBaseURL = "https://fcm.googleapis.com"

// serviceAccount mirrors the fields Google's service-account JSON key file provides.
type serviceAccount struct {
	ProjectID   string `json:"project_id"`
	PrivateKey  string `json:"private_key"`
	ClientEmail string `json:"client_email"`
	TokenURI    string `json:"token_uri"`
}

// Sender is the seam used by the push consumer so it can be unit-tested against a
// mock instead of a real FCM HTTP call.
type Sender interface {
	Send(ctx context.Context, token, title, body string) (string, error)
}

type Client struct {
	projectID   string
	clientEmail string
	privateKey  *rsa.PrivateKey
	tokenURI    string
	http        *http.Client

	mu          sync.Mutex
	accessToken string
	expiresAt   time.Time
}

var _ Sender = (*Client)(nil)

// NewClient parses credentialsJSON — the raw Firebase service-account JSON content
// (e.g. from the FIREBASE_CREDENTIALS_JSON env var), not a file path. Never wrap parse
// errors with the input itself: a malformed/path-like value here is very likely the
// secret itself, and embedding it in an error gets it written straight to logs.
func NewClient(credentialsJSON string) (*Client, error) {
	var sa serviceAccount
	if err := json.Unmarshal([]byte(credentialsJSON), &sa); err != nil {
		return nil, fmt.Errorf("parse firebase credentials JSON: %w", err)
	}
	if sa.ProjectID == "" || sa.PrivateKey == "" || sa.ClientEmail == "" {
		return nil, fmt.Errorf("firebase credentials JSON missing project_id/private_key/client_email")
	}

	key, err := parsePrivateKey(sa.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("parse firebase private key: %w", err)
	}

	tokenURI := sa.TokenURI
	if tokenURI == "" {
		tokenURI = tokenURL
	}

	return &Client{
		projectID:   sa.ProjectID,
		clientEmail: sa.ClientEmail,
		privateKey:  key,
		tokenURI:    tokenURI,
		http:        &http.Client{Timeout: requestTimeout},
	}, nil
}

func parsePrivateKey(pemKey string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemKey))
	if block == nil {
		return nil, fmt.Errorf("invalid PEM block in private key")
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse PKCS8 private key: %w", err)
	}
	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("private key is not RSA")
	}
	return rsaKey, nil
}

type fcmMessage struct {
	Message struct {
		Token        string            `json:"token"`
		Notification map[string]string `json:"notification"`
	} `json:"message"`
}

type fcmErrorResponse struct {
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Send delivers one push notification via FCM and returns the provider message name
// (e.g. "projects/x/messages/y") on success.
func (c *Client) Send(ctx context.Context, token, title, body string) (string, error) {
	accessToken, err := c.getAccessToken(ctx)
	if err != nil {
		return "", fmt.Errorf("get fcm access token: %w", err)
	}

	var msg fcmMessage
	msg.Message.Token = token
	msg.Message.Notification = map[string]string{"title": title, "body": body}
	payload, err := json.Marshal(msg)
	if err != nil {
		return "", fmt.Errorf("marshal fcm message: %w", err)
	}

	sendURL := fmt.Sprintf("%s/v1/projects/%s/messages:send", fcmBaseURL, c.projectID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sendURL, bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("build fcm request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("fcm request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read fcm response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var apiErr fcmErrorResponse
		if jsonErr := json.Unmarshal(respBody, &apiErr); jsonErr == nil && apiErr.Error.Message != "" {
			return "", fmt.Errorf("fcm error (%d): %s", resp.StatusCode, apiErr.Error.Message)
		}
		return "", fmt.Errorf("fcm error (%d): %s", resp.StatusCode, string(respBody))
	}

	var sr struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(respBody, &sr); err != nil {
		return "", fmt.Errorf("unmarshal fcm response: %w", err)
	}
	return sr.Name, nil
}

// getAccessToken returns a cached OAuth2 access token, refreshing it via the
// service-account JWT bearer flow (RFC 7523) once it's within tokenExpirySkew of expiry.
func (c *Client) getAccessToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.accessToken != "" && time.Now().Before(c.expiresAt.Add(-tokenExpirySkew)) {
		return c.accessToken, nil
	}

	assertion, err := c.signJWT()
	if err != nil {
		return "", fmt.Errorf("sign jwt assertion: %w", err)
	}

	form := url.Values{}
	form.Set("grant_type", "urn:ietf:params:oauth:grant-type:jwt-bearer")
	form.Set("assertion", assertion)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.tokenURI, bytesReaderForForm(form))
	if err != nil {
		return "", fmt.Errorf("build token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read token response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("token endpoint error (%d): %s", resp.StatusCode, string(respBody))
	}

	var tr struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(respBody, &tr); err != nil {
		return "", fmt.Errorf("unmarshal token response: %w", err)
	}

	c.accessToken = tr.AccessToken
	c.expiresAt = time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second)
	return c.accessToken, nil
}

// signJWT builds and signs a service-account JWT assertion per Google's JWT bearer flow.
func (c *Client) signJWT() (string, error) {
	now := time.Now()
	header := map[string]string{"alg": "RS256", "typ": "JWT"}
	claims := map[string]any{
		"iss":   c.clientEmail,
		"scope": fcmScope,
		"aud":   c.tokenURI,
		"iat":   now.Unix(),
		"exp":   now.Add(time.Hour).Unix(),
	}

	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}

	signingInput := base64URLEncode(headerJSON) + "." + base64URLEncode(claimsJSON)

	sig, err := signRS256(c.privateKey, signingInput)
	if err != nil {
		return "", err
	}

	return signingInput + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

func base64URLEncode(b []byte) string {
	return base64.RawURLEncoding.EncodeToString(b)
}

func bytesReaderForForm(v url.Values) *bytes.Reader {
	return bytes.NewReader([]byte(v.Encode()))
}

func signRS256(key *rsa.PrivateKey, input string) ([]byte, error) {
	hashed := sha256.Sum256([]byte(input))
	return rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, hashed[:])
}
