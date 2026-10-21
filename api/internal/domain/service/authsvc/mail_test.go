package authsvc_test

// The mails of the invites (ADR 0029 steps 2.5 to 2.9) on a fake notifier.

import (
	"context"
	"errors"
	"sync"
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

// fakeNotifier records the notices; Err is what it returns, Hang makes it wait
// for its context (a provider that never answers).
type fakeNotifier struct {
	mu   sync.Mutex
	got  []authsvc.Notice
	Err  error
	Hang bool
}

func (f *fakeNotifier) Notify(ctx context.Context, n authsvc.Notice) error {
	f.mu.Lock()
	f.got = append(f.got, n)
	f.mu.Unlock()
	if f.Hang {
		<-ctx.Done()
		return ctx.Err()
	}
	return f.Err
}

func (f *fakeNotifier) notices() []authsvc.Notice {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]authsvc.Notice(nil), f.got...)
}

// newMailEnv is a Google-mode service with n as its notifier and a short mail
// timeout.
func newMailEnv(t *testing.T, n authsvc.Notifier) (authsvc.SVC, *mocks.MockDAL) {
	t.Helper()
	dal := mocks.NewMockDAL(t)
	idp := mocks.NewMockIdentityProvider(t)
	svc, err := authsvc.New(authsvc.Deps{
		DAL: dal, Tx: &tx{}, Provider: idp, Notifier: n, MailTimeout: 50 * time.Millisecond,
		Now: func() time.Time { return now }, NewID: func() uuid.UUID { return newID },
		NewToken: func() (string, error) { return "tok", nil },
	})
	require.NoError(t, err)
	return svc, dal
}

// expectNewInvite names the storage calls of creating an invite for cid.
func expectNewInvite(dal *mocks.MockDAL, lang authsvc.Language) {
	dal.EXPECT().UserByEmail(mock.Anything, "cid@example.com").Return(nil, notFound()).Once()
	dal.EXPECT().PendingInvite(mock.Anything, "cid@example.com").Return(nil, notFound()).Once()
	dal.EXPECT().InsertInvite(mock.Anything, authsvc.Invite{
		ID: newID, Email: "cid@example.com", InvitedBy: otherID, CreatedAt: now, Language: lang,
	}).Return(nil).Once()
}

func inviter(dal *mocks.MockDAL) {
	dal.EXPECT().UserByID(mock.Anything, otherID).Return(&authsvc.User{ID: otherID, Email: "ann@example.com", DisplayName: "Ann Admin"}, nil)
}

// Step 2.6: the invite stores the language the admin picked; none is English;
// anything but en or fr is a field error on "language" and writes nothing.
func TestCreateInvite_Step2_6_LanguageIsStored(t *testing.T) {
	for _, tc := range []struct{ in, want authsvc.Language }{
		{"fr", authsvc.LanguageFrench}, {"en", authsvc.LanguageEnglish}, {"", authsvc.LanguageEnglish},
	} {
		t.Run(string(tc.in), func(t *testing.T) {
			svc, dal := newMailEnv(t, nil)
			expectNewInvite(dal, tc.want)
			inv, _, err := svc.CreateInvite(context.Background(), otherID, "cid@example.com", tc.in)
			require.NoError(t, err)
			assert.Equal(t, tc.want, inv.Language)
		})
	}
	t.Run("refused", func(t *testing.T) {
		svc, _ := newMailEnv(t, nil)
		_, _, err := svc.CreateInvite(context.Background(), otherID, "cid@example.com", "de")
		require.ErrorIs(t, err, domain.ErrInvalidInput)
		var fe *domain.FieldError
		require.ErrorAs(t, err, &fe)
		assert.Equal(t, "language", fe.Field)
	})
}

// Step 2.5: creating an invite sends one mail to the invitee in the invite's
// language, naming the inviter (name and email); the outcome is sent.
func TestCreateInvite_Step2_5_MailsTheInvitee(t *testing.T) {
	n := &fakeNotifier{}
	svc, dal := newMailEnv(t, n)
	expectNewInvite(dal, authsvc.LanguageFrench)
	inviter(dal)

	_, outcome, err := svc.CreateInvite(context.Background(), otherID, "cid@example.com", "fr")
	require.NoError(t, err)
	assert.Equal(t, authsvc.MailSent, outcome)
	assert.Equal(t, []authsvc.Notice{{
		Kind: authsvc.NoticeInvite, To: "cid@example.com", Language: authsvc.LanguageFrench,
		InviterEmail: "ann@example.com", InviterName: "Ann Admin", InviteeEmail: "cid@example.com",
	}}, n.notices())
}

