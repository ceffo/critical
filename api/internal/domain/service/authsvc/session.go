package authsvc

import (
	"context"
	"errors"

	"github.com/ceffo/critical/api/internal/domain"
)

// Authenticate implements SVC (ADR 0029 steps 4.3, 4.4 and 9). A session
// lookup is a read: it takes no lock; only the lazy delete of an expired or
// disabled user's session and the extension write.
func (s *service) Authenticate(ctx context.Context, token string) (*Principal, error) {
	if token == "" {
		return nil, ErrUnauthenticated
	}
	id := SessionID(token)
	sess, err := s.dal.SessionByID(ctx, id)
	if errors.Is(err, domain.ErrNotFound) {
		return nil, ErrUnauthenticated
	}
	if err != nil {
		return nil, err
	}
	now := s.now()
	// Step 4.4: an expired session is refused and removed lazily.
	if !now.Before(sess.Expires) {
		if err := s.tx.InTx(ctx, func(ctx context.Context) error { return s.dal.DeleteSession(ctx, id) }); err != nil {
			return nil, err
		}
		return nil, ErrUnauthenticated
	}
	u, err := s.dal.UserByID(ctx, sess.UserID)
	if errors.Is(err, domain.ErrNotFound) {
		return nil, ErrUnauthenticated
	}
	if err != nil {
		return nil, err
	}
	// Disabling deletes the sessions in its transaction; a session read just
	// before that commit is still refused here, and dropped.
	if u.Disabled {
		if err := s.tx.InTx(ctx, func(ctx context.Context) error { return s.dal.DeleteUserSessions(ctx, u.ID) }); err != nil {
			return nil, err
		}
		return nil, ErrUnauthenticated
	}
	p := &Principal{User: *u, Expires: sess.Expires}
	// Step 4.3: extended while used, at most once per ExtendAfter.
	if now.Sub(sess.LastUsed) >= ExtendAfter {
		expires := now.Add(SessionLifetime)
		if err := s.tx.InTx(ctx, func(ctx context.Context) error {
			return s.dal.ExtendSession(ctx, id, now, expires)
		}); err != nil {
			return nil, err
		}
		p.Expires, p.Extended = expires, true
	}
	return p, nil
}

// Logout implements SVC (ADR 0029 step 4.4).
func (s *service) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	return s.tx.InTx(ctx, func(ctx context.Context) error { return s.dal.DeleteSession(ctx, SessionID(token)) })
}
