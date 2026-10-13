// Package buffhdl exposes /api/buffs, /api/round and /api/stats/str.
package buffhdl

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/render"
	"github.com/go-chi/chi/v5"
)

// Router is the chi subset we need.
type Router interface {
	Get(pattern string, h http.HandlerFunc)
	Post(pattern string, h http.HandlerFunc)
	Patch(pattern string, h http.HandlerFunc)
	Put(pattern string, h http.HandlerFunc)
	Delete(pattern string, h http.HandlerFunc)
}

// CharacterIDFunc resolves the active character.
type CharacterIDFunc func(r *http.Request) int64

type buffSvc interface {
	ListStatus(ctx context.Context, characterID int64) ([]buffsvc.BuffStatus, error)
	Activate(ctx context.Context, characterID int64, buff buffsvc.BuffEffect) ([]buffsvc.BuffEffect, error)
	Edit(ctx context.Context, characterID int64, buffID string, e buffsvc.BuffEdit) ([]buffsvc.BuffEffect, error)
	Dismiss(ctx context.Context, characterID int64, buffID string) ([]buffsvc.BuffEffect, error)
	Trigger(ctx context.Context, characterID int64, buffID string) ([]buffsvc.BuffEffect, error)
	GetRound(ctx context.Context, characterID int64) (int, error)
	AdvanceRound(ctx context.Context, characterID int64) (active []buffsvc.BuffEffect, expired []buffsvc.BuffEffect, round int, err error)
	ResetRound(ctx context.Context, characterID int64) (active []buffsvc.BuffEffect, round int, err error)
	SetRound(ctx context.Context, characterID int64, to int) (active []buffsvc.BuffEffect, round int, err error)
	PassTime(ctx context.Context, characterID int64, hours float64) (active []buffsvc.BuffEffect, expired []buffsvc.BuffEffect, rounds int, round int, err error)
	GetEffectiveStr(ctx context.Context, characterID int64) (buffsvc.Strength, error)
	ListSaved(ctx context.Context, characterID int64) ([]buffsvc.SavedBuff, error)
	SaveBuff(ctx context.Context, characterID int64, saved buffsvc.SavedBuff) ([]buffsvc.SavedBuff, error)
	DeleteSaved(ctx context.Context, characterID int64, id int64) ([]buffsvc.SavedBuff, error)
}

// RegisterRoutes wires buff, round and effective-STR endpoints onto r.
func RegisterRoutes(r Router, charID CharacterIDFunc, svc buffsvc.SVC) {
	r.Get("/api/buffs", render.Handle(ListActive(charID, svc)))
	r.Post("/api/buffs", render.Handle(Activate(charID, svc)))
	r.Get("/api/buffs/saved", render.Handle(ListSaved(charID, svc)))
	r.Post("/api/buffs/saved", render.Handle(SaveBuff(charID, svc)))
	r.Delete("/api/buffs/saved/{id}", render.Handle(DeleteSaved(charID, svc)))
	r.Patch("/api/buffs/{id}", render.Handle(Edit(charID, svc)))
	r.Delete("/api/buffs/{id}", render.Handle(Dismiss(charID, svc)))
	r.Post("/api/buffs/{id}/trigger", render.Handle(Trigger(charID, svc)))
	r.Get("/api/round", render.Handle(GetRound(charID, svc)))
	r.Post("/api/round/advance", render.Handle(AdvanceRound(charID, svc)))
	r.Post("/api/round/reset", render.Handle(ResetRound(charID, svc)))
	r.Put("/api/round", render.Handle(SetRound(charID, svc)))
	r.Post("/api/round/pass-time", render.Handle(PassTime(charID, svc)))
	r.Get("/api/stats/str", render.Handle(GetEffectiveStr(charID, svc)))
}

// renderList writes the active list with each buff's applies flag.
func renderList(w http.ResponseWriter, r *http.Request, charID CharacterIDFunc, svc buffSvc) error {
	buffs, err := svc.ListStatus(r.Context(), charID(r))
	if err != nil {
		return err
	}
	render.OK(r.Context(), w, toStatusDTOs(buffs))
	return nil
}

// ListActive returns the current buff list.
func ListActive(charID CharacterIDFunc, svc buffSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		return renderList(w, r, charID, svc)
	}
}

// Activate adds a buff to the active list.
func Activate(charID CharacterIDFunc, svc buffSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		var dto BuffEffectDTO
		if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
			return render.BadRequestError("invalid request body")
		}
		if _, err := svc.Activate(r.Context(), charID(r), dto.toDomain()); err != nil {
			return err
		}
		return renderList(w, r, charID, svc)
	}
}

