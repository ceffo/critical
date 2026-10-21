package composition_test

// ADR 0029 steps 1.3, 2, 4.4 and 5.2 end to end over the in-memory storage:
// the admin section through the router and the real services.

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/mailadpt"
	"github.com/ceffo/critical/api/internal/composition"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/rest/router"
	"github.com/ceffo/critical/api/internal/storage/memory"
)

type adminUsersView struct {
	Users []struct {
		ID         string   `json:"id"`
		Email      string   `json:"email"`
		Role       string   `json:"role"`
		Root       bool     `json:"root"`
		Disabled   bool     `json:"disabled"`
		LastLogin  *string  `json:"lastLogin"`
		Characters []string `json:"characters"`
	} `json:"users"`
}

type invitesView struct {
	Invites []struct {
		ID    string `json:"id"`
		Email string `json:"email"`
	} `json:"invites"`
}

type codeView struct {
	Code string `json:"code"`
}

// An admin invites an email, sees it pending, cannot invite it twice nor an
// email that has a user, cancels it; then the email can be invited again.
func TestAdmin_Step2_InviteListCancel(t *testing.T) {
	tb, _ := newTable(t)
	root, _ := tb.login("")
	tb.login("bob@example.com")

	var inv struct {
		ID    string `json:"id"`
		Email string `json:"email"`
	}
	require.Equal(t, http.StatusCreated, tb.call(root, "POST", "/api/admin/invites", `{"email":"Cid@Example.com"}`, &inv))
	assert.Equal(t, "cid@example.com", inv.Email)
	var pending invitesView
	require.Equal(t, http.StatusOK, tb.call(root, "GET", "/api/admin/invites", "", &pending))
	require.Len(t, pending.Invites, 1)
	assert.Equal(t, inv.ID, pending.Invites[0].ID)

	var refused codeView
	assert.Equal(t, http.StatusConflict, tb.call(root, "POST", "/api/admin/invites", `{"email":"cid@example.com"}`, &refused))
	assert.Equal(t, "already_invited", refused.Code)
	assert.Equal(t, http.StatusConflict, tb.call(root, "POST", "/api/admin/invites", `{"email":"bob@example.com"}`, &refused))
	assert.Equal(t, "email_has_user", refused.Code)
	assert.Equal(t, http.StatusBadRequest, tb.call(root, "POST", "/api/admin/invites", `{"email":"cid"}`, &refused))
	assert.Equal(t, "invalid_input", refused.Code)

	require.Equal(t, http.StatusOK, tb.call(root, "DELETE", "/api/admin/invites/"+inv.ID, "", &pending))
	assert.Empty(t, pending.Invites)
	assert.Equal(t, http.StatusNotFound, tb.call(root, "DELETE", "/api/admin/invites/"+inv.ID, "", &refused))
	assert.Equal(t, "invite_not_found", refused.Code)
	require.Equal(t, http.StatusCreated, tb.call(root, "POST", "/api/admin/invites", `{"email":"cid@example.com"}`, nil),
		"a cancelled invite does not block a new one")
}

// Steps 1.3, 4.4 and 5.2: the admin list shows each user's characters by
// name; a player gets 403 on it; disabling a user logs them out at once and
// re-enabling lets them log in again; the root admin is neither disabled nor
// demoted.
func TestAdmin_Step4_4_DisablingLogsTheUserOut(t *testing.T) {
	tb, m := newTable(t)
	root, rootID := tb.login("")
	bob, bobID := tb.login("bob@example.com")
	_, err := m.AddCharacterOf(context.Background(), bobID, charactersvc.Character{
		Name: "Bram", Class: "fighter", Str: 16, Dex: 12, Con: 14, Int: 9, Wis: 10, Cha: 8, HPMax: 30,
	})
	require.NoError(t, err)

	// The dev login makes admins: Bob becomes a player first.
	require.Equal(t, http.StatusOK, tb.call(root, "PATCH", "/api/admin/users/"+bobID.String(), `{"role":"player"}`, nil))
	var refused codeView
	assert.Equal(t, http.StatusForbidden, tb.call(bob, "GET", "/api/admin/users", "", &refused), "at once on this server")
	assert.Equal(t, "admin_only", refused.Code)

	var list adminUsersView
	require.Equal(t, http.StatusOK, tb.call(root, "GET", "/api/admin/users", "", &list))
	require.Len(t, list.Users, 2)
	assert.Equal(t, rootID.String(), list.Users[0].ID)
	assert.True(t, list.Users[0].Root)
	assert.Equal(t, []string{"Test Bard"}, list.Users[0].Characters)
	assert.Equal(t, "player", list.Users[1].Role)
	assert.Equal(t, []string{"Bram"}, list.Users[1].Characters)
	assert.NotNil(t, list.Users[1].LastLogin)

	require.Equal(t, http.StatusOK, tb.call(bob, "GET", "/api/characters", "", nil))
	require.Equal(t, http.StatusOK, tb.call(root, "PATCH", "/api/admin/users/"+bobID.String(), `{"disabled":true}`, nil))
	assert.Equal(t, http.StatusUnauthorized, tb.call(bob, "GET", "/api/characters", "", nil), "logged out at once")
	assert.Equal(t, http.StatusForbidden, tb.dev("bob@example.com"), "a disabled user cannot log in")

	require.Equal(t, http.StatusOK, tb.call(root, "PATCH", "/api/admin/users/"+bobID.String(), `{"disabled":false}`, nil))
	assert.Equal(t, http.StatusOK, tb.dev("bob@example.com"), "re-enabled")

	for _, body := range []string{`{"disabled":true}`, `{"role":"player"}`} {
		assert.Equal(t, http.StatusConflict, tb.call(root, "PATCH", "/api/admin/users/"+rootID.String(), body, &refused), body)
		assert.Equal(t, "root_admin_protected", refused.Code)
	}
}

