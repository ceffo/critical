// The auth service runs on mocks of its ports: every storage call a test does
// not name fails it (mockery's strict mocks), so a refusal that "writes
// nothing" is checked by the absence of write expectations (ADR 0025 step 6).
package authsvc_test

import (
	"context"
	"errors"
	"fmt"
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

var (
	now     = time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	userID  = uuid.MustParse("11111111-1111-1111-1111-111111111111")
	newID   = uuid.MustParse("22222222-2222-2222-2222-222222222222")
	otherID = uuid.MustParse("33333333-3333-3333-3333-333333333333")
)

// tx runs the body in the caller's context and counts the transactions.
type tx struct{ n int }

func (t *tx) InTx(ctx context.Context, fn func(context.Context) error) error {
	t.n++
	return fn(ctx)
}

type env struct {
	svc    authsvc.SVC
	dal    *mocks.MockDAL
	idp    *mocks.MockIdentityProvider
	tx     *tx
	tokens []string
}

// newEnv builds the service in Google mode (google true) or dev login mode.
// Tokens come in order: tok-1, tok-2…
func newEnv(t *testing.T, google bool) *env {
	t.Helper()
	e := &env{dal: mocks.NewMockDAL(t), tx: &tx{}}
	deps := authsvc.Deps{
		DAL: e.dal, Tx: e.tx,
		Now:   func() time.Time { return now },
		NewID: func() uuid.UUID { return newID },
		NewToken: func() (string, error) {
			tok := fmt.Sprintf("tok-%d", len(e.tokens)+1)
			e.tokens = append(e.tokens, tok)
			return tok, nil
		},
	}
	if google {
		e.idp = mocks.NewMockIdentityProvider(t)
		deps.Provider = e.idp
	}
	svc, err := authsvc.New(deps)
	require.NoError(t, err)
	e.svc = svc
	return e
}

func notFound() error { return fmt.Errorf("%w: test", domain.ErrNotFound) }

func ptr[T any](v T) *T { return &v }

// callback is a login coming back with the state it started with.
func callback() authsvc.FinishLogin {
	return authsvc.FinishLogin{Code: "code", State: "st", ExpectedState: "st", Verifier: "ver"}
}

func (e *env) exchange(id authsvc.Identity) {
	e.idp.EXPECT().Exchange(mock.Anything, "code", "ver").Return(&id, nil).Once()
}

// expectSession expects the session the login starts for user id: the hash of
// the next token, 30 days.
func (e *env) expectSession(id uuid.UUID, token string) {
	e.dal.EXPECT().InsertSession(mock.Anything, authsvc.Session{
		ID: authsvc.SessionID(token), UserID: id, CreatedAt: now, LastUsed: now, Expires: now.Add(authsvc.SessionLifetime),
	}).Return(nil).Once()
}

var verified = authsvc.Identity{Subject: "g-123", Email: "Ann@Example.com", EmailVerified: true}

// Step 3.4 and 2.4: a first login whose email has a pending invite creates the
// user as a player bound to the token's subject, consumes the invite and
// starts a session.
func TestFinishLogin_InvitedEmailBindsItsSubject(t *testing.T) {
	e := newEnv(t, true)
	e.exchange(verified)
	inviteID := uuid.New()
	e.dal.EXPECT().UserBySubject(mock.Anything, "g-123").Return(nil, notFound()).Once()
	e.dal.EXPECT().UserByEmail(mock.Anything, "ann@example.com").Return(nil, notFound()).Once()
	e.dal.EXPECT().PendingInvite(mock.Anything, "ann@example.com").Return(&authsvc.Invite{ID: inviteID, Email: "ann@example.com"}, nil).Once()
	want := authsvc.User{
		ID: newID, Email: "ann@example.com", GoogleSubject: "g-123", Role: authsvc.RolePlayer,
		LastLogin: ptr(now), CreatedAt: now,
	}
	ins := e.dal.EXPECT().InsertUser(mock.Anything, want).Return(nil).Once()
	e.expectSession(newID, "tok-1")
	e.dal.EXPECT().AcceptInvite(mock.Anything, inviteID, newID, now).Return(nil).Once().NotBefore(ins)

	got, err := e.svc.FinishLogin(context.Background(), callback())
	require.NoError(t, err)
	assert.Equal(t, "tok-1", got.Token)
	assert.Equal(t, want, got.User)
	assert.Equal(t, now.Add(authsvc.SessionLifetime), got.Expires)
	assert.Equal(t, 1, e.tx.n, "the login's writes are one transaction")
}

// Step 3.3: a known subject logs in whatever email the token now carries, and
// its last login is set (step 3.6).
func TestFinishLogin_KnownSubjectLogsIn(t *testing.T) {
	e := newEnv(t, true)
	e.exchange(authsvc.Identity{Subject: "g-123", Email: "renamed@example.com", EmailVerified: true})
	u := &authsvc.User{ID: userID, Email: "ann@example.com", GoogleSubject: "g-123", Role: authsvc.RolePlayer}
	e.dal.EXPECT().UserBySubject(mock.Anything, "g-123").Return(u, nil).Once()
	e.dal.EXPECT().UpdateUser(mock.Anything, authsvc.User{
		ID: userID, Email: "ann@example.com", GoogleSubject: "g-123", Role: authsvc.RolePlayer, LastLogin: ptr(now),
	}).Return(nil).Once()
	e.expectSession(userID, "tok-1")

	got, err := e.svc.FinishLogin(context.Background(), callback())
	require.NoError(t, err)
	assert.Equal(t, userID, got.User.ID)
}

// Step 3.4 and 5.1: the root admin set by the script (an email, no subject)
// binds the subject at its first login, with no invite.
func TestFinishLogin_RootAdminBindsItsSubjectAtFirstLogin(t *testing.T) {
	e := newEnv(t, true)
	e.exchange(verified)
	root := &authsvc.User{ID: userID, Email: "ann@example.com", Role: authsvc.RoleAdmin, Root: true}
	e.dal.EXPECT().UserBySubject(mock.Anything, "g-123").Return(nil, notFound()).Once()
	e.dal.EXPECT().UserByEmail(mock.Anything, "ann@example.com").Return(root, nil).Once()
	e.dal.EXPECT().UpdateUser(mock.Anything, authsvc.User{
		ID: userID, Email: "ann@example.com", GoogleSubject: "g-123", Role: authsvc.RoleAdmin, Root: true, LastLogin: ptr(now),
	}).Return(nil).Once()
	e.expectSession(userID, "tok-1")

	_, err := e.svc.FinishLogin(context.Background(), callback())
	require.NoError(t, err)
}

// Step 3.4: an email with no pending invite (none, or only a cancelled or
// used one, which PendingInvite does not return) is refused and nothing is
// written (no write expectation: any write fails the test).
func TestFinishLogin_UninvitedRefusedAndNothingWritten(t *testing.T) {
	e := newEnv(t, true)
	e.exchange(verified)
	e.dal.EXPECT().UserBySubject(mock.Anything, "g-123").Return(nil, notFound()).Once()
	e.dal.EXPECT().UserByEmail(mock.Anything, "ann@example.com").Return(nil, notFound()).Once()
	e.dal.EXPECT().PendingInvite(mock.Anything, "ann@example.com").Return(nil, notFound()).Once()

	_, err := e.svc.FinishLogin(context.Background(), callback())
	require.ErrorIs(t, err, authsvc.ErrNotInvited)
	assert.Empty(t, e.tokens, "no session token drawn")
}

// Step 3.3: an email whose user is bound to another Google account does not
// log this one in.
func TestFinishLogin_EmailOfAnotherBoundUserRefused(t *testing.T) {
	e := newEnv(t, true)
	e.exchange(verified)
	e.dal.EXPECT().UserBySubject(mock.Anything, "g-123").Return(nil, notFound()).Once()
	e.dal.EXPECT().UserByEmail(mock.Anything, "ann@example.com").Return(&authsvc.User{
		ID: userID, Email: "ann@example.com", GoogleSubject: "g-other", Role: authsvc.RolePlayer,
	}, nil).Once()

	_, err := e.svc.FinishLogin(context.Background(), callback())
	require.ErrorIs(t, err, authsvc.ErrNotInvited)
}

// Step 3.5: a disabled user is refused and no session is created, whether
// found by subject or, at a first login, by email.
func TestFinishLogin_DisabledUserRefused(t *testing.T) {
	t.Run("by subject", func(t *testing.T) {
		e := newEnv(t, true)
		e.exchange(verified)
		e.dal.EXPECT().UserBySubject(mock.Anything, "g-123").Return(&authsvc.User{
			ID: userID, Email: "ann@example.com", GoogleSubject: "g-123", Role: authsvc.RolePlayer, Disabled: true,
		}, nil).Once()
		_, err := e.svc.FinishLogin(context.Background(), callback())
		require.ErrorIs(t, err, authsvc.ErrUserDisabled)
	})
	t.Run("first login by email", func(t *testing.T) {
		e := newEnv(t, true)
		e.exchange(verified)
		e.dal.EXPECT().UserBySubject(mock.Anything, "g-123").Return(nil, notFound()).Once()
		e.dal.EXPECT().UserByEmail(mock.Anything, "ann@example.com").Return(&authsvc.User{
			ID: userID, Email: "ann@example.com", Role: authsvc.RolePlayer, Disabled: true,
		}, nil).Once()
		_, err := e.svc.FinishLogin(context.Background(), callback())
		require.ErrorIs(t, err, authsvc.ErrUserDisabled)
	})
}

// Step 3.1: a callback whose state is not the one the login started with (or
// with no login cookie: no expected state or verifier) is refused before the
// code is exchanged (the provider mock has no expectation).
func TestFinishLogin_StateOrPKCEMismatchRefused(t *testing.T) {
	cases := map[string]authsvc.FinishLogin{
		"other state":       {Code: "code", State: "forged", ExpectedState: "st", Verifier: "ver"},
		"no login cookie":   {Code: "code", State: "st"},
		"no state returned": {Code: "code", ExpectedState: "st", Verifier: "ver"},
		"no verifier":       {Code: "code", State: "st", ExpectedState: "st"},
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t, true)
			_, err := e.svc.FinishLogin(context.Background(), in)
			require.ErrorIs(t, err, authsvc.ErrStateMismatch)
		})
	}
}