// BuffEditDTO is the PATCH /api/buffs/{id} body; omitted fields are unchanged.
type BuffEditDTO struct {
	Duration *int    `json:"duration"`
	Charges  *int    `json:"charges"`
	Note     *string `json:"note"`
}

// Edit corrects a buff's remaining rounds and/or charges.
func Edit(charID CharacterIDFunc, svc buffSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		var dto BuffEditDTO
		if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
			return render.BadRequestError("invalid request body")
		}
		edit := buffsvc.BuffEdit{Duration: dto.Duration, Charges: dto.Charges, Note: dto.Note}
		if _, err := svc.Edit(r.Context(), charID(r), chi.URLParam(r, "id"), edit); err != nil {
			return err
		}
		return renderList(w, r, charID, svc)
	}
}

// Dismiss removes the buff with the given id.
func Dismiss(charID CharacterIDFunc, svc buffSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		if _, err := svc.Dismiss(r.Context(), charID(r), chi.URLParam(r, "id")); err != nil {
			return err
		}
		return renderList(w, r, charID, svc)
	}
}

// Trigger uses one charge of a charged buff (absorbed a hit).
func Trigger(charID CharacterIDFunc, svc buffSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		if _, err := svc.Trigger(r.Context(), charID(r), chi.URLParam(r, "id")); err != nil {
			return err
		}
		return renderList(w, r, charID, svc)
	}
}

// GetRound returns the current round number.
func GetRound(charID CharacterIDFunc, svc buffSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		round, err := svc.GetRound(r.Context(), charID(r))
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, map[string]int{"round": round})
		return nil
	}
}

// AdvanceRound increments the counter and ages duration buffs.
func AdvanceRound(charID CharacterIDFunc, svc buffSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		active, expired, round, err := svc.AdvanceRound(r.Context(), charID(r))
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, RoundAdvanceDTO{
			RoundNumber:  round,
			ActiveBuffs:  toDTOs(active),
			ExpiredBuffs: toDTOs(expired),
		})
		return nil
	}
}

// ResetRound is End combat: the counter goes back to 0, buffs untouched.
func ResetRound(charID CharacterIDFunc, svc buffSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		active, round, err := svc.ResetRound(r.Context(), charID(r))
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, RoundAdvanceDTO{RoundNumber: round, ActiveBuffs: toDTOs(active), ExpiredBuffs: []BuffEffectDTO{}})
		return nil
	}
}

// SetRoundDTO is the PUT /api/round body.
type SetRoundDTO struct {
	Round *int `json:"round"`
}

// SetRound corrects the counter to a value >= 0, buffs untouched.
func SetRound(charID CharacterIDFunc, svc buffSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		var dto SetRoundDTO
		if err := json.NewDecoder(r.Body).Decode(&dto); err != nil || dto.Round == nil {
			return render.BadRequestError("invalid request body")
		}
		active, round, err := svc.SetRound(r.Context(), charID(r), *dto.Round)
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, RoundAdvanceDTO{RoundNumber: round, ActiveBuffs: toDTOs(active), ExpiredBuffs: []BuffEffectDTO{}})
		return nil
	}
}

// PassTimeDTO is the POST /api/round/pass-time body: game hours, decimals
// allowed (1.5 = 90 rounds).
type PassTimeDTO struct {
	Hours *float64 `json:"hours"`
}

// PassTime fast-forwards game time: timed buffs lose floor(hours × 60)
// rounds, the counter goes to 0. The response is Next Round's, plus the
// rounds that passed.
func PassTime(charID CharacterIDFunc, svc buffSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		var dto PassTimeDTO
		if err := json.NewDecoder(r.Body).Decode(&dto); err != nil || dto.Hours == nil {
			return render.BadRequestError("invalid request body")
		}
		active, expired, rounds, round, err := svc.PassTime(r.Context(), charID(r), *dto.Hours)
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, RoundAdvanceDTO{
			RoundNumber:  round,
			ActiveBuffs:  toDTOs(active),
			ExpiredBuffs: toDTOs(expired),
			RoundsPassed: rounds,
		})
		return nil
	}
}

// StrengthDTO is GET /api/stats/str.
type StrengthDTO struct {
	Base      int `json:"base"`
	Effective int `json:"effective"`
}

// GetEffectiveStr returns the sheet STR and the STR with buffs.
func GetEffectiveStr(charID CharacterIDFunc, svc buffSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		s, err := svc.GetEffectiveStr(r.Context(), charID(r))
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, StrengthDTO{Base: s.Base, Effective: s.Effective})
		return nil
	}
}

