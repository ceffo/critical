// Package casthdl exposes POST /api/spells/{id}/cast,
// POST /api/spells/receive, POST /api/spells/effect and
// POST /api/spells/exposure.
package casthdl

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/ceffo/critical/api/internal/domain/service/castsvc"
	"github.com/ceffo/critical/api/internal/render"
	"github.com/ceffo/critical/api/internal/rest/wire"
)

// Router is the chi subset we need.
type Router interface {
	Post(pattern string, h http.HandlerFunc)
}

// CharacterIDFunc resolves the active character.
type CharacterIDFunc func(r *http.Request) int64

type castSvc interface {
	Cast(ctx context.Context, characterID int64, slotIndex int, req castsvc.CastRequest) (castsvc.CastResult, error)
	Receive(ctx context.Context, characterID int64, req castsvc.ReceiveRequest) (castsvc.CastResult, error)
	AddEffect(ctx context.Context, characterID int64, req castsvc.AddEffectRequest) (castsvc.CastResult, error)
	Expose(ctx context.Context, characterID int64, req castsvc.ExposureRequest) (castsvc.ExposureResult, error)
}

// RegisterRoutes wires the cast endpoints onto r.
func RegisterRoutes(r Router, charID CharacterIDFunc, svc castsvc.SVC) {
	r.Post("/api/spells/{id}/cast", render.Handle(Cast(charID, svc)))
	r.Post("/api/spells/receive", render.Handle(Receive(charID, svc)))
	r.Post("/api/spells/effect", render.Handle(AddEffect(charID, svc)))
	r.Post("/api/spells/exposure", render.Handle(Expose(charID, svc)))
}

// ExposureRequestDTO is the body of POST /api/spells/exposure: one round of
// exposure to a spell's area (ADR 0008 step 16).
type ExposureRequestDTO struct {
	SpellID     string `json:"spellId"`
	CasterLevel int    `json:"casterLevel"`
	// Mode: the key of the exposure mode (omitted: the first).
	Mode string `json:"mode,omitempty"`
	// Round: the round of exposure, from 1 (omitted: 1).
	Round int `json:"round,omitempty"`
	// OnSelf: the Bard is the one exposed; the damage comes off his HP.
	OnSelf bool `json:"onSelf,omitempty"`
}

// Expose rolls one round of exposure to a spell's area and logs it.
func Expose(charID CharacterIDFunc, svc castSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		var body ExposureRequestDTO
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			return render.BadRequestError("invalid JSON body")
		}
		res, err := svc.Expose(r.Context(), charID(r), castsvc.ExposureRequest{
			SpellID: body.SpellID, CasterLevel: body.CasterLevel, Mode: body.Mode, Round: body.Round, OnSelf: body.OnSelf,
		})
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, newExposureAnswer(res))
		return nil
	}
}

// CastRequestDTO is the optional body of POST /api/spells/{id}/cast.
type CastRequestDTO struct {
	// OnSelf: does a buff spell affect the Bard? Omitted: the spell's
	// default (see castsvc).
	OnSelf *bool `json:"onSelf,omitempty"`
	// SpellID: the spell cast from a blank extra slot.
	SpellID string `json:"spellId,omitempty"`
	// ExtraSlots: the split of the levels a spell that grants slots adds
	// (one of the spellbook entry's slotSplits; omitted: the first).
	ExtraSlots []int `json:"extraSlots,omitempty"`
	// ItemID: the item a spell that consumes an item spends (omitted: the
	// only one of its type left).
	ItemID string `json:"itemId,omitempty"`
	// TargetAC: the target's AC, required for a spell with an attack roll.
	TargetAC *int `json:"targetAc,omitempty"`
	// Mode: the key of the cast-time mode of a spell with modes (omitted:
	// the first).
	Mode string `json:"mode,omitempty"`
}

// ReceiveRequestDTO is the body of POST /api/spells/receive.
type ReceiveRequestDTO struct {
	SpellID     string `json:"spellId"`
	CasterLevel int    `json:"casterLevel"`
	// Mode: the cast-time mode the caster picked (omitted: the first).
	Mode string `json:"mode,omitempty"`
}

