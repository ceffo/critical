// The adapter runs on a mocked Querier: the tests assert the operations called,
// their arguments and how rows and errors map to domain values; no database,
// no SQL text (ADR 0025 step 6.2).
package authdal_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/authdal"
	"github.com/ceffo/critical/api/internal/adapters/authdal/queries"
	"github.com/ceffo/critical/api/internal/adapters/authdal/queries/mocks"
	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/authsvc"
)

var (
	ctx = context.Background()
	at  = time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	uid = uuid.MustParse("11111111-1111-1111-1111-111111111111")
	iid = uuid.MustParse("22222222-2222-2222-2222-222222222222")
)

// A row maps to a user: NULL subject and last login are the zero values.
func TestUserReads_MapRowsAndNotFound(t *testing.T) {
	q := mocks.NewMockQuerier(t)
	q.EXPECT().UserBySubject(ctx, sql.NullString{String: "g-1", Valid: true}).Return(queries.User{
		ID: uid, Email: "ann@example.com", GoogleSubject: sql.NullString{String: "g-1", Valid: true},
		Role: "admin", IsRoot: true, LastLogin: sql.NullTime{Time: at, Valid: true}, CreatedAt: at,
	}, nil).Once()
	q.EXPECT().UserByEmail(ctx, "bob@example.com").Return(queries.User{
		ID: uid, Email: "bob@example.com", Role: "player", Disabled: true, CreatedAt: at,
	}, nil).Once()
	q.EXPECT().UserByID(ctx, uid).Return(queries.User{}, sql.ErrNoRows).Once()
	q.EXPECT().RootUser(ctx).Return(queries.User{}, sql.ErrNoRows).Once()
	d := authdal.New(q)

	u, err := d.UserBySubject(ctx, "g-1")
	require.NoError(t, err)
	assert.Equal(t, authsvc.User{
		ID: uid, Email: "ann@example.com", GoogleSubject: "g-1", Role: authsvc.RoleAdmin, Root: true, LastLogin: &at, CreatedAt: at,
	}, *u)

	u, err = d.UserByEmail(ctx, "bob@example.com")
	require.NoError(t, err)
	assert.Equal(t, authsvc.User{ID: uid, Email: "bob@example.com", Role: authsvc.RolePlayer, Disabled: true, CreatedAt: at}, *u)

	_, err = d.UserByID(ctx, uid)
	require.ErrorIs(t, err, domain.ErrNotFound)
	_, err = d.RootUser(ctx)
	require.ErrorIs(t, err, domain.ErrNotFound)
}

// Writes pass every field; an empty subject and no last login are NULL; an
// update of no row is not found.
func TestUserWrites_PassEveryField(t *testing.T) {
	q := mocks.NewMockQuerier(t)
	q.EXPECT().InsertUser(ctx, queries.InsertUserParams{
		ID: uid, Email: "ann@example.com", Role: "admin", IsRoot: true, CreatedAt: at,
	}).Return(nil).Once()
	q.EXPECT().UpdateUser(ctx, queries.UpdateUserParams{
		ID: uid, GoogleSubject: sql.NullString{String: "g-1", Valid: true}, Role: "player", Disabled: true,
		LastLogin: sql.NullTime{Time: at, Valid: true},
	}).Return(1, nil).Once()
	q.EXPECT().UpdateUser(ctx, queries.UpdateUserParams{ID: iid, Role: "player"}).Return(0, nil).Once()
	d := authdal.New(q)

	require.NoError(t, d.InsertUser(ctx, authsvc.User{ID: uid, Email: "ann@example.com", Role: authsvc.RoleAdmin, Root: true, CreatedAt: at}))
	require.NoError(t, d.UpdateUser(ctx, authsvc.User{
		ID: uid, Email: "ann@example.com", GoogleSubject: "g-1", Role: authsvc.RolePlayer, Disabled: true, LastLogin: &at,
	}))
	require.ErrorIs(t, d.UpdateUser(ctx, authsvc.User{ID: iid, Role: authsvc.RolePlayer}), domain.ErrNotFound)
}

// The pending invite and its acceptance; an invite no longer pending is not
// found.
func TestInvites_PendingAndAccept(t *testing.T) {
	q := mocks.NewMockQuerier(t)
	q.EXPECT().PendingInvite(ctx, "ann@example.com").Return(queries.PendingInviteRow{
		ID: iid, Email: "ann@example.com", InvitedBy: uid, CreatedAt: at,
	}, nil).Once()
	q.EXPECT().PendingInvite(ctx, "bob@example.com").Return(queries.PendingInviteRow{}, sql.ErrNoRows).Once()
	accept := queries.AcceptInviteParams{
		ID: iid, AcceptedBy: uuid.NullUUID{UUID: uid, Valid: true}, AcceptedAt: sql.NullTime{Time: at, Valid: true},
	}
	q.EXPECT().AcceptInvite(ctx, accept).Return(1, nil).Once()
	q.EXPECT().AcceptInvite(ctx, accept).Return(0, nil).Once()
	d := authdal.New(q)

	inv, err := d.PendingInvite(ctx, "ann@example.com")
	require.NoError(t, err)
	assert.Equal(t, authsvc.Invite{ID: iid, Email: "ann@example.com", InvitedBy: uid, CreatedAt: at}, *inv)
	_, err = d.PendingInvite(ctx, "bob@example.com")
	require.ErrorIs(t, err, domain.ErrNotFound)

	require.NoError(t, d.AcceptInvite(ctx, iid, uid, at))
	require.ErrorIs(t, d.AcceptInvite(ctx, iid, uid, at), domain.ErrNotFound)
}

// Sessions: insert, read, extend and the two deletes.
func TestSessions_RoundTrip(t *testing.T) {
	q := mocks.NewMockQuerier(t)
	exp := at.Add(time.Hour)
	q.EXPECT().InsertSession(ctx, queries.InsertSessionParams{ID: "h", UserID: uid, CreatedAt: at, LastUsed: at, ExpiresAt: exp}).Return(nil).Once()
	q.EXPECT().SessionByID(ctx, "h").Return(queries.Session{ID: "h", UserID: uid, CreatedAt: at, LastUsed: at, ExpiresAt: exp}, nil).Once()
	q.EXPECT().SessionByID(ctx, "x").Return(queries.Session{}, sql.ErrNoRows).Once()
	q.EXPECT().ExtendSession(ctx, queries.ExtendSessionParams{ID: "h", LastUsed: exp, ExpiresAt: exp}).Return(nil).Once()
	q.EXPECT().DeleteSession(ctx, "h").Return(nil).Once()
	q.EXPECT().DeleteUserSessions(ctx, uid).Return(nil).Once()
	d := authdal.New(q)

	s := authsvc.Session{ID: "h", UserID: uid, CreatedAt: at, LastUsed: at, Expires: exp}
	require.NoError(t, d.InsertSession(ctx, s))
	got, err := d.SessionByID(ctx, "h")
	require.NoError(t, err)
	assert.Equal(t, s, *got)
	_, err = d.SessionByID(ctx, "x")
	require.ErrorIs(t, err, domain.ErrNotFound)
	require.NoError(t, d.ExtendSession(ctx, "h", exp, exp))
	require.NoError(t, d.DeleteSession(ctx, "h"))
	require.NoError(t, d.DeleteUserSessions(ctx, uid))
}
