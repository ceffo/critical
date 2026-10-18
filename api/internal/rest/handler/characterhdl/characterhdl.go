// Package characterhdl exposes /api/character (the current character's
// sheet) and /api/characters (the user's characters and which one is current,
// ADR 0029 step 7).
package characterhdl

import (
	"context"
	"encoding/json"
	"maps"
	"net/http"
	"slices"

	"github.com/google/uuid"

	"github.com/ceffo/critical/api/internal/domain/service/authsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/render"
	"github.com/ceffo/critical/api/internal/rest/handler/detailshdl"
	mw "github.com/ceffo/critical/api/internal/rest/middleware"
)

// Router is the chi subset we need.
type Router interface {
	Get(pattern string, h http.HandlerFunc)
	Put(pattern string, h http.HandlerFunc)
	Post(pattern string, h http.HandlerFunc)
	Patch(pattern string, h http.HandlerFunc)
}

// CharacterIDFunc resolves the active character ID per-request.
type CharacterIDFunc func(r *http.Request) int64

type characterGetter interface {
	GetCharacter(ctx context.Context, id int64) (*charactersvc.Character, error)
}

// RegisterRoutes wires the current character's endpoints onto r.
func RegisterRoutes(r Router, charID CharacterIDFunc, svc charactersvc.SVC) {
	r.Get("/api/character", render.Handle(GetCharacter(charID, svc)))
}

// RegisterUserRoutes wires the user's characters onto r: they need a
// session, not a current character (a new player has none yet).
func RegisterUserRoutes(r Router, svc charactersvc.SVC) {
	r.Get("/api/characters", render.Handle(List(svc)))
	r.Put("/api/characters/current", render.Handle(SetCurrent(svc)))
	r.Get("/api/characters/options", render.Handle(Options(svc)))
	r.Post("/api/characters", render.Handle(Create(svc)))
	r.Patch("/api/characters/{id}", render.Handle(Change(svc)))
}

type ownedLister interface {
	List(ctx context.Context, owner uuid.UUID) ([]charactersvc.Summary, error)
}

type currentSetter interface {
	ownedLister
	SetCurrent(ctx context.Context, owner uuid.UUID, id int64) error
}

// SummaryDTO is a character in its owner's list.
type SummaryDTO struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Class string `json:"class"`
	Level int    `json:"level"`
}

// CharactersDTO is the user's characters, not retired, in creation order, and
// the id of the current one (null: none).
type CharactersDTO struct {
	Characters []SummaryDTO `json:"characters"`
	CurrentID  *int64       `json:"currentId"`
}

// user is the request's user; the gate lets no request without one reach a
// data route.
func user(r *http.Request) (*authsvc.User, error) {
	p := mw.PrincipalFrom(r.Context())
	if p == nil {
		return nil, authsvc.ErrUnauthenticated
	}
	return &p.User, nil
}

func renderCharacters(w http.ResponseWriter, r *http.Request, svc ownedLister, owner uuid.UUID, current int64) error {
	return renderCharactersStatus(w, r, svc, owner, current, http.StatusOK)
}

func renderCharactersStatus(w http.ResponseWriter, r *http.Request, svc ownedLister, owner uuid.UUID, current int64, status int) error {
	list, err := svc.List(r.Context(), owner)
	if err != nil {
		return err
	}
	out := CharactersDTO{Characters: make([]SummaryDTO, 0, len(list))}
	for _, c := range list {
		out.Characters = append(out.Characters, SummaryDTO{ID: c.ID, Name: c.Name, Class: c.Class, Level: c.Level})
		if c.ID == current {
			id := c.ID
			out.CurrentID = &id
		}
	}
	render.JSON(r.Context(), w, status, out)
	return nil
}

// List is GET /api/characters: the user's own characters (ADR 0029 steps
// 7.1 to 7.3).
func List(svc ownedLister) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		u, err := user(r)
		if err != nil {
			return err
		}
		return renderCharacters(w, r, svc, u.ID, u.CurrentCharacterID)
	}
}

// CurrentBody is the body of PUT /api/characters/current.
type CurrentBody struct {
	ID *int64 `json:"id"`
}

// SetCurrent is PUT /api/characters/current {id}: the user's character id
// becomes current (ADR 0029 step 7.2); another user's character is 404, as an
// unknown id (step 7.1). It answers with the list, as GET /api/characters.
func SetCurrent(svc currentSetter) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		u, err := user(r)
		if err != nil {
			return err
		}
		var b CurrentBody
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		if err := dec.Decode(&b); err != nil {
			return render.BadRequestError("invalid JSON body")
		}
		if b.ID == nil {
			return render.BadRequestError("id is required")
		}
		if err := svc.SetCurrent(r.Context(), u.ID, *b.ID); err != nil {
			return err
		}
		return renderCharacters(w, r, svc, u.ID, *b.ID)
	}
}

// GetCharacter renders the character stats DTO.
func GetCharacter(charID CharacterIDFunc, svc characterGetter) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		c, err := svc.GetCharacter(r.Context(), charID(r))
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, toDTO(c))
		return nil
	}
}

