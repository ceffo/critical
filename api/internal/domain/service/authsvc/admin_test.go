package authsvc_test

// The admin section (ADR 0029 steps 1.2, 1.3, 2 and 5.2) on mocks of the DAL.

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/authsvc"
)

// Step 2.1: an invite is the email lower-cased, who invited and when, written
// once the email has neither a user nor a pending invite.
func TestCreateInvite_Step2_1_AuthorisesTheEmail(t *testing.T) {
	e := newEnv(t, true)
	e.dal.EXPECT().UserByEmail(mock.Anything, "cid@example.com").Return(nil, notFound()).Once()
	e.dal.EXPECT().PendingInvite(mock.Anything, "cid@example.com").Return(nil, notFound()).Once()
	want := authsvc.Invite{ID: newID, Email: "cid@example.com", InvitedBy: otherID, CreatedAt: now, Language: authsvc.LanguageEnglish}
	e.dal.EXPECT().InsertInvite(mock.Anything, want).Return(nil).Once()

	got, outcome, err := e.svc.CreateInvite(context.Background(), otherID, "  Cid@Example.COM ", "")
	require.NoError(t, err)
	assert.Equal(t, want, *got)
	assert.Equal(t, authsvc.MailNotConfigured, outcome, "no notifier, no mail")
	assert.Equal(t, 1, e.tx.n, "the checks and the write are one transaction")
}

// Step 2.1: an email that already has a user, or a pending invite, is
// refused with its code; nothing is written.
func TestCreateInvite_Step2_1_ExistingUserOrDuplicateRefused(t *testing.T) {
	t.Run("an existing user", func(t *testing.T) {
		e := newEnv(t, true)
		e.dal.EXPECT().UserByEmail(mock.Anything, "bob@example.com").Return(&authsvc.User{ID: userID, Email: "bob@example.com"}, nil).Once()
		_, _, err := e.svc.CreateInvite(context.Background(), otherID, "Bob@example.com", "")
		require.ErrorIs(t, err, authsvc.ErrEmailHasUser)
		require.ErrorIs(t, err, domain.ErrConflict, "a 409")
	})
	t.Run("a pending invite", func(t *testing.T) {
		e := newEnv(t, true)
		e.dal.EXPECT().UserByEmail(mock.Anything, "cid@example.com").Return(nil, notFound()).Once()
		e.dal.EXPECT().PendingInvite(mock.Anything, "cid@example.com").Return(&authsvc.Invite{ID: userID, Email: "cid@example.com"}, nil).Once()
		_, _, err := e.svc.CreateInvite(context.Background(), otherID, "cid@example.com", "")
		require.ErrorIs(t, err, authsvc.ErrAlreadyInvited)
		require.ErrorIs(t, err, domain.ErrConflict, "a 409")
	})
}

// An address that is not a bare email is a field error on "email", checked
// before any read.
func TestCreateInvite_NotAnEmailRefused(t *testing.T) {
	for _, email := range []string{"", "bob", "bob@", "@example.com", "Bob <bob@example.com>", "a b@example.com"} {
		t.Run(email, func(t *testing.T) {
			e := newEnv(t, true)
			_, _, err := e.svc.CreateInvite(context.Background(), otherID, email, "")
			require.ErrorIs(t, err, domain.ErrInvalidInput)
			var fe *domain.FieldError
			require.ErrorAs(t, err, &fe)
			assert.Equal(t, "email", fe.Field)
			assert.Equal(t, 0, e.tx.n)
		})
	}
}

// Step 2.3: an invite is cancelled (at now), never deleted; one no longer
// pending is invite_not_found.
func TestCancelInvite_Step2_3(t *testing.T) {
	e := newEnv(t, true)
	e.dal.EXPECT().CancelInvite(mock.Anything, userID, now).Return(nil).Once()
	require.NoError(t, e.svc.CancelInvite(context.Background(), userID))

	e.dal.EXPECT().CancelInvite(mock.Anything, otherID, now).Return(notFound()).Once()
	err := e.svc.CancelInvite(context.Background(), otherID)
	require.ErrorIs(t, err, authsvc.ErrInviteNotFound)
	require.ErrorIs(t, err, domain.ErrNotFound, "a 404")
}

