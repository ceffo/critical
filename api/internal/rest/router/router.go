// Package router builds the chi router for the api binary. Per
// clean_architecture.md §7.2 the router file does not list endpoints — each
// handler package's RegisterRoutes does that, keeping URL surface co-located
// with handlers.
package router

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/ceffo/critical/api/internal/composition"
	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/render"
	"github.com/ceffo/critical/api/internal/rest/handler/actionloghdl"
	"github.com/ceffo/critical/api/internal/rest/handler/authhdl"
	"github.com/ceffo/critical/api/internal/rest/handler/buffhdl"
	"github.com/ceffo/critical/api/internal/rest/handler/casthdl"
	"github.com/ceffo/critical/api/internal/rest/handler/characterhdl"
	"github.com/ceffo/critical/api/internal/rest/handler/combathdl"
	"github.com/ceffo/critical/api/internal/rest/handler/detailshdl"
	"github.com/ceffo/critical/api/internal/rest/handler/dicehdl"
	"github.com/ceffo/critical/api/internal/rest/handler/hphdl"
	"github.com/ceffo/critical/api/internal/rest/handler/itemhdl"
	"github.com/ceffo/critical/api/internal/rest/handler/resthdl"
	"github.com/ceffo/critical/api/internal/rest/handler/skillhdl"
	"github.com/ceffo/critical/api/internal/rest/handler/spellhdl"
	"github.com/ceffo/critical/api/internal/rest/handler/spellprephdl"
	"github.com/ceffo/critical/api/internal/rest/handler/weaponhdl"
	"github.com/ceffo/critical/api/internal/rest/handler/xphdl"
	mw "github.com/ceffo/critical/api/internal/rest/middleware"
)

// Options are the binary-level settings the router needs besides the services.
type Options struct {
	// Web serves every path outside /api (the built web app, ADR 0024 step
	// 1.1). Nil serves nothing there: such paths get the JSON 404, as in
	// `just dev` where Vite serves the app (step 1.2).
	Web http.Handler
	// Commit is the commit the binary was built from, reported by
	// /api/health (ADR 0024 step 1.4).
	Commit string
	// Version is the release version (X.Y.Z) baked into the image, reported
	// by /api/health (ADR 0024 step 1.4); empty is reported as "unknown".
	Version string
	// Host is the application's own host, from its public URL (the CSRF
	// check of ADR 0029 step 4.5); empty accepts the request's Host.
	Host string
	// SecureCookies marks the session cookies Secure (HTTPS, step 4.2).
	SecureCookies bool
	// CookieApp names the auth cookies of this app (mw.SessionCookieName):
	// its public host and port, or its dev port. Empty, the base names.
	CookieApp string
}

// RequestDeadline bounds every request: a slow or unavailable database answers
// a click with an error within a few seconds instead of holding it (the
// connection gate of storage/postgres waits no longer than this either). A
// request on a database that answers takes a small fraction of it.
const RequestDeadline = 8 * time.Second

// unknown is what /api/health reports for a build identity that was not set.
const unknown = "unknown"

func orUnknown(s string) string {
	if s == "" {
		return unknown
	}
	return s
}

// apiPrefix is the path every endpoint lives under.
const apiPrefix = "/api"

// New builds the router and mounts each handler package against bs.
func New(log *slog.Logger, bs *composition.BackingServices, opts Options) http.Handler {
	r := chi.NewRouter()
	r.Use(mw.Logger(log))
	r.Use(mw.Recoverer)
	// Before the gate, which reads the session from the database.
	r.Use(mw.Deadline(RequestDeadline))
	// The one access check (ADR 0029 step 6.2): every route below, and the
	// web app outside /api, needs a session unless mw.Public lists it.
	var auth mw.Authenticator
	if bs.Auth != nil {
		auth = bs.Auth
	}
	r.Use(mw.Gate(auth, mw.GateConfig{
		Host: opts.Host, Secure: opts.SecureCookies, Cookie: mw.SessionCookieName(opts.CookieApp),
	}))

	r.Get("/api/health", func(w http.ResponseWriter, req *http.Request) {
		render.OK(req.Context(), w, map[string]string{
			"status": "ok", "commit": orUnknown(opts.Commit), "version": orUnknown(opts.Version),
		})
	})
	r.NotFound(notFound(opts.Web))

	if bs.Auth != nil {
		authhdl.RegisterRoutes(r, bs.Auth, authhdl.Options{Secure: opts.SecureCookies, App: opts.CookieApp})
	}

	charID := func(_ *http.Request) int64 { return bs.CharacterID }

	characterhdl.RegisterRoutes(r, charID, bs.Character)
	hphdl.RegisterRoutes(r, charID, bs.HP)
	actionloghdl.RegisterRoutes(r, charID, bs.Log)
	buffhdl.RegisterRoutes(r, charID, bs.Buffs)
	spellhdl.RegisterRoutes(r, charID, bs.Spells, bs.Character, bs.Cast)
	spellprephdl.RegisterRoutes(r, charID, bs.Prep)
	casthdl.RegisterRoutes(r, charID, bs.Cast)
	combathdl.RegisterRoutes(r, charID, bs.Combat)
	itemhdl.RegisterRoutes(r, charID, bs.Items)
	weaponhdl.RegisterRoutes(r, charID, bs.Weapons)
	resthdl.RegisterRoutes(r, charID, bs.Rest)
	xphdl.RegisterRoutes(r, charID, bs.XP)
	detailshdl.RegisterRoutes(r, charID, bs.Details)
	skillhdl.RegisterRoutes(r, charID, bs.Skills)
	dicehdl.RegisterRoutes(r, charID, bs.Dice)

	return r
}

// notFound handles every path no route matches: outside /api it is the web
// app's when one is configured; anything else is the JSON 404 of the error
// table.
func notFound(web http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		p := req.URL.Path
		isAPI := p == apiPrefix || strings.HasPrefix(p, apiPrefix+"/")
		if web != nil && !isAPI {
			web.ServeHTTP(w, req)
			return
		}
		render.Error(req.Context(), w, fmt.Errorf("%w: %s", domain.ErrNotFound, p))
	}
}
