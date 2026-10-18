package router_test

// ADR 0029 steps 1.3, 2 and 5.2 at the router: the admin section and the
// crit table's editing answer only to an admin.

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/service/authsvc"
	authmocks "github.com/ceffo/critical/api/internal/domain/service/authsvc/mocks"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
)

// adminPrefix is the admin section's routes.
const adminPrefix = "/api/admin/"

// adminOnly are the routes outside adminPrefix that only an admin reaches.
var adminOnly = map[string]bool{"PUT /api/crit-table": true}

var inviteID = uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")

// adminAndPlayer is an auth service with the sessions "admin" (annID) and
// "player" (bobID).
func adminAndPlayer(t *testing.T) *authmocks.MockSVC {
	t.Helper()
	return usersAuth(t, map[string]authsvc.User{
		"admin":  {ID: annID, Email: "ann@example.com", Role: authsvc.RoleAdmin, CurrentCharacterID: 1},
		"player": {ID: bobID, Email: "bob@example.com", Role: authsvc.RolePlayer, CurrentCharacterID: 2},
	})
}

// Step 1.3: every admin route (walked from the router, so a new one is
// covered) answers 403 admin_only to a player before any service runs (the
// strict mocks expect no call).
func TestAdmin_Step1_3_APlayerIsRefusedEveryAdminRoute(t *testing.T) {
	h, _ := newRouterWith(t, adminAndPlayer(t))
	routes, ok := h.(chi.Routes)
	require.True(t, ok)
	guarded := 0
	require.NoError(t, chi.Walk(routes, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		key := method + " " + route
		if !strings.HasPrefix(route, adminPrefix) && !adminOnly[key] {
			return nil
		}
		guarded++
		rec := as(h, "player", method, urlParam.ReplaceAllString(route, inviteID.String()), `{"email":"x@example.com","disabled":true}`)
		assert.Equal(t, http.StatusForbidden, rec.Code, key)
		assert.JSONEq(t, `{"error":"only an admin may do this","code":"admin_only"}`, rec.Body.String(), key)
		return nil
	}))
	assert.Equal(t, 6, guarded, "the walk found the admin routes")
}