// Steps 1.3 and 4.4: one change may disable and demote at once; it is one
// transaction in which the sessions are deleted after the user is written,
// and the answer is the user as the admin list shows them.
func TestChangeUser_Step4_4_DisablingDeletesTheSessionsInTheSameTransaction(t *testing.T) {
	e := newEnv(t, true)
	admin := *player
	admin.Role = authsvc.RoleAdmin
	e.dal.EXPECT().UserByID(mock.Anything, userID).Return(&admin, nil).Once()
	changed := admin
	changed.Role, changed.Disabled = authsvc.RolePlayer, true
	upd := e.dal.EXPECT().UpdateUser(mock.Anything, changed).Return(nil).Once()
	del := e.dal.EXPECT().DeleteUserSessions(mock.Anything, userID).Return(nil).Once().NotBefore(upd)
	listed := authsvc.UserSummary{User: changed, Characters: []string{"Bram"}}
	other := authsvc.User{ID: otherID, Email: "root@example.com", Role: authsvc.RoleAdmin, Root: true}
	e.dal.EXPECT().ListUsers(mock.Anything).Return([]authsvc.UserSummary{{User: other}, listed}, nil).Once().NotBefore(del)

	got, err := e.svc.ChangeUser(context.Background(), userID, authsvc.UserChange{
		Disabled: ptr(true), Role: ptr(authsvc.RolePlayer),
	})
	require.NoError(t, err)
	assert.Equal(t, listed, *got)
	assert.Equal(t, 1, e.tx.n)
}

// Step 5.2: the root admin is neither disabled nor demoted, alone or with
// another field; nothing is written (the strict mock expects no write).
func TestChangeUser_Step5_2_TheRootAdminIsProtected(t *testing.T) {
	for name, change := range map[string]authsvc.UserChange{
		"disable":             {Disabled: ptr(true)},
		"demote":              {Role: ptr(authsvc.RolePlayer)},
		"disable as an admin": {Disabled: ptr(true), Role: ptr(authsvc.RoleAdmin)},
	} {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t, true)
			r := *root
			e.dal.EXPECT().UserByID(mock.Anything, userID).Return(&r, nil).Once()
			_, err := e.svc.ChangeUser(context.Background(), userID, change)
			require.ErrorIs(t, err, authsvc.ErrRootProtected)
			require.ErrorIs(t, err, domain.ErrConflict, "a 409")
		})
	}
}

// An unknown user is user_not_found; an unknown role is a field error read
// before the user.
func TestChangeUser_UnknownUserOrRole(t *testing.T) {
	e := newEnv(t, true)
	e.dal.EXPECT().UserByID(mock.Anything, otherID).Return(nil, notFound()).Once()
	_, err := e.svc.ChangeUser(context.Background(), otherID, authsvc.UserChange{Disabled: ptr(true)})
	require.ErrorIs(t, err, authsvc.ErrUserNotFound)

	_, err = e.svc.ChangeUser(context.Background(), otherID, authsvc.UserChange{Role: ptr(authsvc.Role("dm"))})
	require.ErrorIs(t, err, domain.ErrInvalidInput)
}

// Steps 4.4 and 4.7: once an admin disabled a user, that user's next request
// on this server is refused at once, even inside the cache window.
func TestChangeUser_Step4_7_DisablingLogsTheUserOutAtOnce(t *testing.T) {
	e := newEnv(t, true)
	clock := now
	svc, err := authsvc.New(authsvc.Deps{DAL: e.dal, Tx: e.tx, Now: func() time.Time { return clock }})
	require.NoError(t, err)
	u := *player
	sess := &authsvc.Session{ID: authsvc.SessionID("tok"), UserID: userID, LastUsed: now, Expires: now.Add(time.Hour)}
	e.dal.EXPECT().SessionByID(mock.Anything, sess.ID).Return(sess, nil).Once()
	e.dal.EXPECT().UserByID(mock.Anything, userID).Return(&u, nil).Once()
	_, err = svc.Authenticate(context.Background(), "tok")
	require.NoError(t, err)

	e.dal.EXPECT().UserByID(mock.Anything, userID).Return(&u, nil).Once()
	disabled := u
	disabled.Disabled = true
	e.dal.EXPECT().UpdateUser(mock.Anything, disabled).Return(nil).Once()
	e.dal.EXPECT().DeleteUserSessions(mock.Anything, userID).Return(nil).Once()
	e.dal.EXPECT().ListUsers(mock.Anything).Return([]authsvc.UserSummary{{User: disabled}}, nil).Once()
	_, err = svc.ChangeUser(context.Background(), userID, authsvc.UserChange{Disabled: ptr(true)})
	require.NoError(t, err)

	clock = clock.Add(time.Second) // well inside SessionCacheTTL
	e.dal.EXPECT().SessionByID(mock.Anything, sess.ID).Return(nil, notFound()).Once()
	_, err = svc.Authenticate(context.Background(), "tok")
	require.ErrorIs(t, err, authsvc.ErrUnauthenticated)
}
