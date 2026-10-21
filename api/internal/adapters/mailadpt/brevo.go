package mailadpt

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// BrevoEndpoint is Brevo's transactional email API.
const BrevoEndpoint = "https://api.brevo.com/v3/smtp/email"

// BrevoConfig is the Brevo account's settings. Endpoint and HTTPClient exist
// for tests.
type BrevoConfig struct {
	// APIKey is sent in the api-key header; never logged.
	APIKey string
	// From is the verified sender address, FromName its display name
	// (optional).
	From, FromName string
	Endpoint       string
	HTTPClient     *http.Client
}

// Brevo sends through Brevo's HTTPS API.
type Brevo struct {
	cfg    BrevoConfig
	client *http.Client
}

var _ Mailer = (*Brevo)(nil)

// ErrBrevo is any refusal or failure of the Brevo call.
var ErrBrevo = errors.New("brevo")

// NewBrevo builds the Brevo mailer; it makes no request.
func NewBrevo(cfg BrevoConfig) (*Brevo, error) {
	if cfg.APIKey == "" || cfg.From == "" {
		return nil, errors.New("mailadpt: the Brevo API key and the sender address are required")
	}
	if cfg.Endpoint == "" {
		cfg.Endpoint = BrevoEndpoint
	}
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &Brevo{cfg: cfg, client: client}, nil
}

type brevoAddress struct {
	Name  string `json:"name,omitempty"`
	Email string `json:"email"`
}

type brevoRequest struct {
	Sender      brevoAddress   `json:"sender"`
	To          []brevoAddress `json:"to"`
	Subject     string         `json:"subject"`
	TextContent string         `json:"textContent"`
	HTMLContent string         `json:"htmlContent,omitempty"`
}

// Send implements Mailer. A 2xx is sent; any other status is an ErrBrevo
// carrying the status and Brevo's own message, never the key.
func (b *Brevo) Send(ctx context.Context, m Message) error {
	body, err := json.Marshal(brevoRequest{
		Sender: brevoAddress{Name: b.cfg.FromName, Email: b.cfg.From}, To: []brevoAddress{{Email: m.To}},
		Subject: m.Subject, TextContent: m.TextBody, HTMLContent: m.HTMLBody,
	})
	if err != nil {
		return fmt.Errorf("%w: %w", ErrBrevo, err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.cfg.Endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("%w: %w", ErrBrevo, err)
	}
	req.Header.Set("api-key", b.cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := b.client.Do(req)
	if err != nil {
		// The transport error names the URL, not the key.
		return fmt.Errorf("%w: %w", ErrBrevo, err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode/100 == 2 {
		return nil
	}
	var e struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	_ = json.Unmarshal(raw, &e)
	return fmt.Errorf("%w: status %d %s %s", ErrBrevo, resp.StatusCode, e.Code, e.Message)
}
