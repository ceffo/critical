// Package googleadpt is Google's OpenID Connect as authsvc.IdentityProvider
// (ADR 0029 step 3.1): the authorisation-code flow with PKCE (S256), and the
// ID token checked for signature (Google's published keys), issuer, audience
// and expiry. Nothing is fetched at start-up: the signing keys are fetched on
// the first login and cached, so a Google outage stops new logins only.
package googleadpt

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"github.com/ceffo/critical/api/internal/domain/service/authsvc"
)

// Google's OpenID Connect endpoints (https://accounts.google.com/.well-known/openid-configuration).
const (
	googleIssuer   = "https://accounts.google.com"
	googleAuthURL  = "https://accounts.google.com/o/oauth2/v2/auth"
	googleTokenURL = "https://oauth2.googleapis.com/token" //nolint:gosec // a public endpoint, not a credential
	googleKeysURL  = "https://www.googleapis.com/oauth2/v3/certs"
)

// Config is the OAuth client of the app and, for tests only, other
// endpoints than Google's.
type Config struct {
	ClientID     string
	ClientSecret string
	// RedirectURL is the app's callback, <public URL>/api/auth/google/callback.
	RedirectURL string
	// Issuer, AuthURL, TokenURL and KeysURL default to Google's.
	Issuer, AuthURL, TokenURL, KeysURL string
	// HTTPClient defaults to one with a 10 s timeout.
	HTTPClient *http.Client
}

// Provider implements authsvc.IdentityProvider.
type Provider struct {
	oauth    *oauth2.Config
	verifier *oidc.IDTokenVerifier
	client   *http.Client
}

var _ authsvc.IdentityProvider = (*Provider)(nil)

// New builds the provider; it makes no request.
func New(cfg Config) (*Provider, error) {
	if cfg.ClientID == "" || cfg.ClientSecret == "" || cfg.RedirectURL == "" {
		return nil, errors.New("googleadpt: client id, client secret and redirect URL are required")
	}
	issuer, keys := or(cfg.Issuer, googleIssuer), or(cfg.KeysURL, googleKeysURL)
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	// The key set fetches with the client of the context it was built with.
	keyCtx := oidc.ClientContext(context.Background(), client)
	return &Provider{
		oauth: &oauth2.Config{
			ClientID: cfg.ClientID, ClientSecret: cfg.ClientSecret, RedirectURL: cfg.RedirectURL,
			Endpoint: oauth2.Endpoint{
				AuthURL: or(cfg.AuthURL, googleAuthURL), TokenURL: or(cfg.TokenURL, googleTokenURL),
				AuthStyle: oauth2.AuthStyleInParams,
			},
			Scopes: []string{oidc.ScopeOpenID, "email"},
		},
		verifier: oidc.NewVerifier(issuer, oidc.NewRemoteKeySet(keyCtx, keys), &oidc.Config{ClientID: cfg.ClientID}),
		client:   client,
	}, nil
}

func or(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

// AuthURL implements authsvc.IdentityProvider: the state and the S256
// challenge of verifier; Google shows its account chooser.
func (p *Provider) AuthURL(state, verifier string) string {
	return p.oauth.AuthCodeURL(state, oauth2.S256ChallengeOption(verifier),
		oauth2.SetAuthURLParam("prompt", "select_account"))
}

// Exchange implements authsvc.IdentityProvider: the code and the PKCE
// verifier for an ID token, verified (signature, issuer, audience, expiry).
// The token is used once and not kept.
func (p *Provider) Exchange(ctx context.Context, code, verifier string) (*authsvc.Identity, error) {
	ctx = context.WithValue(ctx, oauth2.HTTPClient, p.client)
	tok, err := p.oauth.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		return nil, fmt.Errorf("code exchange: %w", err)
	}
	raw, ok := tok.Extra("id_token").(string)
	if !ok || raw == "" {
		return nil, errors.New("code exchange: no id_token in the response")
	}
	idt, err := p.verifier.Verify(ctx, raw)
	if err != nil {
		return nil, fmt.Errorf("id token: %w", err)
	}
	var claims struct {
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
	}
	if err := idt.Claims(&claims); err != nil {
		return nil, fmt.Errorf("id token claims: %w", err)
	}
	return &authsvc.Identity{Subject: idt.Subject, Email: claims.Email, EmailVerified: claims.EmailVerified}, nil
}
