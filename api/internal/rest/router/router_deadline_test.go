package router_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/service/skillsvc"
	"github.com/ceffo/critical/api/internal/rest/router"
)

// Every request reaches its service with a deadline of RequestDeadline at
// most: a slow or stopped database answers the click with an error then,
// instead of holding it.
func TestRouter_EveryRequestHasADeadline(t *testing.T) {
	h, m := newRouter(t)
	var deadline time.Time
	var ok bool
	m.skills.EXPECT().List(mock.Anything, mock.Anything).RunAndReturn(func(ctx context.Context, _ int64) (*skillsvc.Skills, error) {
		deadline, ok = ctx.Deadline()
		return &skillsvc.Skills{}, nil
	}).Once()
	started := time.Now()
	rec := do(h, http.MethodGet, "/api/skills", "")
	require.Equal(t, http.StatusOK, rec.Code)
	require.True(t, ok, "the request's context has a deadline")
	assert.WithinDuration(t, started.Add(router.RequestDeadline), deadline, time.Second)
	assert.LessOrEqual(t, router.RequestDeadline, 10*time.Second, "a few seconds, not tens")
}