// Step 3.1: the code is exchanged with the verifier kept at the start (the
// PKCE proof); a failed exchange or token check is a refusal.
func TestFinishLogin_FailedExchangeRefused(t *testing.T) {
	e := newEnv(t, true)
	e.idp.EXPECT().Exchange(mock.Anything, "code", "ver").Return(nil, errors.New("invalid_grant")).Once()
	_, err := e.svc.FinishLogin(context.Background(), callback())
	require.ErrorIs(t, err, authsvc.ErrLoginFailed)
}

// Step 3.1: the provider's own refusal (the user cancelled) is a refusal,
// with nothing exchanged.
func TestFinishLogin_ProviderErrorRefused(t *testing.T) {
	e := newEnv(t, true)
	in := callback()
	in.Code, in.ProviderError = "", "access_denied"
	_, err := e.svc.FinishLogin(context.Background(), in)
	require.ErrorIs(t, err, authsvc.ErrLoginFailed)
}

// Step 3.2: only a verified email is accepted.
func TestFinishLogin_UnverifiedEmailRefused(t *testing.T) {
	e := newEnv(t, true)
	e.exchange(authsvc.Identity{Subject: "g-123", Email: "ann@example.com", EmailVerified: false})
	_, err := e.svc.FinishLogin(context.Background(), callback())
	require.ErrorIs(t, err, authsvc.ErrEmailUnverified)
}

