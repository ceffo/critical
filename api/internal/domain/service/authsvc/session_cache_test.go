package authsvc_test

// ADR 0029 step 4.7: a server trusts a session it found valid for
// SessionCacheTTL, never past its expiry, and forgets it at once when it logs
// it out, disables its user or changes the user's role.

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
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

// midLookup starts an Authenticate of "tok" and holds it inside its read of
// the user (the old, still valid state is read) until release is closed.
func (e *clockEnv) midLookup(t *testing.T, u *authsvc.User) (release chan struct{}, done chan *authsvc.Principal) {
	t.Helper()
	reading := make(chan struct{})
	release = make(chan struct{})
	done = make(chan *authsvc.Principal, 1)
	e.dal.EXPECT().SessionByID(mock.Anything, authsvc.SessionID("tok")).Return(session(now.Add(-time.Hour), now.Add(time.Hour)), nil).Once()
	e.dal.EXPECT().UserByID(mock.Anything, userID).RunAndReturn(func(context.Context, uuid.UUID) (*authsvc.User, error) {
		close(reading)
		<-release
		return u, nil
	}).Once()
	go func() {
		p, err := e.svc.Authenticate(context.Background(), "tok")
		assert.NoError(t, err)
		done <- p
	}()
	<-reading
	return release, done
}

// A request that read the session before a logout and finishes after it does
// not put the logged-out session back: the next request reads it again.
func TestAuthenticate_Step4_7_ALookupInFlightDuringLogoutIsNotKept(t *testing.T) {
	e := newClockEnv(t)
	release, done := e.midLookup(t, player)

	e.dal.EXPECT().DeleteSession(mock.Anything, authsvc.SessionID("tok")).Return(nil).Once()
	require.NoError(t, e.svc.Logout(context.Background(), "tok"))
	close(release)
	<-done

	e.dal.EXPECT().SessionByID(mock.Anything, authsvc.SessionID("tok")).Return(nil, notFound()).Once()
	_, err := e.svc.Authenticate(context.Background(), "tok")
	require.ErrorIs(t, err, authsvc.ErrUnauthenticated)
}

// The same for disabling the user while their session is being read.
func TestAuthenticate_Step4_7_ALookupInFlightDuringDisablingIsNotKept(t *testing.T) {
	e := newClockEnv(t)
	stale := *player
	release, done := e.midLookup(t, &stale)

	u := *player
	e.dal.EXPECT().UserByID(mock.Anything, userID).Return(&u, nil).Once()
	e.dal.EXPECT().UpdateUser(mock.Anything, mock.Anything).Return(nil).Once()
	e.dal.EXPECT().DeleteUserSessions(mock.Anything, userID).Return(nil).Once()
	require.NoError(t, e.svc.SetDisabled(context.Background(), userID, true))
	close(release)
	<-done

	e.dal.EXPECT().SessionByID(mock.Anything, authsvc.SessionID("tok")).Return(nil, notFound()).Once()
	_, err := e.svc.Authenticate(context.Background(), "tok")
	require.ErrorIs(t, err, authsvc.ErrUnauthenticated)
}

// Step 7.2: the server that changes a user's current character (ForgetUser,
// called by the character service once it committed) or gives the root admin
// the characters without an owner forgets the user's sessions at once, so the
// next request plays the new character.
func TestAuthenticate_Step7_2_ACurrentCharacterChangeForgetsAtOnce(t *testing.T) {
	e := newClockEnv(t)
	before := *player
	before.CurrentCharacterID = 1
	e.expectLookup(now.Add(time.Hour), &before)
	assert.Equal(t, int64(1), e.auth(t).User.CurrentCharacterID)

	after := before
	after.CurrentCharacterID = 2
	e.svc.ForgetUser(userID)
	e.expectLookup(now.Add(time.Hour), &after)
	assert.Equal(t, int64(2), e.auth(t).User.CurrentCharacterID, "read again at once")

	r := *root
	e.dal.EXPECT().RootUser(mock.Anything).Return(&r, nil).Once()
	e.dal.EXPECT().AdoptUnownedCharacters(mock.Anything, userID).Return(nil).Once()
	require.NoError(t, e.svc.AdoptUnownedCharacters(context.Background()))
	e.expectLookup(now.Add(time.Hour), &after)
	e.auth(t)
}

// Step 7.2: the current character as the database holds it, for a request
// that shows another one than this server remembers (a switch made on the
// other server): read at once, and the user's remembered sessions are
// forgotten, so their next request reads the user again.
func TestFreshCurrentCharacter_Step7_2_ReadsTheDatabaseAndForgetsAStaleSession(t *testing.T) {
	e := newClockEnv(t)
	before := *player
	before.CurrentCharacterID = 1
	e.expectLookup(now.Add(time.Hour), &before)
	p := e.auth(t)
	assert.Equal(t, int64(1), e.auth(t).User.CurrentCharacterID, "remembered: no second lookup")

	after := before
	after.CurrentCharacterID = 2
	e.dal.EXPECT().UserByID(mock.Anything, userID).Return(&after, nil).Once()
	got, err := e.svc.FreshCurrentCharacter(context.Background(), p)
	require.NoError(t, err)
	assert.Equal(t, int64(2), got)

	e.expectLookup(now.Add(time.Hour), &after)
	assert.Equal(t, int64(2), e.auth(t).User.CurrentCharacterID, "the stale session is read again")
}

// Step 7.2: when the database agrees with the remembered session, it stays
// remembered; a user deleted meanwhile is unauthenticated, and a database
// error is returned as is (its chain maps to 503 when it is unavailable).
func TestFreshCurrentCharacter_Step7_2_KeepsAnAgreeingSession(t *testing.T) {
	e := newClockEnv(t)
	u := *player
	u.CurrentCharacterID = 1
	e.expectLookup(now.Add(time.Hour), &u)
	p := e.auth(t)
	same := u
	e.dal.EXPECT().UserByID(mock.Anything, userID).Return(&same, nil).Once()
	got, err := e.svc.FreshCurrentCharacter(context.Background(), p)
	require.NoError(t, err)
	assert.Equal(t, int64(1), got)
	assert.Equal(t, int64(1), e.auth(t).User.CurrentCharacterID, "still remembered: no lookup")

	e.dal.EXPECT().UserByID(mock.Anything, userID).Return(nil, domain.ErrNotFound).Once()
	_, err = e.svc.FreshCurrentCharacter(context.Background(), p)
	require.ErrorIs(t, err, authsvc.ErrUnauthenticated)

	e.dal.EXPECT().UserByID(mock.Anything, userID).Return(nil, domain.ErrUnavailable).Once()
	_, err = e.svc.FreshCurrentCharacter(context.Background(), p)
	require.ErrorIs(t, err, domain.ErrUnavailable)
}
