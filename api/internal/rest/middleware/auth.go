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

// SessionCookie is the session cookie's base name (ADR 0029 step 4.2). It
// holds the session's random token and nothing about the user. A server
// names its cookie SessionCookieName(app), so the apps one browser reaches
// on the same host keep separate sessions.
const SessionCookie = "critical_session"

// SessionCookieName is the session cookie of the app identified by app (its
// public host and port, or its local dev port): browsers send a cookie to
// every port of a host, so `just dev` on localhost:5173 and a hosted app
// tunnelled to localhost:<port> would otherwise share one cookie, log each
// other out and send one app's token to the other. Empty app is the base
// name. Characters a cookie name cannot hold become '_'.
func SessionCookieName(app string) string { return CookieName(SessionCookie, app) }

// CookieName is base suffixed with app, as SessionCookieName.
func CookieName(base, app string) string {
	if app == "" {
		return base
	}
	b := []byte(app)
	for i, c := range b {
		if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') && c != '-' && c != '.' {
			b[i] = '_'
		}
	}
	return base + "_" + string(b)
}

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
	// Cookie is the session cookie's name (SessionCookieName); empty is
	// SessionCookie.
	Cookie string
}

func (c GateConfig) cookie() string {
	if c.Cookie == "" {
		return SessionCookie
	}
	return c.Cookie
}

// staticPrefix holds the web build's content-hashed bundle (Vite's assets/).
const staticPrefix = "/assets/"

// favicon is the one file of the web build outside assets/ the login page
// loads.
const favicon = "/favicon.svg"

// Public reports whether path is served without a session (ADR 0029 step
// 6.1): the login page and what is under it, the static files it loads (the
// web build's bundle under /assets/ and the favicon: the login page is part
// of the one bundle, which is the public repository's code and carries no
// data), the /api/auth endpoints and /api/health. Everything else needs one
// (step 6.2): every /api data route and the catalogue; a new route is
// protected by default.
func Public(path string) bool {
	return path == "/api/health" ||
		path == "/api/auth" || strings.HasPrefix(path, "/api/auth/") ||
		path == LoginPath || strings.HasPrefix(path, LoginPath+"/") ||
		strings.HasPrefix(path, staticPrefix) || path == favicon
}

// ExpiredLoginPath is where a page opened with a session the server no
// longer knows goes: the login page says the session has ended.
const ExpiredLoginPath = LoginPath + "?error=session_expired"

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
// session redirects a page to the login page (saying the session has ended
// when the request carried one the server no longer knows) and answers 401 to
// every /api route, except on the public paths. A nil auth authenticates nobody.
func Gate(auth Authenticator, cfg GateConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			if !safeMethod(r.Method) && !sameOrigin(r, cfg.Host) {
				render.Error(ctx, w, authsvc.ErrCrossOrigin)
				return
			}
			name := cfg.cookie()
			p, err := principal(r, name, auth)
			stale := errors.Is(err, authsvc.ErrUnauthenticated)
			switch {
			case stale:
				ClearSessionCookie(w, name, cfg.Secure)
				ctx = withStaleSession(ctx)
				r = r.WithContext(ctx)
			case err != nil:
				render.Error(ctx, w, err)
				return
			case p != nil && p.Extended:
				SetSessionCookie(w, name, CookieToken(r, name), p.Expires, cfg.Secure)
			}
			if p != nil {
				r = r.WithContext(WithPrincipal(ctx, p))
			}
			if p == nil && !Public(r.URL.Path) {
				if !isAPI(r.URL.Path) && isNavigation(r) {
					to := LoginPath
					if stale {
						to = ExpiredLoginPath
					}
					http.Redirect(w, r, to, http.StatusFound)
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
// which goes to the login page; anything else answers 401.
func isNavigation(r *http.Request) bool {
	return (r.Method == http.MethodGet || r.Method == http.MethodHead) &&
		strings.Contains(r.Header.Get("Accept"), "text/html")
}

// CookieToken is the value of the cookie name, empty without one.
func CookieToken(r *http.Request, name string) string {
	c, err := r.Cookie(name)
	if err != nil {
		return ""
	}
	return c.Value
}

// principal is the session's user; nil with no error when the request has no
// session cookie.
func principal(r *http.Request, name string, auth Authenticator) (*authsvc.Principal, error) {
	token := CookieToken(r, name)
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

type staleKey struct{}

func withStaleSession(ctx context.Context) context.Context {
	return context.WithValue(ctx, staleKey{}, true)
}

// StaleSession reports a request that carried a session cookie the server no
// longer knows (expired, logged out, or its user disabled): the gate dropped
// the cookie and the request has no principal.
func StaleSession(ctx context.Context) bool {
	v, _ := ctx.Value(staleKey{}).(bool)
	return v
}

// WithPrincipal returns ctx carrying the request's user.
func WithPrincipal(ctx context.Context, p *authsvc.Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}

// PrincipalFrom is the request's user, nil without a session.
func PrincipalFrom(ctx context.Context) *authsvc.Principal {
	p, _ := ctx.Value(principalKey{}).(*authsvc.Principal)
	return p
}

// SetSessionCookie sets the session cookie name (ADR 0029 step 4.2):
// HttpOnly, SameSite=Lax, path /, host-only (no domain), Secure on HTTPS.
func SetSessionCookie(w http.ResponseWriter, name, token string, expires time.Time, secure bool) {
	http.SetCookie(w, &http.Cookie{ //nolint:gosec // Secure on HTTPS only: dev login runs on plain HTTP (ADR 0029 step 4.2)
		Name: name, Value: token, Path: "/", Expires: expires,
		MaxAge: int(time.Until(expires).Seconds()), HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode,
	})
}

// ClearSessionCookie deletes the session cookie name.
func ClearSessionCookie(w http.ResponseWriter, name string, secure bool) {
	http.SetCookie(w, &http.Cookie{ //nolint:gosec // Secure on HTTPS only: dev login runs on plain HTTP (ADR 0029 step 4.2)
		Name: name, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode,
	})
}
