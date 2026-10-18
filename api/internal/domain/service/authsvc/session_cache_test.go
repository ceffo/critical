package authsvc_test

// ADR 0029 step 4.7: a server trusts a session it found valid for
// SessionCacheTTL, never past its expiry, and forgets it at once when it logs
// it out, disables its user or changes the user's role.

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/service/authsvc"
	"github.com/ceffo/critical/api/internal/domain/service/authsvc/mocks"
)

// clockEnv is a service whose clock the test moves.
type clockEnv struct {
	svc authsvc.SVC
	dal *mocks.MockDAL
	at  time.Time
}

func newClockEnv(t *testing.T) *clockEnv {
	t.Helper()
	e := &clockEnv{dal: mocks.NewMockDAL(t), at: now}
	svc, err := authsvc.New(authsvc.Deps{DAL: e.dal, Tx: &tx{}, Now: func() time.Time { return e.at }})
	require.NoError(t, err)
	e.svc = svc
	return e
}

// expectLookup expects one read of the session "tok" (expiring at expires)
// and of its user u.
func (e *clockEnv) expectLookup(expires time.Time, u *authsvc.User) {
	e.dal.EXPECT().SessionByID(mock.Anything, authsvc.SessionID("tok")).Return(session(now.Add(-time.Hour), expires), nil).Once()
	e.dal.EXPECT().UserByID(mock.Anything, userID).Return(u, nil).Once()
}

func (e *clockEnv) auth(t *testing.T) *authsvc.Principal {
	t.Helper()
	p, err := e.svc.Authenticate(context.Background(), "tok")
	require.NoError(t, err)
	return p
}

// The requests of a page load read the session once; after SessionCacheTTL
// it is read again.
func TestAuthenticate_Step4_7_TrustedForTheCacheTTL(t *testing.T) {
	e := newClockEnv(t)
	e.expectLookup(now.Add(time.Hour), player)
	for range 5 {
		assert.Equal(t, *player, e.auth(t).User)
	}
	e.at = now.Add(authsvc.SessionCacheTTL - time.Millisecond)
	e.auth(t)

	e.at = now.Add(authsvc.SessionCacheTTL)
	e.expectLookup(now.Add(time.Hour), player)
	e.auth(t)
}

// A session about to expire is not trusted past its expiry: it is read again,
// found expired and refused.
func TestAuthenticate_Step4_7_NeverPastTheSessionsExpiry(t *testing.T) {
	e := newClockEnv(t)
	expires := now.Add(3 * time.Second)
	e.expectLookup(expires, player)
	e.auth(t)

	e.at = expires
	e.dal.EXPECT().SessionByID(mock.Anything, authsvc.SessionID("tok")).Return(session(now.Add(-time.Hour), expires), nil).Once()
	e.dal.EXPECT().DeleteSession(mock.Anything, authsvc.SessionID("tok")).Return(nil).Once()
	_, err := e.svc.Authenticate(context.Background(), "tok")
	require.ErrorIs(t, err, authsvc.ErrUnauthenticated)
}

// Logging out forgets the session at once on this server.
func TestAuthenticate_Step4_7_LogoutForgetsAtOnce(t *testing.T) {
	e := newClockEnv(t)
	e.expectLookup(now.Add(time.Hour), player)
	e.auth(t)

	e.dal.EXPECT().DeleteSession(mock.Anything, authsvc.SessionID("tok")).Return(nil).Once()
	require.NoError(t, e.svc.Logout(context.Background(), "tok"))
	e.dal.EXPECT().SessionByID(mock.Anything, authsvc.SessionID("tok")).Return(nil, notFound()).Once()
	_, err := e.svc.Authenticate(context.Background(), "tok")
	require.ErrorIs(t, err, authsvc.ErrUnauthenticated)
}

// Disabling a user forgets their sessions at once on this server.
func TestAuthenticate_Step4_7_DisablingForgetsAtOnce(t *testing.T) {
	e := newClockEnv(t)
	e.expectLookup(now.Add(time.Hour), player)
	e.auth(t)

	u := *player
	e.dal.EXPECT().UserByID(mock.Anything, userID).Return(&u, nil).Once()
	e.dal.EXPECT().UpdateUser(mock.Anything, mock.Anything).Return(nil).Once()
	e.dal.EXPECT().DeleteUserSessions(mock.Anything, userID).Return(nil).Once()
	require.NoError(t, e.svc.SetDisabled(context.Background(), userID, true))

	e.dal.EXPECT().SessionByID(mock.Anything, authsvc.SessionID("tok")).Return(nil, notFound()).Once()
	_, err := e.svc.Authenticate(context.Background(), "tok")
	require.ErrorIs(t, err, authsvc.ErrUnauthenticated)
}

// A role change is seen by the next request on this server.
func TestAuthenticate_Step4_7_ARoleChangeForgetsAtOnce(t *testing.T) {
	e := newClockEnv(t)
	e.expectLookup(now.Add(time.Hour), player)
	e.auth(t)

	u := *player
	e.dal.EXPECT().UserByID(mock.Anything, userID).Return(&u, nil).Once()
	e.dal.EXPECT().UpdateUser(mock.Anything, mock.Anything).Return(nil).Once()
	require.NoError(t, e.svc.SetRole(context.Background(), userID, authsvc.RoleAdmin))

	admin := *player
	admin.Role = authsvc.RoleAdmin
	e.expectLookup(now.Add(time.Hour), &admin)
	assert.Equal(t, authsvc.RoleAdmin, e.auth(t).User.Role)
}

// An extension is reported once, by the request that wrote it; the cached
// answers after it carry no extension.
func TestAuthenticate_Step4_7_TheCacheNeverReportsAnExtension(t *testing.T) {
	e := newClockEnv(t)
	e.dal.EXPECT().SessionByID(mock.Anything, authsvc.SessionID("tok")).Return(session(now.Add(-authsvc.ExtendAfter), now.Add(time.Hour)), nil).Once()
	e.dal.EXPECT().UserByID(mock.Anything, userID).Return(player, nil).Once()
	e.dal.EXPECT().ExtendSession(mock.Anything, authsvc.SessionID("tok"), now, now.Add(authsvc.SessionLifetime)).Return(nil).Once()
	assert.True(t, e.auth(t).Extended)
	p := e.auth(t)
	assert.False(t, p.Extended)
	assert.Equal(t, now.Add(authsvc.SessionLifetime), p.Expires)
}

// A failed lookup (the database unavailable) is not remembered.
func TestAuthenticate_Step4_7_AFailureIsNotCached(t *testing.T) {
	e := newClockEnv(t)
	boom := errors.New("database unavailable")
	e.dal.EXPECT().SessionByID(mock.Anything, authsvc.SessionID("tok")).Return(nil, boom).Once()
	_, err := e.svc.Authenticate(context.Background(), "tok")
	require.ErrorIs(t, err, boom)
	e.expectLookup(now.Add(time.Hour), player)
	e.auth(t)
}
