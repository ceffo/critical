// Package buffhdl exposes /api/buffs, /api/round and /api/stats/str.
package buffhdl

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/render"
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
	ListPowers(ctx context.Context, characterID int64) ([]buffsvc.ItemPower, error)
	Activate(ctx context.Context, characterID int64, buff buffsvc.BuffEffect) ([]buffsvc.BuffEffect, error)
	Edit(ctx context.Context, characterID int64, buffID string, e buffsvc.BuffEdit) ([]buffsvc.BuffEffect, error)
	Dismiss(ctx context.Context, characterID int64, buffID string) ([]buffsvc.BuffEffect, error)
	Trigger(ctx context.Context, characterID int64, buffID string) ([]buffsvc.BuffEffect, error)
	GetRound(ctx context.Context, characterID int64) (int, error)
	GetInitiative(ctx context.Context, characterID int64) (*buffsvc.Initiative, error)
	RerollInitiative(ctx context.Context, characterID int64) (buffsvc.Initiative, error)
	AdvanceRound(ctx context.Context, characterID int64) (active, expired []buffsvc.BuffEffect, round int, initiative buffsvc.Initiative, err error)
	ResetRound(ctx context.Context, characterID int64) (active []buffsvc.BuffEffect, round int, err error)
	SetRound(ctx context.Context, characterID int64, to int) (active []buffsvc.BuffEffect, round int, err error)
	PassTime(ctx context.Context, characterID int64, hours float64) (active, expired []buffsvc.BuffEffect, rounds, round int, err error)
	PreviewPassTime(ctx context.Context, characterID int64, hours float64) (rounds int, expiring []buffsvc.BuffEffect, err error)
	GetEffectiveStr(ctx context.Context, characterID int64) (buffsvc.Strength, error)
	ListSaved(ctx context.Context, characterID int64) ([]buffsvc.SavedBuff, error)
	SaveBuff(ctx context.Context, characterID int64, saved buffsvc.SavedBuff) ([]buffsvc.SavedBuff, error)
	DeleteSaved(ctx context.Context, characterID, id int64) ([]buffsvc.SavedBuff, error)
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
	r.Post("/api/round/initiative/reroll", render.Handle(RerollInitiative(charID, svc)))
	r.Put("/api/round", render.Handle(SetRound(charID, svc)))
	r.Post("/api/round/pass-time", render.Handle(PassTime(charID, svc)))
	r.Get("/api/round/pass-time/preview", render.Handle(PreviewPassTime(charID, svc)))
	r.Get("/api/stats/str", render.Handle(GetEffectiveStr(charID, svc)))
}

// renderList writes the effects list: the active buffs with each one's
// applies flag, and the item powers that are on.
func renderList(w http.ResponseWriter, r *http.Request, charID CharacterIDFunc, svc buffSvc) error {
	buffs, err := svc.ListStatus(r.Context(), charID(r))
	if err != nil {
		return err
	}
	powers, err := svc.ListPowers(r.Context(), charID(r))
	if err != nil {
		return err
	}
	render.OK(r.Context(), w, BuffListDTO{Buffs: toStatusDTOs(buffs), Powers: toPowerDTOs(powers)})
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

// InitiativeDTO is the round's initiative: the d10 (lowest acts first) and
// the log entry that rolled it.
type InitiativeDTO struct {
	Value      int   `json:"value"`
	LogEntryID int64 `json:"logEntryId"`
}

func toInitiativeDTO(i *buffsvc.Initiative) *InitiativeDTO {
	if i == nil {
		return nil
	}
	return &InitiativeDTO{Value: i.D10, LogEntryID: i.LogEntryID}
}

// RoundDTO is GET /api/round: the counter and its initiative (null out of
// combat).
type RoundDTO struct {
	Round      int            `json:"round"`
	Initiative *InitiativeDTO `json:"initiative"`
}

// GetRound returns the current round number and its initiative.
func GetRound(charID CharacterIDFunc, svc buffSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		round, err := svc.GetRound(r.Context(), charID(r))
		if err != nil {
			return err
		}
		initiative, err := svc.GetInitiative(r.Context(), charID(r))
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, RoundDTO{Round: round, Initiative: toInitiativeDTO(initiative)})
		return nil
	}
}

// AdvanceRound increments the counter, ages duration buffs and rolls the new
// round's initiative.
func AdvanceRound(charID CharacterIDFunc, svc buffSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		active, expired, round, initiative, err := svc.AdvanceRound(r.Context(), charID(r))
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, RoundAdvanceDTO{
			RoundNumber:  round,
			ActiveBuffs:  toDTOs(active),
			ExpiredBuffs: toDTOs(expired),
			Initiative:   toInitiativeDTO(&initiative),
		})
		return nil
	}
}

// RerollInitiative pops the round's initiative entry and rolls a new one;
// 409 when there is none or another entry is newer.
func RerollInitiative(charID CharacterIDFunc, svc buffSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		initiative, err := svc.RerollInitiative(r.Context(), charID(r))
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, toInitiativeDTO(&initiative))
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