// Step 3.1: every login gets a fresh state and verifier, both handed to the
// provider (which puts the state and the verifier's S256 challenge in the URL).
func TestStartLogin_FreshStateAndVerifier(t *testing.T) {
	e := newEnv(t, true)
	e.idp.EXPECT().AuthURL("tok-1", "tok-2").Return("https://accounts.example/auth?x").Once()
	got, err := e.svc.StartLogin(context.Background())
	require.NoError(t, err)
	assert.Equal(t, authsvc.LoginStart{URL: "https://accounts.example/auth?x", State: "tok-1", Verifier: "tok-2"}, *got)
}

// Step 8.1: without OAuth settings there is no Google login, and with them
// no dev login (step 8.2).
func TestLoginModes_GoogleOrDevNeverBoth(t *testing.T) {
	dev := newEnv(t, false)
	assert.True(t, dev.svc.DevLoginEnabled())
	_, err := dev.svc.StartLogin(context.Background())
	require.ErrorIs(t, err, domain.ErrNotFound)
	_, err = dev.svc.FinishLogin(context.Background(), callback())
	require.ErrorIs(t, err, domain.ErrNotFound)

	google := newEnv(t, true)
	assert.False(t, google.svc.DevLoginEnabled())
	_, err = google.svc.DevLogin(context.Background(), "")
	require.ErrorIs(t, err, authsvc.ErrDevLoginDisabled)
	require.ErrorIs(t, err, domain.ErrNotFound)
}

