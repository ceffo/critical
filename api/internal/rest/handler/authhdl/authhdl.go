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
}

// LoginCookie holds a started Google login's state and PKCE verifier for
// authsvc.LoginStateLifetime (ADR 0029 step 3.1), on the login's paths only.
const LoginCookie = "critical_login"

const loginCookiePath = "/api/auth/google"

// Options are the cookie settings: Secure on HTTPS (ADR 0029 step 4.2).
type Options struct {
	Secure bool
}

// RegisterRoutes wires the auth endpoints onto r. POST /api/auth/dev exists
// only in dev login mode (ADR 0029 step 8.2: else 404).
func RegisterRoutes(r Router, svc Service, opts Options) {
	r.Get("/api/auth/google/start", render.Handle(Start(svc, opts)))
	r.Get("/api/auth/google/callback", Callback(svc, opts))
	r.Post("/api/auth/logout", render.Handle(Logout(svc, opts)))
	r.Get("/api/auth/me", render.Handle(Me()))
	if svc.DevLoginEnabled() {
		r.Post("/api/auth/dev", render.Handle(Dev(svc, opts)))
	}
}

// MeDTO is the current user (ADR 0029 step 4.6).
type MeDTO struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Role  string `json:"role"`
	Root  bool   `json:"root"`
}

func meDTO(u authsvc.User) MeDTO {
	return MeDTO{ID: u.ID.String(), Email: u.Email, Role: string(u.Role), Root: u.Root}
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
			Name: LoginCookie, Value: ls.State + "." + ls.Verifier, Path: loginCookiePath,
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
		if c, err := r.Cookie(LoginCookie); err == nil {
			state, verifier, _ = strings.Cut(c.Value, ".")
		}
		http.SetCookie(w, &http.Cookie{ //nolint:gosec // Secure on HTTPS only: dev login runs on plain HTTP (ADR 0029 step 4.2)
			Name: LoginCookie, Path: loginCookiePath, MaxAge: -1, HttpOnly: true, Secure: opts.Secure,
			SameSite: http.SameSiteLaxMode,
		})
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
		mw.SetSessionCookie(w, s.Token, s.Expires, opts.Secure)
		http.Redirect(w, r, "/", http.StatusFound)
	}
}

// Logout is POST /api/auth/logout: the session and the cookie are deleted.
func Logout(svc Service, opts Options) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		if c, err := r.Cookie(mw.SessionCookie); err == nil {
			if err := svc.Logout(r.Context(), c.Value); err != nil {
				return err
			}
		}
		mw.ClearSessionCookie(w, opts.Secure)
		render.OK(r.Context(), w, struct{}{})
		return nil
	}
}

// Me is GET /api/auth/me: the current user, 401 without a session (how the
// web decides to show the login page, ADR 0029 step 4.6).
func Me() render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		p := mw.PrincipalFrom(r.Context())
		if p == nil {
			return authsvc.ErrUnauthenticated
		}
		render.OK(r.Context(), w, meDTO(p.User))
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
		mw.SetSessionCookie(w, s.Token, s.Expires, opts.Secure)
		render.OK(r.Context(), w, meDTO(s.User))
		return nil
	}
}
