// The adapter runs against a fake Google on httptest: a token endpoint that
// returns an ID token signed by a test key, and the key set publishing it.
// Nothing reaches the network.
package googleadpt_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/googleadpt"
	"github.com/ceffo/critical/api/internal/domain/service/authsvc"
)

const clientID = "client-1"

type fakeGoogle struct {
	srv      *httptest.Server
	key      *rsa.PrivateKey
	claims   map[string]any
	verifier string // the code_verifier the token endpoint received
}

func newFakeGoogle(t *testing.T) *fakeGoogle {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	g := &fakeGoogle{key: key}
	mux := http.NewServeMux()
	mux.HandleFunc("/keys", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{
			{Key: &key.PublicKey, KeyID: "k1", Algorithm: "RS256", Use: "sig"},
		}})
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		g.verifier = r.PostForm.Get("code_verifier")
		if r.PostForm.Get("code") != "good-code" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "at", "token_type": "Bearer", "expires_in": 3600, "id_token": g.sign(t),
		})
	})
	g.srv = httptest.NewServer(mux)
	t.Cleanup(g.srv.Close)
	g.claims = map[string]any{
		"iss": g.srv.URL, "aud": clientID, "sub": "g-123", "email": "ann@example.com", "email_verified": true,
		"iat": time.Now().Unix(), "exp": time.Now().Add(time.Hour).Unix(),
	}
	return g
}

func (g *fakeGoogle) sign(t *testing.T) string {
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: g.key},
		(&jose.SignerOptions{}).WithHeader("kid", "k1").WithType("JWT"))
	require.NoError(t, err)
	payload, err := json.Marshal(g.claims)
	require.NoError(t, err)
	jws, err := signer.Sign(payload)
	require.NoError(t, err)
	out, err := jws.CompactSerialize()
	require.NoError(t, err)
	return out
}

func (g *fakeGoogle) provider(t *testing.T) *googleadpt.Provider {
	t.Helper()
	p, err := googleadpt.New(googleadpt.Config{
		ClientID: clientID, ClientSecret: "secret", RedirectURL: "https://app.example/api/auth/google/callback",
		Issuer: g.srv.URL, AuthURL: g.srv.URL + "/auth", TokenURL: g.srv.URL + "/token", KeysURL: g.srv.URL + "/keys",
		HTTPClient: g.srv.Client(),
	})
	require.NoError(t, err)
	return p
}

// ADR 0029 step 3.1: the authorisation URL carries the state, the client, the
// redirect, and the S256 challenge of the verifier (never the verifier).
func TestAuthURL_StateAndS256Challenge(t *testing.T) {
	g := newFakeGoogle(t)
	u, err := url.Parse(g.provider(t).AuthURL("the-state", "the-verifier"))
	require.NoError(t, err)
	q := u.Query()
	sum := sha256.Sum256([]byte("the-verifier"))
	assert.Equal(t, "the-state", q.Get("state"))
	assert.Equal(t, base64.RawURLEncoding.EncodeToString(sum[:]), q.Get("code_challenge"))
	assert.Equal(t, "S256", q.Get("code_challenge_method"))
	assert.Equal(t, clientID, q.Get("client_id"))
	assert.Equal(t, "https://app.example/api/auth/google/callback", q.Get("redirect_uri"))
	assert.Equal(t, "code", q.Get("response_type"))
	assert.Contains(t, q.Get("scope"), "openid")
	assert.NotContains(t, u.String(), "the-verifier")
}

// Step 3.1: the code is exchanged with the PKCE verifier, and a valid ID
// token gives its subject and email (step 3.2: with email_verified).
func TestExchange_ValidTokenIsItsIdentity(t *testing.T) {
	g := newFakeGoogle(t)
	id, err := g.provider(t).Exchange(t.Context(), "good-code", "the-verifier")
	require.NoError(t, err)
	assert.Equal(t, authsvc.Identity{Subject: "g-123", Email: "ann@example.com", EmailVerified: true}, *id)
	assert.Equal(t, "the-verifier", g.verifier)
}

// Step 3.1: a token for another audience, from another issuer, expired, or
// signed by another key is refused; so is a refused code.
func TestExchange_RefusesABadToken(t *testing.T) {
	cases := map[string]func(g *fakeGoogle){
		"other audience": func(g *fakeGoogle) { g.claims["aud"] = "someone-else" },
		"other issuer":   func(g *fakeGoogle) { g.claims["iss"] = "https://evil.example" },
		"expired":        func(g *fakeGoogle) { g.claims["exp"] = time.Now().Add(-time.Hour).Unix() },
		"other key": func(g *fakeGoogle) {
			k, _ := rsa.GenerateKey(rand.Reader, 2048)
			g.key = k
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			g := newFakeGoogle(t)
			p := g.provider(t)
			mutate(g)
			_, err := p.Exchange(t.Context(), "good-code", "v")
			require.Error(t, err)
		})
	}
	g := newFakeGoogle(t)
	_, err := g.provider(t).Exchange(t.Context(), "bad-code", "v")
	require.Error(t, err)
}

// An incomplete client configuration is refused.
func TestNew_RequiresTheClient(t *testing.T) {
	_, err := googleadpt.New(googleadpt.Config{ClientID: "a", ClientSecret: "b"})
	require.Error(t, err)
}