func session(lastUsed, expires time.Time) *authsvc.Session {
	return &authsvc.Session{ID: authsvc.SessionID("tok"), UserID: userID, CreatedAt: lastUsed, LastUsed: lastUsed, Expires: expires}
}

var player = &authsvc.User{ID: userID, Email: "ann@example.com", GoogleSubject: "g-123", Role: authsvc.RolePlayer}

// Step 4.1: the session is looked up by the hash of the cookie's token, and
// a session used within the day is not written to (step 4.3).
func TestAuthenticate_ValidSessionIsItsUser(t *testing.T) {
	e := newEnv(t, true)
	e.dal.EXPECT().SessionByID(mock.Anything, authsvc.SessionID("tok")).Return(session(now.Add(-time.Hour), now.Add(time.Hour)), nil).Once()
	e.dal.EXPECT().UserByID(mock.Anything, userID).Return(player, nil).Once()

	p, err := e.svc.Authenticate(context.Background(), "tok")
	require.NoError(t, err)
	assert.Equal(t, *player, p.User)
	assert.False(t, p.Extended)
	assert.Equal(t, now.Add(time.Hour), p.Expires)
	assert.Zero(t, e.tx.n, "a session lookup is a read: no transaction, no lock")
}

// Step 4.3: a request more than a day after the last extension pushes the
// expiry to 30 days from now.
func TestAuthenticate_ExtendsAfterADay(t *testing.T) {
	e := newEnv(t, true)
	e.dal.EXPECT().SessionByID(mock.Anything, authsvc.SessionID("tok")).Return(session(now.Add(-authsvc.ExtendAfter), now.Add(time.Hour)), nil).Once()
	e.dal.EXPECT().UserByID(mock.Anything, userID).Return(player, nil).Once()
	e.dal.EXPECT().ExtendSession(mock.Anything, authsvc.SessionID("tok"), now, now.Add(authsvc.SessionLifetime)).Return(nil).Once()

	p, err := e.svc.Authenticate(context.Background(), "tok")
	require.NoError(t, err)
	assert.True(t, p.Extended)
	assert.Equal(t, now.Add(authsvc.SessionLifetime), p.Expires)
}

// Step 4.4: an expired session is refused and deleted.
func TestAuthenticate_ExpiredSessionRefusedAndDeleted(t *testing.T) {
	e := newEnv(t, true)
	e.dal.EXPECT().SessionByID(mock.Anything, authsvc.SessionID("tok")).Return(session(now.Add(-31*24*time.Hour), now), nil).Once()
	e.dal.EXPECT().DeleteSession(mock.Anything, authsvc.SessionID("tok")).Return(nil).Once()

	_, err := e.svc.Authenticate(context.Background(), "tok")
	require.ErrorIs(t, err, authsvc.ErrUnauthenticated)
}

// Step 4.4: no token or an unknown one is refused.
func TestAuthenticate_UnknownSessionRefused(t *testing.T) {
	e := newEnv(t, true)
	_, err := e.svc.Authenticate(context.Background(), "")
	require.ErrorIs(t, err, authsvc.ErrUnauthenticated)

	e.dal.EXPECT().SessionByID(mock.Anything, authsvc.SessionID("tok")).Return(nil, notFound()).Once()
	_, err = e.svc.Authenticate(context.Background(), "tok")
	require.ErrorIs(t, err, authsvc.ErrUnauthenticated)
}

