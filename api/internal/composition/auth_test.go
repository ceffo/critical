package composition_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/googleadpt"
)

// ADR 0029 step 8: the root wires the Google login when it gets the OAuth
// client, else the dev login; never both.
func TestBuild_Step8_LoginModeFollowsTheOAuthSettings(t *testing.T) {
	h := newHarness(t)
	h.expectStartup(fixtureSeeds)
	bs, err := h.build(t, fixtureConfig())
	require.NoError(t, err)
	assert.True(t, bs.Auth.DevLoginEnabled(), "no OAuth client: dev login")

	h = newHarness(t)
	h.expectStartup(fixtureSeeds)
	cfg := fixtureConfig()
	cfg.Google = &googleadpt.Config{ClientID: "id", ClientSecret: "secret", RedirectURL: "https://x/api/auth/google/callback"}
	bs, err = h.build(t, cfg)
	require.NoError(t, err)
	assert.False(t, bs.Auth.DevLoginEnabled(), "OAuth client: Google login, no dev login")
}
