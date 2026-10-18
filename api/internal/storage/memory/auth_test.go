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