// Step 4.4: a disabled user's session is refused, and every session of that
// user is dropped.
func TestAuthenticate_DisabledUserRefusedAndSessionsDropped(t *testing.T) {
	e := newEnv(t, true)
	e.dal.EXPECT().SessionByID(mock.Anything, authsvc.SessionID("tok")).Return(session(now, now.Add(time.Hour)), nil).Once()
	disabled := *player
	disabled.Disabled = true
	e.dal.EXPECT().UserByID(mock.Anything, userID).Return(&disabled, nil).Once()
	e.dal.EXPECT().DeleteUserSessions(mock.Anything, userID).Return(nil).Once()

	_, err := e.svc.Authenticate(context.Background(), "tok")
	require.ErrorIs(t, err, authsvc.ErrUnauthenticated)
}

// Step 4.4: logout deletes the token's session.
func TestLogout_DeletesTheSession(t *testing.T) {
	e := newEnv(t, true)
	e.dal.EXPECT().DeleteSession(mock.Anything, authsvc.SessionID("tok")).Return(nil).Once()
	require.NoError(t, e.svc.Logout(context.Background(), "tok"))
	require.NoError(t, e.svc.Logout(context.Background(), ""), "no cookie: nothing to do")
}

// Steps 1.2 and 4.4: disabling a user deletes all their sessions in the same
// transaction.
func TestSetDisabled_DropsTheUsersSessionsInTheSameTransaction(t *testing.T) {
	e := newEnv(t, true)
	u := *player
	e.dal.EXPECT().UserByID(mock.Anything, userID).Return(&u, nil).Once()
	disabled := *player
	disabled.Disabled = true
	upd := e.dal.EXPECT().UpdateUser(mock.Anything, disabled).Return(nil).Once()
	e.dal.EXPECT().DeleteUserSessions(mock.Anything, userID).Return(nil).Once().NotBefore(upd)

	require.NoError(t, e.svc.SetDisabled(context.Background(), userID, true))
	assert.Equal(t, 1, e.tx.n)
}

// Step 1.2: re-enabling undoes it and deletes nothing.
func TestSetDisabled_ReEnable(t *testing.T) {
	e := newEnv(t, true)
	u := *player
	u.Disabled = true
	e.dal.EXPECT().UserByID(mock.Anything, userID).Return(&u, nil).Once()
	e.dal.EXPECT().UpdateUser(mock.Anything, *player).Return(nil).Once()
	require.NoError(t, e.svc.SetDisabled(context.Background(), userID, false))
}

var root = &authsvc.User{ID: userID, Email: "root@example.com", GoogleSubject: "g-root", Role: authsvc.RoleAdmin, Root: true}

// Step 5.2: the root admin cannot be disabled or demoted; nothing is written.
func TestRootAdmin_CannotBeDisabledOrDemoted(t *testing.T) {
	e := newEnv(t, true)
	r := *root
	e.dal.EXPECT().UserByID(mock.Anything, userID).Return(&r, nil).Times(2)

	require.ErrorIs(t, e.svc.SetDisabled(context.Background(), userID, true), authsvc.ErrRootProtected)
	err := e.svc.SetRole(context.Background(), userID, authsvc.RolePlayer)
	require.ErrorIs(t, err, authsvc.ErrRootProtected)
	require.ErrorIs(t, err, domain.ErrConflict, "a 409")
}

// Step 1.3: an admin makes and unmakes admins.
func TestSetRole_PromotesAndDemotes(t *testing.T) {
	e := newEnv(t, true)
	u := *player
	e.dal.EXPECT().UserByID(mock.Anything, userID).Return(&u, nil).Once()
	admin := *player
	admin.Role = authsvc.RoleAdmin
	e.dal.EXPECT().UpdateUser(mock.Anything, admin).Return(nil).Once()
	require.NoError(t, e.svc.SetRole(context.Background(), userID, authsvc.RoleAdmin))

	require.ErrorIs(t, e.svc.SetRole(context.Background(), userID, "dm"), domain.ErrInvalidInput)
}

