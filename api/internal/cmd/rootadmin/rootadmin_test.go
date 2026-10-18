package rootadmin

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/service/authsvc"
	"github.com/ceffo/critical/api/internal/domain/service/authsvc/mocks"
)

func opener(svc authsvc.SVC, gotDSN *string) Opener {
	return func(_ context.Context, dsn string) (authsvc.SVC, func() error, error) {
		*gotDSN = dsn
		return svc, func() error { return nil }, nil
	}
}

// ADR 0029 step 5.1: the script sets the root admin by email on the named
// database and says whether it is bound yet.
func TestRun_SetsTheRootAdmin(t *testing.T) {
	svc := mocks.NewMockSVC(t)
	svc.EXPECT().SetRootAdmin(mock.Anything, "me@example.com").Return(&authsvc.User{Email: "me@example.com", Root: true}, nil).Once()
	var dsn string
	var out bytes.Buffer
	require.NoError(t, run(context.Background(), []string{"me@example.com"}, "postgres://x", &out, opener(svc, &dsn)))
	assert.Equal(t, "postgres://x", dsn)
	assert.Contains(t, out.String(), "root admin: me@example.com (not logged in yet")
}

// The script needs an email and an explicit DSN; it opens nothing otherwise.
func TestRun_RefusesWithoutEmailOrDSN(t *testing.T) {
	var dsn string
	open := opener(nil, &dsn)
	require.Error(t, run(context.Background(), nil, "postgres://x", &bytes.Buffer{}, open))
	require.Error(t, run(context.Background(), []string{"me@example.com"}, "", &bytes.Buffer{}, open))
	assert.Empty(t, dsn, "nothing opened")
}

// Step 5.1: moving a bound root is refused with the service's error.
func TestRun_RootTaken(t *testing.T) {
	svc := mocks.NewMockSVC(t)
	svc.EXPECT().SetRootAdmin(mock.Anything, "other@example.com").Return(nil, authsvc.ErrRootTaken).Once()
	var dsn string
	err := run(context.Background(), []string{"other@example.com"}, "postgres://x", &bytes.Buffer{}, opener(svc, &dsn))
	require.ErrorIs(t, err, authsvc.ErrRootTaken)
}