// Step 2.8: whatever the mail does, the invite is created and the call
// succeeds; the outcome says what happened. A refused invite sends nothing.
func TestCreateInvite_Step2_8_TheInviteSurvivesTheMail(t *testing.T) {
	for _, tc := range []struct {
		name string
		n    *fakeNotifier
		want authsvc.MailOutcome
	}{
		{"an error", &fakeNotifier{Err: errors.New("provider down")}, authsvc.MailFailed},
		{"a timeout", &fakeNotifier{Hang: true}, authsvc.MailFailed},
		{"not configured", &fakeNotifier{Err: authsvc.ErrMailNotConfigured}, authsvc.MailNotConfigured},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, dal := newMailEnv(t, tc.n)
			expectNewInvite(dal, authsvc.LanguageEnglish) // InsertInvite is expected once: no rollback, no second try
			inviter(dal)
			start := time.Now()
			inv, outcome, err := svc.CreateInvite(context.Background(), otherID, "cid@example.com", "en")
			require.NoError(t, err)
			require.NotNil(t, inv)
			assert.Equal(t, tc.want, outcome)
			assert.Len(t, tc.n.notices(), 1, "one try, no retry")
			assert.Less(t, time.Since(start), 2*time.Second, "never blocks on the mailer")
		})
	}
	t.Run("a nil notifier is not configured", func(t *testing.T) {
		svc, dal := newMailEnv(t, nil)
		expectNewInvite(dal, authsvc.LanguageEnglish)
		_, outcome, err := svc.CreateInvite(context.Background(), otherID, "cid@example.com", "en")
		require.NoError(t, err)
		assert.Equal(t, authsvc.MailNotConfigured, outcome)
	})
	t.Run("a refused invite sends nothing", func(t *testing.T) {
		n := &fakeNotifier{}
		svc, dal := newMailEnv(t, n)
		dal.EXPECT().UserByEmail(mock.Anything, "cid@example.com").Return(&authsvc.User{ID: userID}, nil).Once()
		_, _, err := svc.CreateInvite(context.Background(), otherID, "cid@example.com", "en")
		require.ErrorIs(t, err, authsvc.ErrEmailHasUser)
		assert.Empty(t, n.notices())
	})
}

// Step 2.7: a resend mails a pending invite again, the same mail; a cancelled
// or used invite is refused, an unknown one is not found; nothing is sent.
func TestResendInvite_Step2_7(t *testing.T) {
	pending := authsvc.Invite{ID: newID, Email: "cid@example.com", InvitedBy: otherID, Language: authsvc.LanguageFrench}
	t.Run("pending", func(t *testing.T) {
		n := &fakeNotifier{}
		svc, dal := newMailEnv(t, n)
		dal.EXPECT().InviteByID(mock.Anything, newID).Return(&pending, nil).Once()
		inviter(dal)
		_, outcome, err := svc.ResendInvite(context.Background(), newID)
		require.NoError(t, err)
		assert.Equal(t, authsvc.MailSent, outcome)
		require.Len(t, n.notices(), 1)
		assert.Equal(t, authsvc.NoticeInvite, n.notices()[0].Kind)
		assert.Equal(t, authsvc.LanguageFrench, n.notices()[0].Language)
	})
	t.Run("failed mail is an outcome, not an error", func(t *testing.T) {
		svc, dal := newMailEnv(t, &fakeNotifier{Err: errors.New("down")})
		dal.EXPECT().InviteByID(mock.Anything, newID).Return(&pending, nil).Once()
		inviter(dal)
		_, outcome, err := svc.ResendInvite(context.Background(), newID)
		require.NoError(t, err)
		assert.Equal(t, authsvc.MailFailed, outcome)
	})
	for name, inv := range map[string]authsvc.Invite{
		"cancelled": {ID: newID, Cancelled: true},
		"used":      {ID: newID, Accepted: true},
	} {
		t.Run(name, func(t *testing.T) {
			n := &fakeNotifier{}
			svc, dal := newMailEnv(t, n)
			dal.EXPECT().InviteByID(mock.Anything, newID).Return(&inv, nil).Once()
			_, _, err := svc.ResendInvite(context.Background(), newID)
			require.ErrorIs(t, err, authsvc.ErrInviteNotPending)
			require.ErrorIs(t, err, domain.ErrConflict, "a 409")
			assert.Empty(t, n.notices())
		})
	}
	t.Run("unknown", func(t *testing.T) {
		svc, dal := newMailEnv(t, &fakeNotifier{})
		dal.EXPECT().InviteByID(mock.Anything, newID).Return(nil, notFound()).Once()
		_, _, err := svc.ResendInvite(context.Background(), newID)
		require.ErrorIs(t, err, authsvc.ErrInviteNotFound)
	})
}