// expectAdopt expects the root admin id to get the characters without an
// owner after write, then to be read back as readBack (its current character
// set).
func (e *env) expectAdopt(id uuid.UUID, readBack authsvc.User, after *mock.Call) {
	adopt := e.dal.EXPECT().AdoptUnownedCharacters(mock.Anything, id).Return(nil).Once()
	if after != nil {
		adopt.NotBefore(after)
	}
	e.dal.EXPECT().UserByID(mock.Anything, id).Return(&readBack, nil).Once().NotBefore(adopt)
}

// Step 5.1: the script creates the root admin as an admin with no subject;
// step 5.3: the characters without an owner (Kelsen) become theirs, the first
// one current, in the same transaction.
func TestSetRootAdmin_CreatesTheRoot(t *testing.T) {
	e := newEnv(t, true)
	e.dal.EXPECT().RootUser(mock.Anything).Return(nil, notFound()).Once()
	e.dal.EXPECT().UserByEmail(mock.Anything, "root@example.com").Return(nil, notFound()).Once()
	want := authsvc.User{ID: newID, Email: "root@example.com", Role: authsvc.RoleAdmin, Root: true, CreatedAt: now}
	insert := e.dal.EXPECT().InsertUser(mock.Anything, want).Return(nil).Once()
	adopted := want
	adopted.CurrentCharacterID = 1
	e.expectAdopt(newID, adopted, insert)

	got, err := e.svc.SetRootAdmin(context.Background(), " Root@Example.com ")
	require.NoError(t, err)
	assert.Equal(t, adopted, *got)
	assert.Equal(t, 1, e.tx.n, "one transaction")
}

// Step 5.1: run again with the same email it changes nothing but giving the
// root admin the characters still without an owner (step 5.3).
func TestSetRootAdmin_Idempotent(t *testing.T) {
	e := newEnv(t, true)
	r := *root
	e.dal.EXPECT().RootUser(mock.Anything).Return(&r, nil).Once()
	e.expectAdopt(root.ID, *root, nil)
	got, err := e.svc.SetRootAdmin(context.Background(), "root@example.com")
	require.NoError(t, err)
	assert.Equal(t, *root, *got)
}

// Step 5.1: with another email it refuses while the root has logged in, so
// the root cannot be silently moved.
func TestSetRootAdmin_RefusesToMoveABoundRoot(t *testing.T) {
	e := newEnv(t, true)
	r := *root
	e.dal.EXPECT().RootUser(mock.Anything).Return(&r, nil).Once()
	_, err := e.svc.SetRootAdmin(context.Background(), "other@example.com")
	require.ErrorIs(t, err, authsvc.ErrRootTaken)
}

// Step 5.1: a root that never logged in moves to the new email (an existing
// user is promoted); the old row loses the flag and is disabled, never deleted.
// Step 5.3: the characters it was given (Kelsen, after a typo) move with the
// flag, before the new root's current character is set, in one transaction.
func TestSetRootAdmin_MovesAnUnboundRoot(t *testing.T) {
	e := newEnv(t, true)
	old := authsvc.User{ID: otherID, Email: "typo@example.com", Role: authsvc.RoleAdmin, Root: true}
	e.dal.EXPECT().RootUser(mock.Anything).Return(&old, nil).Once()
	demoted := authsvc.User{ID: otherID, Email: "typo@example.com", Role: authsvc.RoleAdmin, Disabled: true}
	first := e.dal.EXPECT().UpdateUser(mock.Anything, demoted).Return(nil).Once()
	e.dal.EXPECT().DeleteUserSessions(mock.Anything, otherID).Return(nil).Once()
	u := *player
	e.dal.EXPECT().UserByEmail(mock.Anything, "ann@example.com").Return(&u, nil).Once()
	promoted := *player
	promoted.Role, promoted.Root = authsvc.RoleAdmin, true
	update := e.dal.EXPECT().UpdateUser(mock.Anything, promoted).Return(nil).Once().NotBefore(first)
	move := e.dal.EXPECT().MoveCharacters(mock.Anything, otherID, player.ID).Return(nil).Once().NotBefore(update)
	playing := promoted
	playing.CurrentCharacterID = 1
	e.expectAdopt(player.ID, playing, move)

	got, err := e.svc.SetRootAdmin(context.Background(), "ann@example.com")
	require.NoError(t, err)
	assert.Equal(t, playing, *got, "the new root plays the moved character")
	assert.Equal(t, 1, e.tx.n, "one transaction")
}

