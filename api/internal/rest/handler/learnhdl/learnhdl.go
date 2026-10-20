// Package learnhdl serves writing a spell in the spellbook (ADR 0030 step
// 20): the chance-to-learn roll that writes a wizard spell in the current
// character's spellbook, or records the failure, and the roll-free add
// (Ajouter, step 20.6).
package learnhdl

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/ceffo/critical/api/internal/domain/service/learnsvc"
	"github.com/ceffo/critical/api/internal/render"
)

// Router is the slice of chi the handler registers on.
type Router interface {
	Post(pattern string, h http.HandlerFunc)
}

// CharacterIDFunc reads the current character's id from the request.
type CharacterIDFunc func(r *http.Request) int64

type learner interface {
	Learn(ctx context.Context, characterID int64, spellID string) (*learnsvc.Result, error)
}

type adder interface {
	Add(ctx context.Context, characterID int64, spellID string) (*learnsvc.Added, error)
}

// RegisterRoutes registers the learn and add endpoints.
func RegisterRoutes(r Router, charID CharacterIDFunc, svc learnsvc.SVC) {
	r.Post("/api/character/spellbook/learn", render.Handle(Learn(charID, svc)))
	r.Post("/api/character/spellbook/add", render.Handle(Add(charID, svc)))
}

// LearnBody is the request: the catalogue id of the spell to learn.
type LearnBody struct {
	SpellID string `json:"spellId"`
}

// ResultDTO is the roll's full breakdown (step 20.2).
type ResultDTO struct {
	SpellID        string `json:"spellId"`
	SpellLevel     int    `json:"spellLevel"`
	Roll           int    `json:"roll"`
	Chance         int    `json:"chance"`
	Learned        bool   `json:"learned"`
	CharacterLevel int    `json:"characterLevel"`
	// RetryLevel is the realized level from which a failed spell may be
	// tried again; 0 when learned.
	RetryLevel int `json:"retryLevel"`
}

// AddedDTO is the entry a roll-free add wrote (step 20.6): the spell, its
// level and the entry's origin (step 20.5).
type AddedDTO struct {
	SpellID    string `json:"spellId"`
	SpellLevel int    `json:"spellLevel"`
	// Origin is "learned", "given", "starting" or "level_grant".
	Origin string `json:"origin"`
}

// spellBody decodes the request's body, the spell's catalogue id.
func spellBody(r *http.Request) (string, error) {
	var b LearnBody
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&b); err != nil {
		return "", render.BadRequestError("invalid JSON body")
	}
	if b.SpellID == "" {
		return "", render.BadRequestError("spellId is required")
	}
	return b.SpellID, nil
}

// Add handles POST /api/character/spellbook/add {spellId}: the spell written
// in the spellbook with no roll, refused like Learn but for a failure's block.
func Add(charID CharacterIDFunc, svc adder) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		spellID, err := spellBody(r)
		if err != nil {
			return err
		}
		res, err := svc.Add(r.Context(), charID(r), spellID)
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, AddedDTO{
			SpellID: res.SpellID, SpellLevel: res.SpellLevel, Origin: string(res.Origin),
		})
		return nil
	}
}

// Learn handles POST /api/character/spellbook/learn.
func Learn(charID CharacterIDFunc, svc learner) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		spellID, err := spellBody(r)
		if err != nil {
			return err
		}
		res, err := svc.Learn(r.Context(), charID(r), spellID)
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, ResultDTO{
			SpellID: res.SpellID, SpellLevel: res.SpellLevel, Roll: res.Roll, Chance: res.Chance,
			Learned: res.Learned, CharacterLevel: res.CharacterLevel,
			RetryLevel: res.RetryLevel,
		})
		return nil
	}
}