// dev is the status of a dev login of email.
func (tb table) dev(email string) int {
	tb.t.Helper()
	return tb.call("", "POST", "/api/auth/dev", `{"email":"`+email+`"}`, nil)
}

// ADR 0029 steps 2.5, 2.7 and 2.8 end to end over the in-memory storage and
// the real notifier on a recorder: creating an invite mails the invitee in the
// chosen language, a resend mails again, a cancelled invite is refused, and a
// failing mail leaves the invite created with the outcome failed.
func TestAdmin_Step2_InviteMail(t *testing.T) {
	rec := &mailadpt.Recorder{}
	notifier, err := mailadpt.New(rec, "https://critical-hit.fly.dev")
	require.NoError(t, err)
	cfg := fixtureConfig()
	cfg.Mail = notifier
	ctx := context.Background()
	bs, err := composition.Build(ctx, composition.MemoryStorage(memory.New()), cfg)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bs.Close(ctx)) })
	tb := table{t: t, h: router.New(slog.New(slog.NewTextHandler(io.Discard, nil)), bs, router.Options{})}
	root, _ := tb.login("")

	var sent struct {
		ID       string `json:"id"`
		Email    string `json:"email"`
		Language string `json:"language"`
		Mail     string `json:"mail"`
	}
	require.Equal(t, http.StatusCreated, tb.call(root, "POST", "/api/admin/invites", `{"email":"cid@example.com","language":"fr"}`, &sent))
	assert.Equal(t, "sent", sent.Mail)
	assert.Equal(t, "fr", sent.Language)
	require.Len(t, rec.Sent(), 1)
	assert.Equal(t, "cid@example.com", rec.Sent()[0].To)
	assert.Contains(t, rec.Sent()[0].Subject, "vous invite sur Critical Hit")
	assert.Contains(t, rec.Sent()[0].TextBody, "https://critical-hit.fly.dev")

	var again struct {
		Mail string `json:"mail"`
	}
	require.Equal(t, http.StatusOK, tb.call(root, "POST", "/api/admin/invites/"+sent.ID+"/resend", "", &again))
	assert.Equal(t, "sent", again.Mail)
	assert.Len(t, rec.Sent(), 2)
	assert.Equal(t, rec.Sent()[0], rec.Sent()[1], "a resend is the same mail")

	rec.Err = errors.New("provider down")
	var failed struct {
		Mail string `json:"mail"`
	}
	require.Equal(t, http.StatusCreated, tb.call(root, "POST", "/api/admin/invites", `{"email":"dee@example.com"}`, &failed))
	assert.Equal(t, "failed", failed.Mail, "the invite is created, the mail failed")
	var pending invitesView
	require.Equal(t, http.StatusOK, tb.call(root, "GET", "/api/admin/invites", "", &pending))
	assert.Len(t, pending.Invites, 2)

	require.Equal(t, http.StatusOK, tb.call(root, "DELETE", "/api/admin/invites/"+sent.ID, "", nil))
	var refused codeView
	assert.Equal(t, http.StatusConflict, tb.call(root, "POST", "/api/admin/invites/"+sent.ID+"/resend", "", &refused))
	assert.Equal(t, "invite_not_pending", refused.Code)
	var bad codeView
	assert.Equal(t, http.StatusBadRequest, tb.call(root, "POST", "/api/admin/invites", `{"email":"eve@example.com","language":"de"}`, &bad))
}
