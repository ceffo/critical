// Package router builds the chi router for the api binary. Per
// clean_architecture.md §7.2 the router file does not list endpoints — each
// handler package's RegisterRoutes does that, keeping URL surface co-located
// with handlers.
package router

import (
	"log/slog"
	"net/http"

	"github.com/ceffo/critical/api/internal/composition"
	"github.com/ceffo/critical/api/internal/render"
	"github.com/ceffo/critical/api/internal/rest/handler/actionloghdl"
	"github.com/ceffo/critical/api/internal/rest/handler/buffhdl"
	"github.com/ceffo/critical/api/internal/rest/handler/casthdl"
	"github.com/ceffo/critical/api/internal/rest/handler/characterhdl"
	"github.com/ceffo/critical/api/internal/rest/handler/combathdl"
	"github.com/ceffo/critical/api/internal/rest/handler/hphdl"
	"github.com/ceffo/critical/api/internal/rest/handler/itemhdl"
	"github.com/ceffo/critical/api/internal/rest/handler/resthdl"
	"github.com/ceffo/critical/api/internal/rest/handler/spellhdl"
	"github.com/ceffo/critical/api/internal/rest/handler/spellprephdl"
	"github.com/ceffo/critical/api/internal/rest/handler/weaponhdl"
	mw "github.com/ceffo/critical/api/internal/rest/middleware"
	"github.com/go-chi/chi/v5"
)

// New builds the router and mounts each handler package against bs.
func New(log *slog.Logger, bs *composition.BackingServices) http.Handler {
	r := chi.NewRouter()
	r.Use(mw.Logger(log))
	r.Use(mw.Recoverer)

	r.Get("/api/health", func(w http.ResponseWriter, req *http.Request) {
		render.OK(req.Context(), w, map[string]string{"status": "ok"})
	})

	charID := func(_ *http.Request) int64 { return bs.CharacterID }

	characterhdl.RegisterRoutes(r, charID, bs.Character)
	hphdl.RegisterRoutes(r, charID, bs.HP)
	actionloghdl.RegisterRoutes(r, charID, bs.Log)
	buffhdl.RegisterRoutes(r, charID, bs.Buffs)
	spellhdl.RegisterRoutes(r, charID, bs.Spells, bs.Character)
	spellprephdl.RegisterRoutes(r, charID, bs.Prep)
	casthdl.RegisterRoutes(r, charID, bs.Cast)
	combathdl.RegisterRoutes(r, charID, bs.Combat)
	itemhdl.RegisterRoutes(r, charID, bs.Items)
	weaponhdl.RegisterRoutes(r, charID, bs.Weapons)
	resthdl.RegisterRoutes(r, charID, bs.Rest)

	return r
}