// BuffEffectDTO is the wire shape for a single buff (snake_case effect
// fields). duration is rounds left (0: not timed), charges what is left to
// absorb (0: none). applies is set on list responses: false while the buff's
// min_str is not met by the effective STR.
type BuffEffectDTO struct {
	ID       string         `json:"id"`
	Name     string         `json:"name"`
	Charges  int            `json:"charges"`
	Duration int            `json:"duration"`
	Effects  BuffEffectsDTO `json:"effects"`
	Note     string         `json:"note"`
	Applies  *bool          `json:"applies,omitempty"`
}

// BuffEffectsDTO is the modifier payload.
type BuffEffectsDTO struct {
	MinStr     *int     `json:"min_str,omitempty"`
	SpellsBuff *float64 `json:"spells_buff,omitempty"`
	WeaponBuff *int     `json:"weapon_buff,omitempty"`
	Multiplier *float64 `json:"multiplier,omitempty"`
	SaveBonus  *int     `json:"save_bonus,omitempty"`
	StrSet     *int     `json:"str_set,omitempty"`
	StrBonus   *int     `json:"str_bonus,omitempty"`
	StrMax     *int     `json:"str_max,omitempty"`
	AC         *int     `json:"ac,omitempty"`
	ToHit      *int     `json:"to_hit,omitempty"`
	Damage     *int     `json:"damage,omitempty"`
}

// RoundAdvanceDTO mirrors the previous response shape from RoundHandler.Advance.
type RoundAdvanceDTO struct {
	RoundNumber  int             `json:"roundNumber"`
	ActiveBuffs  []BuffEffectDTO `json:"activeBuffs"`
	ExpiredBuffs []BuffEffectDTO `json:"expiredBuffs"`
	// RoundsPassed is set by pass-time only: the rounds the hours gave.
	RoundsPassed int `json:"roundsPassed,omitempty"`
}

func (d BuffEffectDTO) toDomain() buffsvc.BuffEffect {
	return buffsvc.BuffEffect{
		ID: d.ID, Name: d.Name, Charges: d.Charges, Duration: d.Duration,
		Effects: buffsvc.BuffEffects(d.Effects), Note: d.Note,
	}
}

func toDTO(b buffsvc.BuffEffect) BuffEffectDTO {
	return BuffEffectDTO{
		ID: b.ID, Name: b.Name, Charges: b.Charges, Duration: b.Duration,
		Effects: BuffEffectsDTO(b.Effects), Note: b.Note,
	}
}

func toDTOs(bs []buffsvc.BuffEffect) []BuffEffectDTO {
	out := make([]BuffEffectDTO, 0, len(bs))
	for _, b := range bs {
		out = append(out, toDTO(b))
	}
	return out
}

func toStatusDTOs(bs []buffsvc.BuffStatus) []BuffEffectDTO {
	out := make([]BuffEffectDTO, 0, len(bs))
	for _, b := range bs {
		d := toDTO(b.BuffEffect)
		applies := b.Applies
		d.Applies = &applies
		out = append(out, d)
	}
	return out
}

// SavedBuffDTO is the wire shape of a saved buff template: the starting
// values of a buff (duration = rounds, charges), its effects and note.
type SavedBuffDTO struct {
	ID       int64          `json:"id"`
	Name     string         `json:"name"`
	Duration int            `json:"duration"`
	Charges  int            `json:"charges"`
	Effects  BuffEffectsDTO `json:"effects"`
	Note     string         `json:"note"`
}

func toSavedDTOs(list []buffsvc.SavedBuff) []SavedBuffDTO {
	out := make([]SavedBuffDTO, 0, len(list))
	for _, s := range list {
		out = append(out, SavedBuffDTO{
			ID: s.ID, Name: s.Name, Duration: s.Duration, Charges: s.Charges,
			Effects: BuffEffectsDTO(s.Effects), Note: s.Note,
		})
	}
	return out
}

// ListSaved returns the saved buff templates.
func ListSaved(charID CharacterIDFunc, svc buffSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		list, err := svc.ListSaved(r.Context(), charID(r))
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, toSavedDTOs(list))
		return nil
	}
}

// SaveBuff stores a template (same name replaces) and returns the list.
func SaveBuff(charID CharacterIDFunc, svc buffSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		var dto SavedBuffDTO
		if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
			return render.BadRequestError("invalid request body")
		}
		list, err := svc.SaveBuff(r.Context(), charID(r), buffsvc.SavedBuff{
			Name: dto.Name, Duration: dto.Duration, Charges: dto.Charges,
			Effects: buffsvc.BuffEffects(dto.Effects), Note: dto.Note,
		})
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, toSavedDTOs(list))
		return nil
	}
}

// DeleteSaved removes a template; active buffs are untouched.
func DeleteSaved(charID CharacterIDFunc, svc buffSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			return render.BadRequestError("invalid saved buff id")
		}
		list, err := svc.DeleteSaved(r.Context(), charID(r), id)
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, toSavedDTOs(list))
		return nil
	}
}
