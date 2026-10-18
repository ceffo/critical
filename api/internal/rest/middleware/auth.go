package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ceffo/critical/api/internal/domain/service/authsvc"
	"github.com/ceffo/critical/api/internal/render"
)

// SessionCookie is the name of the session cookie (ADR 0029 step 4.2). It
// holds the session's random token and nothing about the user.
const SessionCookie = "critical_session"

// LoginPath is the login page, the one public page (ADR 0029 step 6.1): a
// page request without a session is redirected there, and it and everything
// under it (its own assets) is served without one.
const LoginPath = "/login"

// Authenticator resolves a session token (authsvc.SVC satisfies it).
type Authenticator interface {
	Authenticate(ctx context.Context, token string) (*authsvc.Principal, error)
}

// GateConfig is how the gate checks and sets cookies.
type GateConfig struct {
	// Host is the application's own host (the public URL's): the CSRF check
	// accepts state-changing requests whose Origin names it. Empty: the
	// request's own Host (local dev, where the app answers on any address).
	Host string
	// Secure marks cookies Secure (HTTPS; ADR 0029 step 4.2).
	Secure bool
}

// Public reports whether path is served without a session (ADR 0029 step
// 6.1): the login page and what is under it, the /api/auth endpoints and
// /api/health. Everything else needs one (step 6.2): a new route is protected
// by default.
func Public(path string) bool {
	return path == "/api/health" ||
		path == "/api/auth" || strings.HasPrefix(path, "/api/auth/") ||
		path == LoginPath || strings.HasPrefix(path, LoginPath+"/")
}

func isAPI(path string) bool { return path == "/api" || strings.HasPrefix(path, "/api/") }

// safeMethod reports a method that changes no state (every mutation is a
// non-GET, ADR 0029 step 4.5).
func safeMethod(m string) bool {
	return m == http.MethodGet || m == http.MethodHead || m == http.MethodOptions
}

// Gate is the one access check of the router (ADR 0029 step 6.2). It refuses
// a cross-origin state-changing request (step 4.5), resolves the session
// cookie to the request's principal (read once, passed on in the context,
// step 6.3; an extended session gets its cookie again), and without a
// session redirects a page to the login page and answers 401 to everything
// else (/api, the bundle's assets), except on the public paths. A nil auth authenticates nobody.
func Gate(auth Authenticator, cfg GateConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			if !safeMethod(r.Method) && !sameOrigin(r, cfg.Host) {
				render.Error(ctx, w, authsvc.ErrCrossOrigin)
				return
			}
			p, err := principal(r, auth)
			switch {
			case errors.Is(err, authsvc.ErrUnauthenticated):
				ClearSessionCookie(w, cfg.Secure)
			case err != nil:
				render.Error(ctx, w, err)
				return
			case p != nil && p.Extended:
				SetSessionCookie(w, cookieToken(r), p.Expires, cfg.Secure)
			}
			if p != nil {
				r = r.WithContext(WithPrincipal(ctx, p))
			}
			if p == nil && !Public(r.URL.Path) {
				if !isAPI(r.URL.Path) && isNavigation(r) {
					http.Redirect(w, r, LoginPath, http.StatusFound)
					return
				}
				render.Error(ctx, w, authsvc.ErrUnauthenticated)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// isNavigation reports a browser opening a page (a GET asking for HTML),
// which goes to the login page; anything else, an asset of the bundle
// included, answers 401.
func isNavigation(r *http.Request) bool {
	return (r.Method == http.MethodGet || r.Method == http.MethodHead) &&
		strings.Contains(r.Header.Get("Accept"), "text/html")
}

func cookieToken(r *http.Request) string {
	c, err := r.Cookie(SessionCookie)
	if err != nil {
		return ""
	}
	return c.Value
}

// principal is the session's user; nil with no error when the request has no
// session cookie.
func principal(r *http.Request, auth Authenticator) (*authsvc.Principal, error) {
	token := cookieToken(r)
	if token == "" {
		return nil, nil
	}
	if auth == nil {
		return nil, authsvc.ErrUnauthenticated
	}
	return auth.Authenticate(r.Context(), token)
}

// sameOrigin is the CSRF check of ADR 0029 step 4.5: the Origin header, else
// the Referer, must name the application's host. A request with neither is
// not a browser's (browsers send Origin on every cross-origin non-GET), and
// SameSite=Lax keeps the session cookie off cross-site subrequests anyway.
func sameOrigin(r *http.Request, host string) bool {
	if host == "" {
		host = r.Host
	}
	src := r.Header.Get("Origin")
	if src == "" {
		src = r.Header.Get("Referer")
	}
	if src == "" {
		return true
	}
	u, err := url.Parse(src)
	if err != nil || u.Host == "" {
		return false
	}
	return strings.EqualFold(u.Host, host)
}

type principalKey struct{}

// WithPrincipal returns ctx carrying the request's user.
func WithPrincipal(ctx context.Context, p *authsvc.Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}

// PrincipalFrom is the request's user, nil without a session.
func PrincipalFrom(ctx context.Context) *authsvc.Principal {
	p, _ := ctx.Value(principalKey{}).(*authsvc.Principal)
	return p
}

// SetSessionCookie sets the session cookie (ADR 0029 step 4.2): HttpOnly,
// SameSite=Lax, path /, host-only (no domain), Secure on HTTPS.
func SetSessionCookie(w http.ResponseWriter, token string, expires time.Time, secure bool) {
	http.SetCookie(w, &http.Cookie{ //nolint:gosec // Secure on HTTPS only: dev login runs on plain HTTP (ADR 0029 step 4.2)
		Name: SessionCookie, Value: token, Path: "/", Expires: expires,
		MaxAge: int(time.Until(expires).Seconds()), HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode,
	})
}

// ClearSessionCookie deletes the session cookie.
func ClearSessionCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{ //nolint:gosec // Secure on HTTPS only: dev login runs on plain HTTP (ADR 0029 step 4.2)
		Name: SessionCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode,
	})
}