// Step 5.3: a failed move leaves the transaction body: the old root keeps
// its characters and the flag (the transaction rolls back).
func TestSetRootAdmin_MoveFailureAborts(t *testing.T) {
	e := newEnv(t, true)
	old := authsvc.User{ID: otherID, Email: "typo@example.com", Role: authsvc.RoleAdmin, Root: true}
	e.dal.EXPECT().RootUser(mock.Anything).Return(&old, nil).Once()
	e.dal.EXPECT().UpdateUser(mock.Anything, mock.Anything).Return(nil).Twice()
	e.dal.EXPECT().DeleteUserSessions(mock.Anything, otherID).Return(nil).Once()
	u := *player
	e.dal.EXPECT().UserByEmail(mock.Anything, "ann@example.com").Return(&u, nil).Once()
	e.dal.EXPECT().MoveCharacters(mock.Anything, otherID, player.ID).Return(assert.AnError).Once()

	_, err := e.svc.SetRootAdmin(context.Background(), "ann@example.com")
	require.ErrorIs(t, err, assert.AnError)
}

// Step 8.1: the dev login with no email logs in as the root admin.
func TestDevLogin_NoEmailIsTheRootAdmin(t *testing.T) {
	e := newEnv(t, false)
	r := *root
	e.dal.EXPECT().RootUser(mock.Anything).Return(&r, nil).Once()
	logged := *root
	logged.LastLogin = ptr(now)
	e.dal.EXPECT().UpdateUser(mock.Anything, logged).Return(nil).Once()
	e.expectSession(userID, "tok-1")
	read := logged
	read.CurrentCharacterID = 4
	e.expectAdopt(userID, read, nil)

	got, err := e.svc.DevLogin(context.Background(), "")
	require.NoError(t, err)
	assert.Equal(t, read, got.User, "the user as read back, with the current character")
}

// Step 8.1: on a database with no root admin (a first run after a clone, or
// after `just dev-reset`) the dev login creates one, DevEmail: local dev
// needs no setup.
func TestDevLogin_CreatesARootAdminOnAnEmptyDatabase(t *testing.T) {
	e := newEnv(t, false)
	e.dal.EXPECT().RootUser(mock.Anything).Return(nil, notFound()).Once()
	e.dal.EXPECT().UserByEmail(mock.Anything, authsvc.DevEmail).Return(nil, notFound()).Once()
	want := authsvc.User{ID: newID, Email: authsvc.DevEmail, Role: authsvc.RoleAdmin, Root: true, LastLogin: ptr(now), CreatedAt: now}
	insert := e.dal.EXPECT().InsertUser(mock.Anything, want).Return(nil).Once()
	e.expectSession(newID, "tok-1")
	adopted := want
	adopted.CurrentCharacterID = 1
	e.expectAdopt(newID, adopted, insert)

	got, err := e.svc.DevLogin(context.Background(), "")
	require.NoError(t, err)
	assert.Equal(t, int64(1), got.User.CurrentCharacterID, "step 5.3: the seed character is the new root admin's")
}

// Step 8.1: a named email (the e2e suite's) is created as an admin without an
// invite.
func TestDevLogin_NamedUserCreatedAsAdmin(t *testing.T) {
	e := newEnv(t, false)
	e.dal.EXPECT().UserByEmail(mock.Anything, "e2e@example.com").Return(nil, notFound()).Once()
	want := authsvc.User{ID: newID, Email: "e2e@example.com", Role: authsvc.RoleAdmin, LastLogin: ptr(now), CreatedAt: now}
	e.dal.EXPECT().InsertUser(mock.Anything, want).Return(nil).Once()
	e.expectSession(newID, "tok-1")

	_, err := e.svc.DevLogin(context.Background(), "E2E@example.com")
	require.NoError(t, err)
}