// Cast spends slot {id} and applies its spell.
func Cast(charID CharacterIDFunc, svc castSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		idx, err := strconv.Atoi(chi.URLParam(r, "id"))
		if err != nil {
			return render.BadRequestError("invalid slot index")
		}
		var body CastRequestDTO
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
			return render.BadRequestError("invalid JSON body")
		}
		res, err := svc.Cast(r.Context(), charID(r), idx, castsvc.CastRequest{OnSelf: body.OnSelf, SpellID: body.SpellID, ExtraSlots: body.ExtraSlots, ItemID: body.ItemID, TargetAC: body.TargetAC, Mode: body.Mode})
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, newCastAnswer(res))
		return nil
	}
}

// Receive applies a spell a party member cast on the Bard.
func Receive(charID CharacterIDFunc, svc castSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		var body ReceiveRequestDTO
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			return render.BadRequestError("invalid JSON body")
		}
		res, err := svc.Receive(r.Context(), charID(r), castsvc.ReceiveRequest{SpellID: body.SpellID, CasterLevel: body.CasterLevel, Mode: body.Mode})
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, newCastAnswer(res))
		return nil
	}
}

// AddEffectRequestDTO is the body of POST /api/spells/effect: a spell added
// as an effect on the Bard at his level (ADR 0004 step 19).
type AddEffectRequestDTO struct {
	SpellID string `json:"spellId"`
}

// AddEffect puts a spell that adds a buff on the Bard at his level: no slot,
// no damage.
func AddEffect(charID CharacterIDFunc, svc castSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		var body AddEffectRequestDTO
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			return render.BadRequestError("invalid JSON body")
		}
		res, err := svc.AddEffect(r.Context(), charID(r), castsvc.AddEffectRequest{SpellID: body.SpellID})
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, newCastAnswer(res))
		return nil
	}
}

// castAnswer is the answer of the cast endpoints: the cast's breakdown
// (castsvc.CastBreakdown, the logged payload plus logEntryId), then what is
// not part of the cast and so never logged: the catalogue spell (its texts
// and stats, absent when not in the catalogue) and the slots after a cast.
//
// entryId, source, name and consumedItem are logEntryId, action, spell and
// item under the names the previous release's web reads from this answer.
// They are kept for one release so that a page of that release still shows
// the cast against this server during the rolling deploy (ADR 0024 step
// 5.2); they are never logged. The attack's d20Roll is the one compat key
// that is also logged (it lives inside the shared payload type). Drop them
// all in the next release.
type castAnswer struct {
	castsvc.CastBreakdown
	CatalogueSpell *wire.SpellDTO      `json:"catalogueSpell,omitempty"`
	Slots          []wire.SpellSlotDTO `json:"slots,omitempty"`

	EntryID      int64             `json:"entryId"`
	Source       string            `json:"source"`
	Name         string            `json:"name"`
	ConsumedItem *castsvc.CastItem `json:"consumedItem,omitempty"`
}

func newCastAnswer(res castsvc.CastResult) castAnswer {
	b := castsvc.NewCastBreakdown(res)
	out := castAnswer{
		CastBreakdown: b,
		EntryID:       b.LogEntryID, Source: b.Action, Name: b.Spell, ConsumedItem: b.Item,
	}
	if len(res.Spell.Name) > 0 {
		s := wire.SpellToDTO(res.Spell, res.CasterLevel)
		out.CatalogueSpell = &s
	}
	if res.Slots != nil {
		out.Slots = wire.SlotsToDTO(res.Slots)
	}
	return out
}

// exposureAnswer is the answer of POST /api/spells/exposure: the round's
// breakdown (castsvc.ExposureBreakdown, the logged payload plus logEntryId),
// then hpDelta, the HP change the ledger applied when the Bard was exposed
// (the floor may make it smaller than the damage; 0 otherwise), which the
// entry carries as its own delta, not in its payload.
//
// entryId and name are logEntryId and spell under the previous release's
// names, kept for one release like castAnswer's. Drop them in the next
// release.
type exposureAnswer struct {
	castsvc.ExposureBreakdown
	HPDelta int `json:"hpDelta"`

	EntryID int64  `json:"entryId"`
	Name    string `json:"name"`
}

func newExposureAnswer(res castsvc.ExposureResult) exposureAnswer {
	b := castsvc.NewExposureBreakdown(res)
	return exposureAnswer{ExposureBreakdown: b, HPDelta: res.HPDelta, EntryID: b.LogEntryID, Name: b.Spell}
}