// PassTimePreviewDTO is GET /api/round/pass-time/preview: the rounds the
// hours give and the buffs that would expire (nothing is written).
type PassTimePreviewDTO struct {
	Rounds   int             `json:"rounds"`
	Expiring []BuffEffectDTO `json:"expiring"`
}

// PreviewPassTime answers what Time passes would do for ?hours= (decimal,
// "." separator): invalid hours are a 400 with the same message as the
// POST.
func PreviewPassTime(charID CharacterIDFunc, svc buffSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		hours, err := strconv.ParseFloat(r.URL.Query().Get("hours"), 64)
		if err != nil {
			return render.BadRequestError("invalid hours")
		}
		rounds, expiring, err := svc.PreviewPassTime(r.Context(), charID(r), hours)
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, PassTimePreviewDTO{Rounds: rounds, Expiring: toDTOs(expiring)})
		return nil
	}
}

// StrengthDTO is GET /api/stats/str.
// baseExceptional and effectiveExceptional are their 18/xx percentiles
// (ADR 0030 steps 9 and 10; 0: none, 100: 18/00).
type StrengthDTO struct {
	Base                 int `json:"base"`
	Effective            int `json:"effective"`
	BaseExceptional      int `json:"baseExceptional"`
	EffectiveExceptional int `json:"effectiveExceptional"`
}

// GetEffectiveStr returns the sheet STR and the STR with buffs.
func GetEffectiveStr(charID CharacterIDFunc, svc buffSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		s, err := svc.GetEffectiveStr(r.Context(), charID(r))
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, StrengthDTO{
			Base: s.Base, Effective: s.Effective,
			BaseExceptional: s.BaseExceptional, EffectiveExceptional: s.EffectiveExceptional,
		})
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
	// StartRound is the round the buff was added in (0: out of combat or
	// unknown). Response only: a request's value is ignored, the server
	// stamps it.
	StartRound int `json:"startRound"`
	// ChargesTotal is the charge count the buff started with (0: unknown, or
	// no charges). Response only, like startRound.
	ChargesTotal int `json:"chargesTotal"`
	// SpellID is the catalogue id of the spell the buff comes from, omitted
	// for a buff not from a spell (the client names it from the catalogue,
	// name otherwise). Response only, like startRound: a request's value is
	// ignored, only a spell cast stamps it.
	SpellID string `json:"spellId,omitempty"`
	Applies *bool  `json:"applies,omitempty"`
}

// BuffEffectsDTO is the modifier payload.
type BuffEffectsDTO struct {
	MinStr     *int     `json:"min_str,omitempty"`
	SpellsBuff *float64 `json:"spells_buff,omitempty"`
	WeaponBuff *int     `json:"weapon_buff,omitempty"`
	Multiplier *float64 `json:"multiplier,omitempty"`
	SaveBonus  *int     `json:"save_bonus,omitempty"`
	StrBonus   *int     `json:"str_bonus,omitempty"`
	StrMax     *int     `json:"str_max,omitempty"`
	AC         *int     `json:"ac,omitempty"`
	ToHit      *int     `json:"to_hit,omitempty"`
	Damage     *int     `json:"damage,omitempty"`
	// WeaponKind is the weapon-kind condition of the other effects, melee or
	// missile (ADR 0004 step 18); omitted: every roll.
	WeaponKind buffsvc.WeaponKind `json:"weapon_kind,omitempty"`
}

// BuffListDTO is the effects list of GET /api/buffs, also answered by every
// buff change: the active buffs, each with its applies flag, and in a list
// of their own the carried items' powers that are on (ADR 0016 step 13),
// never mixed into the buffs.
type BuffListDTO struct {
	Buffs  []BuffEffectDTO `json:"buffs"`
	Powers []ItemPowerDTO  `json:"powers"`
}

// ItemPowerDTO is an item power that is on: its item's id (the client
// names it from the item), its id within the item and its effects. No
// counters, nothing to edit or dismiss.
type ItemPowerDTO struct {
	ItemID  string         `json:"itemId"`
	PowerID string         `json:"powerId"`
	Effects BuffEffectsDTO `json:"effects"`
}

func toPowerDTOs(ps []buffsvc.ItemPower) []ItemPowerDTO {
	out := make([]ItemPowerDTO, 0, len(ps))
	for _, p := range ps {
		out = append(out, ItemPowerDTO{ItemID: p.ItemID, PowerID: p.PowerID, Effects: BuffEffectsDTO(p.Effects)})
	}
	return out
}

// RoundAdvanceDTO mirrors the previous response shape from RoundHandler.Advance.
type RoundAdvanceDTO struct {
	RoundNumber  int             `json:"roundNumber"`
	ActiveBuffs  []BuffEffectDTO `json:"activeBuffs"`
	ExpiredBuffs []BuffEffectDTO `json:"expiredBuffs"`
	// RoundsPassed is set by pass-time only: the rounds the hours gave.
	RoundsPassed int `json:"roundsPassed,omitempty"`
	// Initiative is set by Next Round only: the new round's roll.
	Initiative *InitiativeDTO `json:"initiative,omitempty"`
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
		Effects: BuffEffectsDTO(b.Effects), Note: b.Note, StartRound: b.StartRound, ChargesTotal: b.ChargesTotal,
		SpellID: b.SpellID,
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