// Step 3.5 holds for the dev login too: a disabled user gets no session.
func TestDevLogin_DisabledUserRefused(t *testing.T) {
	e := newEnv(t, false)
	u := *player
	u.Disabled = true
	e.dal.EXPECT().UserByEmail(mock.Anything, "ann@example.com").Return(&u, nil).Once()
	_, err := e.svc.DevLogin(context.Background(), "ann@example.com")
	require.ErrorIs(t, err, authsvc.ErrUserDisabled)
}

// Step 4.1: the stored session id is a hash of the token, never the token;
// tokens are 256 random bits.
func TestTokens_RandomAndStoredHashed(t *testing.T) {
	a, err := authsvc.RandomToken()
	require.NoError(t, err)
	b, err := authsvc.RandomToken()
	require.NoError(t, err)
	assert.Len(t, a, 43, "32 bytes, base64url without padding")
	assert.NotEqual(t, a, b)
	assert.NotEqual(t, a, authsvc.SessionID(a))
	assert.Len(t, authsvc.SessionID(a), 64)
}

// Step 5.2: naming the DevEmail user while another user is the root logs it
// in as it is (here disabled: refused); only a database with no root makes
// DevEmail the root.
func TestDevLogin_NamedDevEmailIsNotMadeRoot(t *testing.T) {
	e := newEnv(t, false)
	u := authsvc.User{ID: otherID, Email: authsvc.DevEmail, Role: authsvc.RoleAdmin, Disabled: true}
	e.dal.EXPECT().UserByEmail(mock.Anything, authsvc.DevEmail).Return(&u, nil).Once()
	_, err := e.svc.DevLogin(context.Background(), authsvc.DevEmail)
	require.ErrorIs(t, err, authsvc.ErrUserDisabled)
}

// Step 8.1: on a database with no root, an existing DevEmail user (a root
// moved away and back) becomes the root again.
func TestDevLogin_ExistingDevEmailBecomesRootWhenThereIsNone(t *testing.T) {
	e := newEnv(t, false)
	e.dal.EXPECT().RootUser(mock.Anything).Return(nil, notFound()).Once()
	u := authsvc.User{ID: otherID, Email: authsvc.DevEmail, Role: authsvc.RolePlayer, Disabled: true}
	e.dal.EXPECT().UserByEmail(mock.Anything, authsvc.DevEmail).Return(&u, nil).Once()
	made := authsvc.User{ID: otherID, Email: authsvc.DevEmail, Role: authsvc.RoleAdmin, Root: true, LastLogin: ptr(now)}
	update := e.dal.EXPECT().UpdateUser(mock.Anything, made).Return(nil).Once()
	e.expectSession(otherID, "tok-1")
	e.expectAdopt(otherID, made, update)
	_, err := e.svc.DevLogin(context.Background(), "")
	require.NoError(t, err)
}

// Step 5.3: at start-up the characters without an owner go to the root admin
// (the characters stored before characters had owners), in one transaction.
func TestAdoptUnownedCharacters_Step5_3_GivesThemToTheRootAdmin(t *testing.T) {
	e := newEnv(t, true)
	r := *root
	find := e.dal.EXPECT().RootUser(mock.Anything).Return(&r, nil).Once()
	e.dal.EXPECT().AdoptUnownedCharacters(mock.Anything, root.ID).Return(nil).Once().NotBefore(find)
	require.NoError(t, e.svc.AdoptUnownedCharacters(context.Background()))
	assert.Equal(t, 1, e.tx.n)
}

// Step 5.3: without a root admin the characters stay without an owner until
// one exists (the root-admin script or the dev login gives them then).
func TestAdoptUnownedCharacters_Step5_3_NoRootAdminChangesNothing(t *testing.T) {
	e := newEnv(t, true)
	e.dal.EXPECT().RootUser(mock.Anything).Return(nil, notFound()).Once()
	require.NoError(t, e.svc.AdoptUnownedCharacters(context.Background()))

	e = newEnv(t, true)
	e.dal.EXPECT().RootUser(mock.Anything).Return(nil, assert.AnError).Once()
	require.ErrorIs(t, e.svc.AdoptUnownedCharacters(context.Background()), assert.AnError)
}