// Without a session the gate answers first: 401, not 403.
func TestAdmin_NoSessionIs401(t *testing.T) {
	h, _ := newRouterWith(t, adminAndPlayer(t))
	rec := anon(h, "GET", "/api/admin/users")
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAdmin_ListUsers(t *testing.T) {
	a := adminAndPlayer(t)
	login := time.Date(2026, 10, 17, 20, 30, 15, 500, time.UTC)
	created := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	a.EXPECT().ListUsers(mock.Anything).Return([]authsvc.UserSummary{
		{User: authsvc.User{ID: annID, Email: "ann@example.com", Role: authsvc.RoleAdmin, Root: true, LastLogin: &login, CreatedAt: created}, Characters: []string{"Kelsen"}},
		{User: authsvc.User{ID: bobID, Email: "bob@example.com", Role: authsvc.RolePlayer, Disabled: true, CreatedAt: created}},
	}, nil).Once()
	h, _ := newRouterWith(t, a)
	rec := as(h, "admin", "GET", "/api/admin/users", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, `{"users":[
		{"id":"`+annID.String()+`","email":"ann@example.com","role":"admin","root":true,"disabled":false,
		 "lastLogin":"2026-10-17T20:30:15Z","createdAt":"2026-10-01T09:00:00Z","characters":["Kelsen"]},
		{"id":"`+bobID.String()+`","email":"bob@example.com","role":"player","root":false,"disabled":true,
		 "lastLogin":null,"createdAt":"2026-10-01T09:00:00Z","characters":[]}]}`, rec.Body.String())
}

// Step 2.1: the invite is sent by the request's user; refusals keep their
// codes.
func TestAdmin_Step2_1_CreateInvite(t *testing.T) {
	created := time.Date(2026, 10, 18, 9, 0, 0, 0, time.UTC)
	t.Run("201 with the invite, invited by the session's user", func(t *testing.T) {
		a := adminAndPlayer(t)
		a.EXPECT().CreateInvite(mock.Anything, annID, "Cid@Example.com").
			Return(&authsvc.Invite{ID: inviteID, Email: "cid@example.com", InvitedBy: annID, CreatedAt: created}, nil).Once()
		h, _ := newRouterWith(t, a)
		rec := as(h, "admin", "POST", "/api/admin/invites", `{"email":"Cid@Example.com"}`)
		require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
		assert.JSONEq(t, `{"id":"`+inviteID.String()+`","email":"cid@example.com","invitedBy":"`+annID.String()+
			`","createdAt":"2026-10-18T09:00:00Z"}`, rec.Body.String())
	})
	for _, tc := range []struct {
		err    error
		status int
		code   string
	}{
		{authsvc.ErrEmailHasUser, http.StatusConflict, "email_has_user"},
		{authsvc.ErrAlreadyInvited, http.StatusConflict, "already_invited"},
	} {
		t.Run(tc.code, func(t *testing.T) {
			a := adminAndPlayer(t)
			a.EXPECT().CreateInvite(mock.Anything, annID, "bob@example.com").Return(nil, tc.err).Once()
			h, _ := newRouterWith(t, a)
			rec := as(h, "admin", "POST", "/api/admin/invites", `{"email":"bob@example.com"}`)
			assert.Equal(t, tc.status, rec.Code)
			assert.Contains(t, rec.Body.String(), `"code":"`+tc.code+`"`)
		})
	}
}

// Step 2.3: cancelling answers the pending invites left; an invite no longer
// pending is invite_not_found.
func TestAdmin_Step2_3_CancelInvite(t *testing.T) {
	t.Run("the pending invites left", func(t *testing.T) {
		a := adminAndPlayer(t)
		a.EXPECT().CancelInvite(mock.Anything, inviteID).Return(nil).Once()
		a.EXPECT().ListInvites(mock.Anything).Return([]authsvc.Invite{}, nil).Once()
		h, _ := newRouterWith(t, a)
		rec := as(h, "admin", "DELETE", "/api/admin/invites/"+inviteID.String(), "")
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.JSONEq(t, `{"invites":[]}`, rec.Body.String())
	})
	t.Run("invite_not_found", func(t *testing.T) {
		a := adminAndPlayer(t)
		a.EXPECT().CancelInvite(mock.Anything, inviteID).Return(authsvc.ErrInviteNotFound).Once()
		h, _ := newRouterWith(t, a)
		rec := as(h, "admin", "DELETE", "/api/admin/invites/"+inviteID.String(), "")
		assert.Equal(t, http.StatusNotFound, rec.Code)
		assert.Contains(t, rec.Body.String(), `"code":"invite_not_found"`)
	})
	t.Run("an id that is no UUID is 400", func(t *testing.T) {
		h, _ := newRouterWith(t, adminAndPlayer(t))
		rec := as(h, "admin", "DELETE", "/api/admin/invites/12", "")
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

// Steps 1.2, 1.3 and 5.2: the change is passed on as sent; the root admin's
// refusal keeps its code.
func TestAdmin_ChangeUser(t *testing.T) {
	yes, admin := true, authsvc.RoleAdmin
	t.Run("both fields in one change", func(t *testing.T) {
		a := adminAndPlayer(t)
		a.EXPECT().ChangeUser(mock.Anything, bobID, authsvc.UserChange{Disabled: &yes, Role: &admin}).
			Return(&authsvc.UserSummary{User: authsvc.User{ID: bobID, Email: "bob@example.com", Role: admin, Disabled: true}}, nil).Once()
		h, _ := newRouterWith(t, a)
		rec := as(h, "admin", "PATCH", "/api/admin/users/"+bobID.String(), `{"disabled":true,"role":"admin"}`)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.Contains(t, rec.Body.String(), `"disabled":true`)
		assert.Contains(t, rec.Body.String(), `"characters":[]`)
	})
	t.Run("root_admin_protected", func(t *testing.T) {
		a := adminAndPlayer(t)
		a.EXPECT().ChangeUser(mock.Anything, annID, authsvc.UserChange{Disabled: &yes}).Return(nil, authsvc.ErrRootProtected).Once()
		h, _ := newRouterWith(t, a)
		rec := as(h, "admin", "PATCH", "/api/admin/users/"+annID.String(), `{"disabled":true}`)
		assert.Equal(t, http.StatusConflict, rec.Code)
		assert.Contains(t, rec.Body.String(), `"code":"root_admin_protected"`)
	})
	t.Run("nothing to change is 400", func(t *testing.T) {
		h, _ := newRouterWith(t, adminAndPlayer(t))
		rec := as(h, "admin", "PATCH", "/api/admin/users/"+bobID.String(), `{}`)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

// The crit table is one rule set for every user: a player reads it, only an
// admin saves it.
func TestCritTable_APlayerReadsButCannotSave(t *testing.T) {
	h, m := newRouterWith(t, adminAndPlayer(t))
	m.cmb.EXPECT().GetCritTable(mock.Anything).Return(combatsvc.CritTable{Rows: []combatsvc.CritRow{{From: 1, To: 100, Multiplier: 2}}}, nil).Once()
	assert.Equal(t, http.StatusOK, as(h, "player", "GET", "/api/crit-table", "").Code)
	rec := as(h, "player", "PUT", "/api/crit-table", `{"rows":[{"from":1,"to":100,"multiplier":2}]}`)
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Contains(t, rec.Body.String(), `"code":"admin_only"`)
}