// expectInvitedLogin names the storage calls of ann's first login with a
// pending invite of otherID in the given language.
func expectInvitedLogin(dal *mocks.MockDAL, idp *mocks.MockIdentityProvider, lang authsvc.Language) {
	named := verified
	named.Name = "Ann Lee"
	idp.EXPECT().Exchange(mock.Anything, "code", "ver").Return(&named, nil).Once()
	inviteID := uuid.New()
	dal.EXPECT().UserBySubject(mock.Anything, "g-123").Return(nil, notFound()).Once()
	dal.EXPECT().UserByEmail(mock.Anything, "ann@example.com").Return(nil, notFound()).Once()
	dal.EXPECT().PendingInvite(mock.Anything, "ann@example.com").Return(&authsvc.Invite{
		ID: inviteID, Email: "ann@example.com", InvitedBy: otherID, Language: authsvc.LanguageFrench,
	}, nil).Once()
	dal.EXPECT().InsertUser(mock.Anything, mock.Anything).Return(nil).Once()
	dal.EXPECT().InsertSession(mock.Anything, mock.Anything).Return(nil).Once()
	dal.EXPECT().AcceptInvite(mock.Anything, inviteID, newID, now).Return(nil).Once()
	dal.EXPECT().UserByID(mock.Anything, otherID).Return(&authsvc.User{ID: otherID, Email: "root@example.com", DisplayName: "Root Admin"}, nil).Once()
}

// Step 2.9: the first login that consumes an invite tells the inviter, once,
// in French whatever the invite's language; a later login sends nothing; a failing mail never
// stops the login.
func TestFinishLogin_Step2_9_TellsTheInviter(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		lang authsvc.Language
	}{
		{"mail works, French invite", nil, authsvc.LanguageFrench},
		{"mail works, English invite", nil, authsvc.LanguageEnglish},
		{"mail fails", errors.New("provider down"), authsvc.LanguageEnglish},
	} {
		t.Run(tc.name, func(t *testing.T) {
			n := &fakeNotifier{Err: tc.err}
			dal := mocks.NewMockDAL(t)
			idp := mocks.NewMockIdentityProvider(t)
			svc, err := authsvc.New(authsvc.Deps{
				DAL: dal, Tx: &tx{}, Provider: idp, Notifier: n,
				Now: func() time.Time { return now }, NewID: func() uuid.UUID { return newID },
				NewToken: func() (string, error) { return "tok", nil },
			})
			require.NoError(t, err)
			expectInvitedLogin(dal, idp, tc.lang)

			got, err := svc.FinishLogin(context.Background(), callback())
			require.NoError(t, err, "a mail failure never blocks the login")
			assert.Equal(t, "tok", got.Token)
			require.NoError(t, svc.Close(context.Background())) // waits for the background mail
			assert.Equal(t, []authsvc.Notice{{
				Kind: authsvc.NoticeAccepted, To: "root@example.com", Language: authsvc.LanguageFrench,
				InviterEmail: "root@example.com", InviterName: "Root Admin", InviteeEmail: "ann@example.com", InviteeName: "Ann Lee",
			}}, n.notices())
		})
	}
	t.Run("a known subject sends nothing", func(t *testing.T) {
		n := &fakeNotifier{}
		dal := mocks.NewMockDAL(t)
		idp := mocks.NewMockIdentityProvider(t)
		svc, err := authsvc.New(authsvc.Deps{
			DAL: dal, Tx: &tx{}, Provider: idp, Notifier: n,
			Now: func() time.Time { return now }, NewID: func() uuid.UUID { return newID },
			NewToken: func() (string, error) { return "tok", nil },
		})
		require.NoError(t, err)
		idp.EXPECT().Exchange(mock.Anything, "code", "ver").Return(&verified, nil).Once()
		dal.EXPECT().UserBySubject(mock.Anything, "g-123").Return(&authsvc.User{ID: userID, Email: "ann@example.com"}, nil).Once()
		dal.EXPECT().UpdateUser(mock.Anything, mock.Anything).Return(nil).Once()
		dal.EXPECT().InsertSession(mock.Anything, mock.Anything).Return(nil).Once()
		_, err = svc.FinishLogin(context.Background(), callback())
		require.NoError(t, err)
		require.NoError(t, svc.Close(context.Background()))
		assert.Empty(t, n.notices())
	})
}

// Close stops new background mails (a mail after Close is dropped) and honours
// its context while a mail is still in flight.
func TestClose_Step2_9_DropsLateMailsAndHonoursItsContext(t *testing.T) {
	n := &fakeNotifier{Hang: true}
	dal := mocks.NewMockDAL(t)
	idp := mocks.NewMockIdentityProvider(t)
	svc, err := authsvc.New(authsvc.Deps{
		DAL: dal, Tx: &tx{}, Provider: idp, Notifier: n, MailTimeout: time.Minute,
		Now: func() time.Time { return now }, NewID: func() uuid.UUID { return newID },
		NewToken: func() (string, error) { return "tok", nil },
	})
	require.NoError(t, err)
	expectInvitedLogin(dal, idp, authsvc.LanguageFrench)
	_, err = svc.FinishLogin(context.Background(), callback())
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	assert.ErrorIs(t, svc.Close(ctx), context.DeadlineExceeded, "the hanging mail outlives the context")

	expectInvitedLogin(dal, idp, authsvc.LanguageFrench)
	_, err = svc.FinishLogin(context.Background(), callback())
	require.NoError(t, err)
	assert.Len(t, n.notices(), 1, "a login after Close sends nothing")
}
