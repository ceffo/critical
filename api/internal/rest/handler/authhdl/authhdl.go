// Package authhdl exposes the login (ADR 0029): the Google login's start and
// callback, logout, the current user and, without OAuth settings, the dev
// login. Every path is under /api/auth, public by the gate's allow-list.
package authhdl

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/ceffo/critical/api/internal/domain/service/authsvc"
	"github.com/ceffo/critical/api/internal/render"
	mw "github.com/ceffo/critical/api/internal/rest/middleware"
)

// Router is the chi subset we need.
type Router interface {
	Get(pattern string, h http.HandlerFunc)
	Post(pattern string, h http.HandlerFunc)
}

var _ Router = (*chi.Mux)(nil)

// Service is what the handlers call (authsvc.SVC satisfies it).
type Service interface {
	DevLoginEnabled() bool
	StartLogin(ctx context.Context) (*authsvc.LoginStart, error)
	FinishLogin(ctx context.Context, in authsvc.FinishLogin) (*authsvc.NewSession, error)
	DevLogin(ctx context.Context, email string) (*authsvc.NewSession, error)
	Logout(ctx context.Context, token string) error
	FreshCurrentCharacter(ctx context.Context, p *authsvc.Principal) (int64, error)
}

// LoginCookie is the base name of the cookie holding a started Google
// login's state and PKCE verifier for authsvc.LoginStateLifetime (ADR 0029
// step 3.1), on the login's paths only; suffixed per app like the session
// cookie (mw.CookieName).
const LoginCookie = "critical_login"

const loginCookiePath = "/api/auth/google"

// Options are the cookie settings: Secure on HTTPS (ADR 0029 step 4.2), and
// the app the cookies are named for (mw.SessionCookieName; empty, the base
// names).
type Options struct {
	Secure bool
	App    string
}

func (o Options) sessionCookie() string { return mw.SessionCookieName(o.App) }
func (o Options) loginCookie() string   { return mw.CookieName(LoginCookie, o.App) }

// StartPath and CallbackPath are the Google login's two browser navigations.
const (
	StartPath    = loginCookiePath + "/start"
	CallbackPath = loginCookiePath + "/callback"
)

// CodeLoginTimeout is the login page's code for a Google login that did not
// finish within the request deadline (Google or the database too slow).
const CodeLoginTimeout = "login_timeout"

// LateAnswers are the answers of the Google login's navigations when they
// pass the request deadline (mw.Deadline): the login cookie is dropped and the
// browser goes to the login page with CodeLoginTimeout, never to a JSON error
// (ADR 0025 step 10.1, ADR 0029 step 3.7).
func LateAnswers(opts Options) map[string]http.Handler {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		dropLoginCookie(w, opts)
		http.Redirect(w, r, mw.LoginPath+"?error="+CodeLoginTimeout, http.StatusFound)
	})
	return map[string]http.Handler{StartPath: h, CallbackPath: h}
}

func dropLoginCookie(w http.ResponseWriter, opts Options) {
	http.SetCookie(w, &http.Cookie{ //nolint:gosec // Secure on HTTPS only: dev login runs on plain HTTP (ADR 0029 step 4.2)
		Name: opts.loginCookie(), Path: loginCookiePath, MaxAge: -1, HttpOnly: true, Secure: opts.Secure,
		SameSite: http.SameSiteLaxMode,
	})
}

// RegisterRoutes wires the auth endpoints onto r. POST /api/auth/dev exists
// only in dev login mode (ADR 0029 step 8.2: else 404).
func RegisterRoutes(r Router, svc Service, opts Options) {
	r.Get(StartPath, render.Handle(Start(svc, opts)))
	r.Get(CallbackPath, Callback(svc, opts))
	r.Post("/api/auth/logout", render.Handle(Logout(svc, opts)))
	r.Get("/api/auth/me", render.Handle(Me(svc)))
	r.Get("/api/auth/options", render.Handle(LoginOptions(svc)))
	if svc.DevLoginEnabled() {
		r.Post("/api/auth/dev", render.Handle(Dev(svc, opts)))
	}
}

// MeDTO is the current user (ADR 0029 step 4.6), with their current
// character's id (step 7.2; null when they have none).
type MeDTO struct {
	ID                 string `json:"id"`
	Email              string `json:"email"`
	Role               string `json:"role"`
	Root               bool   `json:"root"`
	CurrentCharacterID *int64 `json:"currentCharacterId"`
}

func meDTO(u authsvc.User) MeDTO {
	d := MeDTO{ID: u.ID.String(), Email: u.Email, Role: string(u.Role), Root: u.Root}
	if u.CurrentCharacterID != 0 {
		id := u.CurrentCharacterID
		d.CurrentCharacterID = &id
	}
	return d
}