// CharacterDTO is the current character's sheet: the entered values and the
// derived block (ADR 0030 steps 3 to 8). `race` is the free-text detail the
// player edits, `raceCode` the PHB race the rules read ("" for none).
type CharacterDTO struct {
	ID       int64      `json:"id"`
	Name     string     `json:"name"`
	Str      int        `json:"str"`
	Dex      int        `json:"dex"`
	Con      int        `json:"con"`
	Int      int        `json:"int"`
	Wis      int        `json:"wis"`
	Cha      int        `json:"cha"`
	HPMax    int        `json:"hpMax"`
	XPTotal  int        `json:"xpTotal"`
	Class    string     `json:"class"`
	RaceCode string     `json:"raceCode"`
	Derived  DerivedDTO `json:"derived"`
	detailshdl.DetailsDTO
}

// SavesDTO are five numbers, one per save category.
type SavesDTO struct {
	ParaPoison   int `json:"paraPoison"`
	RodStaffWand int `json:"rodStaffWand"`
	PetriPoly    int `json:"petriPoly"`
	Breath       int `json:"breath"`
	Spell        int `json:"spell"`
}

// SlotCountDTO is the slots per day of one spell level.
type SlotCountDTO struct {
	Level int `json:"level"`
	Count int `json:"count"`
}

// SheetDifferenceDTO is a value the imported sheet states differently from
// the rules (ADR 0030 step 24): `value` is a code the web words.
type SheetDifferenceDTO struct {
	Value    string `json:"value"`
	Sheet    int    `json:"sheet"`
	Computed int    `json:"computed"`
}

// DerivedDTO is the values the rules derive (ADR 0030 steps 3 to 8, 22.1 and
// 24). `xpTotal` is the starting XP plus the ledger, `nextLevelXp` null at the
// last level, `raceSaveBonus` the race's bonus per category (0: none), the
// slots by ascending spell level (levels without a slot left out).
type DerivedDTO struct {
	XPTotal          int                  `json:"xpTotal"`
	Level            int                  `json:"level"`
	NextLevelXP      *int                 `json:"nextLevelXp"`
	THAC0            int                  `json:"thac0"`
	Saves            SavesDTO             `json:"saves"`
	RaceSaveBonus    SavesDTO             `json:"raceSaveBonus"`
	SpellSlots       []SlotCountDTO       `json:"spellSlots"`
	SheetDifferences []SheetDifferenceDTO `json:"sheetDifferences"`
	LevelUp          LevelUpDTO           `json:"levelUp"`
}

// PendingLevelDTO is one level whose hit points are owed (ADR 0030 step 7):
// `die` the hit die's sides to roll up to the name level (0 above it),
// `fixedGain` the gain above it (0 up to it).
type PendingLevelDTO struct {
	Level     int `json:"level"`
	Die       int `json:"die"`
	FixedGain int `json:"fixedGain"`
}

// LevelUpDTO is ADR 0030 step 7: `hpLevel` the level the max HP was last set
// for, `pending` the levels to roll, lowest first (the prompt, one per
// level), `hpAboveLevel` an XP correction brought the level under `hpLevel`
// (step 7.4, the card warns).
type LevelUpDTO struct {
	HPLevel      int               `json:"hpLevel"`
	Pending      []PendingLevelDTO `json:"pending"`
	HPAboveLevel bool              `json:"hpAboveLevel"`
}

// PendingDTO maps the pending levels, never null.
func PendingDTO(p []charactersvc.PendingLevel) []PendingLevelDTO {
	out := make([]PendingLevelDTO, 0, len(p))
	for _, l := range p {
		out = append(out, PendingLevelDTO{Level: l.Level, Die: l.Die, FixedGain: l.FixedGain})
	}
	return out
}

func savesDTO(s charactersvc.Saves) SavesDTO {
	return SavesDTO{ParaPoison: s.ParaPoison, RodStaffWand: s.RodStaffWand, PetriPoly: s.PetriPoly, Breath: s.Breath, Spell: s.Spell}
}

func derivedDTO(d charactersvc.Derived) DerivedDTO {
	out := DerivedDTO{
		XPTotal: d.XPTotal, Level: d.Level, NextLevelXP: d.NextLevelXP, THAC0: d.THAC0,
		Saves: savesDTO(d.Saves), RaceSaveBonus: savesDTO(d.RaceSaveBonus),
		SpellSlots: []SlotCountDTO{}, SheetDifferences: []SheetDifferenceDTO{},
		LevelUp: LevelUpDTO{
			HPLevel: d.LevelUp.HPLevel, Pending: PendingDTO(d.LevelUp.Pending), HPAboveLevel: d.LevelUp.HPAboveLevel,
		},
	}
	for _, level := range slices.Sorted(maps.Keys(d.SpellSlots)) {
		if n := d.SpellSlots[level]; n > 0 {
			out.SpellSlots = append(out.SpellSlots, SlotCountDTO{Level: level, Count: n})
		}
	}
	for _, diff := range d.SheetDifferences {
		out.SheetDifferences = append(out.SheetDifferences, SheetDifferenceDTO{Value: string(diff.Value), Sheet: diff.Sheet, Computed: diff.Computed})
	}
	return out
}

func toDTO(c *charactersvc.Character) CharacterDTO {
	if c == nil {
		return CharacterDTO{}
	}
	return CharacterDTO{
		ID: c.ID, Name: c.Name,
		Str: c.Str, Dex: c.Dex, Con: c.Con, Int: c.Int, Wis: c.Wis, Cha: c.Cha,
		HPMax: c.HPMax, XPTotal: c.XPTotal, Class: c.Class, RaceCode: c.Race,
		Derived: derivedDTO(c.Derived), DetailsDTO: detailshdl.ToDTO(c.Details),
	}
}
