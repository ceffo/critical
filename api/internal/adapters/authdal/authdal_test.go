// The adapter runs on a mocked Querier: the tests assert the operations called,
// their arguments and how rows and errors map to domain values; no database,
// no SQL text (ADR 0025 step 6.2).
package authdal_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
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

// A row maps to a user: NULL subject, last login and current character are
// the zero values.
func TestUserReads_MapRowsAndNotFound(t *testing.T) {
	q := mocks.NewMockQuerier(t)
	q.EXPECT().UserBySubject(ctx, sql.NullString{String: "g-1", Valid: true}).Return(queries.UserBySubjectRow{
		ID: uid, Email: "ann@example.com", GoogleSubject: sql.NullString{String: "g-1", Valid: true},
		Role: "admin", IsRoot: true, LastLogin: sql.NullTime{Time: at, Valid: true}, CreatedAt: at,
		CurrentCharacterID: sql.NullInt64{Int64: 7, Valid: true},
	}, nil).Once()
	q.EXPECT().UserByEmail(ctx, "bob@example.com").Return(queries.UserByEmailRow{
		ID: uid, Email: "bob@example.com", Role: "player", Disabled: true, CreatedAt: at,
	}, nil).Once()
	q.EXPECT().UserByID(ctx, uid).Return(queries.UserByIDRow{}, sql.ErrNoRows).Once()
	q.EXPECT().RootUser(ctx).Return(queries.RootUserRow{}, sql.ErrNoRows).Once()
	d := authdal.New(q)

	u, err := d.UserBySubject(ctx, "g-1")
	require.NoError(t, err)
	assert.Equal(t, authsvc.User{
		ID: uid, Email: "ann@example.com", GoogleSubject: "g-1", Role: authsvc.RoleAdmin, Root: true, LastLogin: &at, CreatedAt: at,
		CurrentCharacterID: 7,
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
		ID: iid, Email: "ann@example.com", InvitedBy: uid, CreatedAt: at, Language: "fr",
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
	assert.Equal(t, authsvc.Invite{ID: iid, Email: "ann@example.com", InvitedBy: uid, CreatedAt: at, Language: authsvc.LanguageFrench}, *inv)
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

// Adopting gives the unowned characters to the user, then sets their first
// character current (the query keeps one already set).
func TestAdoptUnownedCharacters_OwnsThenSetsTheFirstCurrent(t *testing.T) {
	q := mocks.NewMockQuerier(t)
	adopt := q.EXPECT().AdoptUnownedCharacters(ctx, uuid.NullUUID{UUID: uid, Valid: true}).Return(nil).Once()
	q.EXPECT().SetFirstCurrentCharacter(ctx, uid).Return(nil).Once().NotBefore(adopt)
	require.NoError(t, authdal.New(q).AdoptUnownedCharacters(ctx, uid))

	q = mocks.NewMockQuerier(t)
	q.EXPECT().AdoptUnownedCharacters(ctx, uuid.NullUUID{UUID: uid, Valid: true}).Return(assert.AnError).Once()
	require.ErrorIs(t, authdal.New(q).AdoptUnownedCharacters(ctx, uid), assert.AnError)
}

// Moving gives every character of one user to another, then clears the first
// one's current character; a failure stops before the clear.
func TestMoveCharacters_ReownsThenClearsTheOldCurrent(t *testing.T) {
	to := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	q := mocks.NewMockQuerier(t)
	move := q.EXPECT().MoveCharacters(ctx, queries.MoveCharactersParams{
		FromUser: uuid.NullUUID{UUID: uid, Valid: true}, ToUser: uuid.NullUUID{UUID: to, Valid: true},
	}).Return(nil).Once()
	q.EXPECT().ClearCurrentCharacter(ctx, uid).Return(nil).Once().NotBefore(move)
	require.NoError(t, authdal.New(q).MoveCharacters(ctx, uid, to))

	q = mocks.NewMockQuerier(t)
	q.EXPECT().MoveCharacters(ctx, mock.Anything).Return(assert.AnError).Once()
	require.ErrorIs(t, authdal.New(q).MoveCharacters(ctx, uid, to), assert.AnError)
}

// The admin list: each user with the names of the characters they own, in
// the order the query reads them; a user without any has an empty list.
func TestListUsers_JoinsTheCharacterNamesByOwner(t *testing.T) {
	q := mocks.NewMockQuerier(t)
	q.EXPECT().ListUsers(ctx).Return([]queries.ListUsersRow{
		{ID: uid, Email: "ann@example.com", Role: "admin", IsRoot: true, LastLogin: sql.NullTime{Time: at, Valid: true}, CreatedAt: at},
		{ID: iid, Email: "bob@example.com", Role: "player", Disabled: true, CreatedAt: at},
	}, nil).Once()
	q.EXPECT().OwnedCharacterNames(ctx).Return([]queries.OwnedCharacterNamesRow{
		{OwnerUserID: uuid.NullUUID{UUID: uid, Valid: true}, Name: "Kelsen"},
		{OwnerUserID: uuid.NullUUID{UUID: uid, Valid: true}, Name: "Vex"},
	}, nil).Once()

	got, err := authdal.New(q).ListUsers(ctx)
	require.NoError(t, err)
	assert.Equal(t, []authsvc.UserSummary{
		{User: authsvc.User{ID: uid, Email: "ann@example.com", Role: authsvc.RoleAdmin, Root: true, LastLogin: &at, CreatedAt: at}, Characters: []string{"Kelsen", "Vex"}},
		{User: authsvc.User{ID: iid, Email: "bob@example.com", Role: authsvc.RolePlayer, Disabled: true, CreatedAt: at}, Characters: []string{}},
	}, got)
}

// Invites: listed, inserted, and cancelled; a cancel that changes no row (the
// invite is not pending) is not found.
func TestInvites_ListInsertCancel(t *testing.T) {
	q := mocks.NewMockQuerier(t)
	q.EXPECT().ListPendingInvites(ctx).Return([]queries.ListPendingInvitesRow{
		{ID: iid, Email: "cid@example.com", InvitedBy: uid, CreatedAt: at, Language: "fr"},
	}, nil).Once()
	q.EXPECT().InsertInvite(ctx, queries.InsertInviteParams{ID: iid, Email: "cid@example.com", InvitedBy: uid, CreatedAt: at, Language: "fr"}).Return(nil).Once()
	q.EXPECT().CancelInvite(ctx, queries.CancelInviteParams{ID: iid, Cancelled: sql.NullTime{Time: at, Valid: true}}).Return(1, nil).Once()
	q.EXPECT().CancelInvite(ctx, queries.CancelInviteParams{ID: uid, Cancelled: sql.NullTime{Time: at, Valid: true}}).Return(0, nil).Once()
	d := authdal.New(q)

	got, err := d.ListPendingInvites(ctx)
	require.NoError(t, err)
	assert.Equal(t, []authsvc.Invite{{ID: iid, Email: "cid@example.com", InvitedBy: uid, CreatedAt: at, Language: authsvc.LanguageFrench}}, got)
	require.NoError(t, d.InsertInvite(ctx, authsvc.Invite{ID: iid, Email: "cid@example.com", InvitedBy: uid, CreatedAt: at, Language: authsvc.LanguageFrench}))
	require.NoError(t, d.CancelInvite(ctx, iid, at))
	require.ErrorIs(t, d.CancelInvite(ctx, uid, at), domain.ErrNotFound)
}

// Step 2.7: an invite is read by id with its cancelled and used flags.
func TestInviteByID_Step2_7(t *testing.T) {
	q := mocks.NewMockQuerier(t)
	q.EXPECT().InviteByID(ctx, iid).Return(queries.InviteByIDRow{
		ID: iid, Email: "cid@example.com", InvitedBy: uid, CreatedAt: at, Language: "fr", IsCancelled: true,
	}, nil).Once()
	q.EXPECT().InviteByID(ctx, uid).Return(queries.InviteByIDRow{}, sql.ErrNoRows).Once()
	d := authdal.New(q)

	got, err := d.InviteByID(ctx, iid)
	require.NoError(t, err)
	assert.Equal(t, authsvc.Invite{ID: iid, Email: "cid@example.com", InvitedBy: uid, CreatedAt: at, Language: authsvc.LanguageFrench, Cancelled: true}, *got)
	_, err = d.InviteByID(ctx, uid)
	require.ErrorIs(t, err, domain.ErrNotFound)
}

func TestInsertInvite_Step2_1_APendingInviteIndexViolationIsAlreadyInvited(t *testing.T) {
	q := mocks.NewMockQuerier(t)
	p := queries.InsertInviteParams{ID: iid, Email: "cid@example.com", InvitedBy: uid, CreatedAt: at, Language: "fr"}
	q.EXPECT().InsertInvite(ctx, p).Return(fmt.Errorf("exec: %w", &pq.Error{Code: "23505", Constraint: "invites_pending_email"})).Once()
	d := authdal.New(q)

	err := d.InsertInvite(ctx, authsvc.Invite{ID: iid, Email: "cid@example.com", InvitedBy: uid, CreatedAt: at, Language: authsvc.LanguageFrench})
	require.ErrorIs(t, err, authsvc.ErrAlreadyInvited)
	assert.ErrorIs(t, err, domain.ErrConflict)
}

func TestInsertInvite_Step2_1_AnotherInsertErrorIsNotAlreadyInvited(t *testing.T) {
	q := mocks.NewMockQuerier(t)
	p := queries.InsertInviteParams{ID: iid, Email: "cid@example.com", InvitedBy: uid, CreatedAt: at, Language: "fr"}
	q.EXPECT().InsertInvite(ctx, p).Return(&pq.Error{Code: "23505", Constraint: "invites_pkey"}).Once()
	d := authdal.New(q)

	err := d.InsertInvite(ctx, authsvc.Invite{ID: iid, Email: "cid@example.com", InvitedBy: uid, CreatedAt: at, Language: authsvc.LanguageFrench})
	require.Error(t, err)
	assert.NotErrorIs(t, err, authsvc.ErrAlreadyInvited)
}
