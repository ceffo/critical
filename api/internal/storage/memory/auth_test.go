package memory_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/authsvc"
	"github.com/ceffo/critical/api/internal/storage/memory"
)

// The e2e mode's login: the dev login on an empty store creates the root
// admin, its token authenticates, logout ends it (ADR 0029 steps 4 and 8.1).
func TestAuth_DevLoginSessionLifecycle(t *testing.T) {
	ctx := context.Background()
	m := memory.New()
	svc, err := authsvc.New(authsvc.Deps{DAL: m.Auth(), Tx: m})
	require.NoError(t, err)

	s, err := svc.DevLogin(ctx, "")
	require.NoError(t, err)
	assert.True(t, s.User.Root)
	assert.Equal(t, authsvc.DevEmail, s.User.Email)

	p, err := svc.Authenticate(ctx, s.Token)
	require.NoError(t, err)
	assert.Equal(t, s.User.ID, p.User.ID)

	again, err := svc.DevLogin(ctx, "")
	require.NoError(t, err)
	assert.Equal(t, s.User.ID, again.User.ID, "the same root admin")

	require.NoError(t, svc.Logout(ctx, s.Token))
	_, err = svc.Authenticate(ctx, s.Token)
	require.ErrorIs(t, err, authsvc.ErrUnauthenticated)
	_, err = svc.Authenticate(ctx, again.Token)
	require.NoError(t, err, "logout ends one session only")
}

// The store refuses what the schema refuses: a second root, a disabled root,
// a second user of an email; a failed transaction leaves nothing.
func TestAuth_ConstraintsAndRollback(t *testing.T) {
	ctx := context.Background()
	m := memory.New()
	a := m.Auth()
	now := time.Now()
	root := authsvc.User{ID: uuid.New(), Email: "root@example.com", Role: authsvc.RoleAdmin, Root: true, CreatedAt: now}
	require.NoError(t, a.InsertUser(ctx, root))

	require.Error(t, a.InsertUser(ctx, authsvc.User{ID: uuid.New(), Email: "b@example.com", Role: authsvc.RoleAdmin, Root: true}), "a second root")
	require.Error(t, a.InsertUser(ctx, authsvc.User{ID: uuid.New(), Email: "root@example.com", Role: authsvc.RolePlayer}), "a duplicate email")
	disabled := root
	disabled.Disabled = true
	require.Error(t, a.UpdateUser(ctx, disabled), "a disabled root")

	boom := errors.New("boom")
	err := m.InTx(ctx, func(ctx context.Context) error {
		require.NoError(t, a.InsertUser(ctx, authsvc.User{ID: uuid.New(), Email: "c@example.com", Role: authsvc.RolePlayer}))
		return boom
	})
	require.ErrorIs(t, err, boom)
	_, err = a.UserByEmail(ctx, "c@example.com")
	require.ErrorIs(t, err, domain.ErrNotFound)
}

// Users and pending invites created in one instant list in email order, never
// in the order of their random ids, so the admin lists read the same on every
// run (the e2e determinism of ADR 0025 step 8.5).
func TestAuth_Step8_5_ListsTieBreakByEmailNotRandomId(t *testing.T) {
	ctx := context.Background()
	m := memory.New()
	a := m.Auth()
	at := time.Now()
	// Ids in the reverse of the email order: an id tie-break would list "z" first.
	hi := uuid.MustParse("ffffffff-0000-0000-0000-000000000000")
	lo := uuid.MustParse("00000000-0000-0000-0000-000000000000")
	require.NoError(t, a.InsertUser(ctx, authsvc.User{ID: hi, Email: "a@example.com", Role: authsvc.RoleAdmin, Root: true, CreatedAt: at}))
	require.NoError(t, a.InsertUser(ctx, authsvc.User{ID: lo, Email: "z@example.com", Role: authsvc.RolePlayer, CreatedAt: at}))
	users, err := a.ListUsers(ctx)
	require.NoError(t, err)
	require.Len(t, users, 2)
	assert.Equal(t, []string{"a@example.com", "z@example.com"}, []string{users[0].Email, users[1].Email})

	for _, in := range []struct {
		id    uuid.UUID
		email string
	}{{hi, "m@example.com"}, {lo, "n@example.com"}} {
		require.NoError(t, a.InsertInvite(ctx, authsvc.Invite{
			ID: in.id, Email: in.email, InvitedBy: hi, CreatedAt: at, Language: authsvc.LanguageEnglish,
		}))
	}
	invites, err := a.ListPendingInvites(ctx)
	require.NoError(t, err)
	require.Len(t, invites, 2)
	assert.Equal(t, []string{"m@example.com", "n@example.com"}, []string{invites[0].Email, invites[1].Email})
}
