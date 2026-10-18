package authsvc

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/ceffo/critical/api/internal/domain"
)

// Authenticate implements SVC (ADR 0029 steps 4.3, 4.4 and 9). A session
// lookup is a read: it takes no lock; only the lazy delete of an expired or
// disabled user's session and the extension write.
//
// A session found valid is remembered for SessionCacheTTL (never past its
// expiry): the requests of a page load in that window read neither the
// session nor its user from the database.
func (s *service) Authenticate(ctx context.Context, token string) (*Principal, error) {
	if token == "" {
		return nil, ErrUnauthenticated
	}
	id := SessionID(token)
	now := s.now()
	if p, ok := s.sessions.get(id, now); ok {
		return p, nil
	}
	// Taken before the reads: a logout, disable or role change that ends
	// while they run moves it on, and the answer read before is not kept.
	gen := s.sessions.generation()
	p, err := s.authenticate(ctx, id, now)
	if err != nil {
		return nil, err
	}
	s.sessions.put(id, p, now, gen)
	return p, nil
}

// authenticate reads the session id and its user from the database.
func (s *service) authenticate(ctx context.Context, id string, now time.Time) (*Principal, error) {
	sess, err := s.dal.SessionByID(ctx, id)
	if errors.Is(err, domain.ErrNotFound) {
		return nil, ErrUnauthenticated
	}
	if err != nil {
		return nil, err
	}
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
	id := SessionID(token)
	if err := s.tx.InTx(ctx, func(ctx context.Context) error { return s.dal.DeleteSession(ctx, id) }); err != nil {
		return err
	}
	s.sessions.forget(id)
	return nil
}

// SessionCacheTTL is how long a server trusts a session it found valid
// without reading it again (ADR 0029 step 4.7). The server that logs a session
// out, disables its user or changes the user's role forgets it at once; another
// server (production runs two) refuses it, or sees the new role, at most this
// long after the change.
const SessionCacheTTL = 10 * time.Second

// sessionCacheSweep is the size above which a put first drops the entries
// already past their time.
const sessionCacheSweep = 256

// sessionCache holds the principals of recently authenticated sessions, by
// session id.
type sessionCache struct {
	mu   sync.Mutex
	byID map[string]cachedSession
	// gen moves on at every forget, so a principal read before a change and
	// put after it is not remembered.
	gen uint64
}

type cachedSession struct {
	p     Principal
	until time.Time
}

// get returns a copy of the principal of id while its entry is valid; an
// answer from the cache never extends the session (Extended is false).
func (c *sessionCache) get(id string, now time.Time) (*Principal, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.byID[id]
	if !ok || !now.Before(e.until) {
		return nil, false
	}
	p := e.p
	return &p, true
}

// generation is the cache's current generation, to hand to put.
func (c *sessionCache) generation() uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.gen
}

// put remembers p for SessionCacheTTL from now, never past the session's
// expiry, unless something was forgotten since gen was taken: p may then
// predate that change.
func (c *sessionCache) put(id string, p *Principal, now time.Time, gen uint64) {
	until := now.Add(SessionCacheTTL)
	if p.Expires.Before(until) {
		until = p.Expires
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if gen != c.gen {
		return
	}
	if c.byID == nil {
		c.byID = map[string]cachedSession{}
	}
	if len(c.byID) >= sessionCacheSweep {
		for k, e := range c.byID {
			if !now.Before(e.until) {
				delete(c.byID, k)
			}
		}
	}
	e := cachedSession{p: *p, until: until}
	e.p.Extended = false
	c.byID[id] = e
}

// forget drops the session id.
func (c *sessionCache) forget(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.gen++
	delete(c.byID, id)
}

// forgetUser drops every session of the user.
func (c *sessionCache) forgetUser(userID uuid.UUID) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.gen++
	for k, e := range c.byID {
		if e.p.User.ID == userID {
			delete(c.byID, k)
		}
	}
}

// forgetAll drops every session.
func (c *sessionCache) forgetAll() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.gen++
	clear(c.byID)
}