// Start is GET /api/auth/google/start: the state and verifier go into the
// login cookie, the browser to Google.
func Start(svc Service, opts Options) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		ls, err := svc.StartLogin(r.Context())
		if err != nil {
			return err
		}
		http.SetCookie(w, &http.Cookie{ //nolint:gosec // Secure on HTTPS only: dev login runs on plain HTTP (ADR 0029 step 4.2)
			Name: opts.loginCookie(), Value: ls.State + "." + ls.Verifier, Path: loginCookiePath,
			MaxAge: int(authsvc.LoginStateLifetime / time.Second), HttpOnly: true, Secure: opts.Secure,
			SameSite: http.SameSiteLaxMode,
		})
		http.Redirect(w, r, ls.URL, http.StatusFound)
		return nil
	}
}

// Callback is GET /api/auth/google/callback: the login cookie is dropped
// whatever happens; a login starts a session and goes to the app, a refusal
// goes to the login page with its stable code (ADR 0029 step 3.7).
func Callback(svc Service, opts Options) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var state, verifier string
		if c, err := r.Cookie(opts.loginCookie()); err == nil {
			state, verifier, _ = strings.Cut(c.Value, ".")
		}
		dropLoginCookie(w, opts)
		q := r.URL.Query()
		s, err := svc.FinishLogin(r.Context(), authsvc.FinishLogin{
			Code: q.Get("code"), State: q.Get("state"), ExpectedState: state, Verifier: verifier,
			ProviderError: q.Get("error"),
		})
		if err != nil {
			code := render.Code(err)
			log := render.Logger(r.Context())
			if code == "internal" {
				log.Error("login callback", "error", err)
			} else {
				log.Info("login refused", "code", code, "error", err)
			}
			http.Redirect(w, r, mw.LoginPath+"?error="+url.QueryEscape(code), http.StatusFound)
			return
		}
		mw.SetSessionCookie(w, opts.sessionCookie(), s.Token, s.Expires, opts.Secure)
		http.Redirect(w, r, "/", http.StatusFound)
	}
}

// Logout is POST /api/auth/logout: the session and the cookie are deleted.
func Logout(svc Service, opts Options) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		if token := mw.CookieToken(r, opts.sessionCookie()); token != "" {
			if err := svc.Logout(r.Context(), token); err != nil {
				return err
			}
		}
		mw.ClearSessionCookie(w, opts.sessionCookie(), opts.Secure)
		render.OK(r.Context(), w, struct{}{})
		return nil
	}
}

// Me is GET /api/auth/me: the current user, 401 without a session (how the
// web decides to show the login page, ADR 0029 step 4.6): session_expired
// when the request carried a session the server no longer knows, so the
// login page says it has ended, unauthenticated without one.
func Me(svc Service) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		p := mw.PrincipalFrom(r.Context())
		if p == nil && mw.StaleSession(r.Context()) {
			return authsvc.ErrSessionExpired
		}
		if p == nil {
			return authsvc.ErrUnauthenticated
		}
		// The web names the character it shows after this answer (ADR 0029
		// step 7.2), once per page load: the database's current character,
		// not the one this server may remember from before a switch made on
		// the other server.
		u := p.User
		current, err := svc.FreshCurrentCharacter(r.Context(), p)
		if err != nil {
			return err
		}
		u.CurrentCharacterID = current
		render.OK(r.Context(), w, meDTO(u))
		return nil
	}
}

// OptionsDTO is how this server lets a user log in, for the login page's
// buttons: Google with OAuth settings, else the dev login (ADR 0029 step 8).
type OptionsDTO struct {
	Google   bool `json:"google"`
	DevLogin bool `json:"devLogin"`
}

// LoginOptions is GET /api/auth/options: the login page offers the methods it
// lists.
func LoginOptions(svc Service) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		dev := svc.DevLoginEnabled()
		render.OK(r.Context(), w, OptionsDTO{Google: !dev, DevLogin: dev})
		return nil
	}
}

// DevRequest is the dev login's body; it may be empty (the root admin).
type DevRequest struct {
	Email string `json:"email"`
}

// Dev is POST /api/auth/dev (ADR 0029 step 8.1): a session for the email, or
// for the root admin without one.
func Dev(svc Service, opts Options) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		var req DevRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
			return render.BadRequestError("invalid JSON body")
		}
		s, err := svc.DevLogin(r.Context(), req.Email)
		if err != nil {
			return err
		}
		mw.SetSessionCookie(w, opts.sessionCookie(), s.Token, s.Expires, opts.Secure)
		render.OK(r.Context(), w, meDTO(s.User))
		return nil
	}
}
