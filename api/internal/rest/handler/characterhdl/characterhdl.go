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
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
	"github.com/ceffo/critical/api/internal/render"
	mw "github.com/ceffo/critical/api/internal/rest/middleware"
	"github.com/ceffo/critical/api/internal/rest/wire"
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
	r.Post("/api/characters/preview", render.Handle(Preview(svc)))
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
	// Race is the race code ("" when none); Alignment and Sex are codes, ""
	// while still to choose, and Missing names those fields (the card's tag).
	Race      string   `json:"race"`
	Alignment string   `json:"alignment"`
	Sex       string   `json:"sex"`
	Missing   []string `json:"missing"`
	// CurrentHP and MaxHP come from the HP ledger and the sheet's max HP.
	CurrentHP int `json:"currentHp"`
	MaxHP     int `json:"maxHp"`
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
		out.Characters = append(out.Characters, SummaryDTO{
			ID: c.ID, Name: c.Name, Class: c.Class, Level: c.Level, Race: c.Race, Alignment: c.Alignment,
			Sex: c.Sex, Missing: c.Missing, CurrentHP: c.CurrentHP, MaxHP: c.MaxHP,
		})
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
// derived block (ADR 0030 steps 3 to 8). `raceCode` is the PHB race the rules
// read ("" for none).
type CharacterDTO struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Str  int    `json:"str"`
	// StrExceptional is a fighter's 18/xx percentile (ADR 0030 step 9), 0
	// for none.
	StrExceptional int        `json:"strExceptional"`
	Dex            int        `json:"dex"`
	Con            int        `json:"con"`
	Int            int        `json:"int"`
	Wis            int        `json:"wis"`
	Cha            int        `json:"cha"`
	HPMax          int        `json:"hpMax"`
	XPTotal        int        `json:"xpTotal"`
	Class          string     `json:"class"`
	RaceCode       string     `json:"raceCode"`
	Derived        DerivedDTO `json:"derived"`
	// Missing lists the required fields the character lacks (ADR 0032 step
	// 8.2: "alignment", "sex"); the card asks the player to pick them.
	Missing []string `json:"missing"`
	// Warnings are the rules the stored character breaks without being
	// refused (step 8.4: an alignment its class does not admit).
	Warnings []WarningDTO `json:"warnings"`
	wire.DetailsDTO
}

// WarningDTO is a code and the field it is about; the web words it. The
// numbers outside their range (ADR 0032 step 5.2) carry value, min and max;
// the languages over the limit (ADR 0032 step 4.2) also carry how many
// additional languages the character knows (count) and its Intelligence
// allows (limit).
type WarningDTO struct {
	Code  string `json:"code"`
	Field string `json:"field"`
	Count *int   `json:"count,omitempty"`
	Limit *int   `json:"limit,omitempty"`
	// Value, Min and Max are a height (cm), weight (kg) or age (years) outside
	// the race's table range (ADR 0032 step 5.2).
	Value *float64 `json:"value,omitempty"`
	Min   *float64 `json:"min,omitempty"`
	Max   *float64 `json:"max,omitempty"`
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

// DerivedDTO is the values the rules derive (ADR 0030 steps 3 to 8, and 22.1).
// `xpTotal` is the starting XP plus the ledger, `targetLevel` the level
// it gives (step 3, never stored), `realizedLevel` the level taken (step 7),
// which THAC0, saves and slots follow, `nextLevelXp` (the target's next
// level) null at the last level, `raceSaveBonus` the race's bonus per category (0: none), the
// slots by ascending spell level (levels without a slot left out).
type DerivedDTO struct {
	XPTotal       int            `json:"xpTotal"`
	TargetLevel   int            `json:"targetLevel"`
	RealizedLevel int            `json:"realizedLevel"`
	NextLevelXP   *int           `json:"nextLevelXp"`
	THAC0         int            `json:"thac0"`
	Saves         SavesDTO       `json:"saves"`
	RaceSaveBonus SavesDTO       `json:"raceSaveBonus"`
	SpellSlots    []SlotCountDTO `json:"spellSlots"`
	// SheetDifferences is retired (ADR 0030 step 24) and always empty. The
	// previous release's web reads `.length` on it, so the field stays one
	// release for the rolling deploy; drop it after.
	SheetDifferences []struct{} `json:"sheetDifferences"`
	LevelUp          LevelUpDTO `json:"levelUp"`
	// Specializes is ADR 0030 step 12: the class may specialize in a weapon.
	Specializes bool `json:"specializes"`
	// Spellbook is ADR 0030 step 20.1: the learning rules of a class with a
	// spellbook (mage, bard); null for the others.
	Spellbook *SpellbookRulesDTO `json:"spellbook"`
	// SpellFailure is ADR 0030 step 14: the percent chance a cast fails (0:
	// no die).
	SpellFailure int `json:"spellFailure"`
	// Turning is ADR 0030 step 16: PHB Table 61 at the realized level, the
	// book's rows in order; null for a class that does not turn undead.
	Turning []TurningTargetDTO `json:"turning"`
	// ThiefSkills is ADR 0030 step 17: each thieving skill with its terms
	// and percentage; null for a class without them.
	ThiefSkills []ThiefSkillDTO `json:"thiefSkills"`
	// BardSkills is ADR 0030 step 18: each bard skill with its terms and
	// percentage; null for a class without them.
	BardSkills []ThiefSkillDTO `json:"bardSkills"`
	// SkillArmor is the skill armour the thief's or the bard's skills read
	// (ADR 0033 step 14: from the worn body armour's Table 29 column; none,
	// leather, padded_studded_leather, elven_chain or metal), "" for a class
	// without them.
	SkillArmor string `json:"skillArmor"`
	// SkillBudget is step 17's or 18's warnings on the entered points,
	// never null.
	SkillBudget []BudgetWarningDTO `json:"skillBudget"`
	// Backstab is ADR 0030 step 19: PHB Table 30's damage multiplier at the
	// realized level, 0 for a class without the backstab option.
	Backstab int `json:"backstab"`
}

// skillDTOs maps a class's skills; nil stays nil (no such skills).
func skillDTOs(skills []ruletablesvc.SkillPercent) []ThiefSkillDTO {
	if skills == nil {
		return nil
	}
	out := make([]ThiefSkillDTO, len(skills))
	for i, s := range skills {
		out[i] = ThiefSkillDTO(s)
	}
	return out
}

// ThiefSkillDTO is one thieving skill (ADR 0030 step 17) or bard skill (step
// 18): percent = base + points + race + dexterity + armor, capped at 95 and
// floored at 0.
type ThiefSkillDTO struct {
	Skill     string `json:"skill"`
	Base      int    `json:"base"`
	Points    int    `json:"points"`
	Race      int    `json:"race"`
	Dexterity int    `json:"dexterity"`
	Armor     int    `json:"armor"`
	Percent   int    `json:"percent"`
}

// BudgetWarningDTO is a breach of the skill point budget (ADR 0030 step 17):
// `code` skill_points_total (`skill` empty) or skill_points_per_skill, the
// points and the limit at the realized level.
type BudgetWarningDTO struct {
	Code   string `json:"code"`
	Skill  string `json:"skill,omitempty"`
	Points int    `json:"points"`
	Limit  int    `json:"limit"`
}

// TurningTargetDTO is one undead type a cleric may try to turn (ADR 0030
// step 16): its code, its Hit Dice (`hdMin` 0: by type only, `hdMax` 0: and
// more) and the cell at the cleric's level, `roll` the d20 target or
// `outcome` (turn, destroy, destroy_more, cannot) when no die is rolled.
type TurningTargetDTO struct {
	Undead  string `json:"undead"`
	HDMin   int    `json:"hdMin"`
	HDMax   int    `json:"hdMax"`
	Roll    int    `json:"roll,omitempty"`
	Outcome string `json:"outcome,omitempty"`
}

// SpellbookRulesDTO is what a character may write in its spellbook (ADR 0030
// step 20.1): the class's and PHB Table 4's highest spell level, Table 4's
// chance to learn (percent) and its maximum spells per level
// (`noMaximum`: the book's "All", `maxSpellsPerLevel` then 0).
type SpellbookRulesDTO struct {
	ClassMaxSpellLevel int  `json:"classMaxSpellLevel"`
	IntMaxSpellLevel   int  `json:"intMaxSpellLevel"`
	LearnChance        int  `json:"learnChance"`
	MaxSpellsPerLevel  int  `json:"maxSpellsPerLevel"`
	NoMaximum          bool `json:"noMaximum"`
}

// LevelUpDTO is ADR 0030 step 7: `pending` the levels from the realized one
// to the target, lowest first (the prompt, one per level),
// `realizedAboveTarget` an XP correction brought the target under the
// realized level (step 7.4, the card warns).
type LevelUpDTO struct {
	Pending             []wire.PendingLevelDTO `json:"pending"`
	RealizedAboveTarget bool                   `json:"realizedAboveTarget"`
}

func savesDTO(s charactersvc.Saves) SavesDTO {
	return SavesDTO{ParaPoison: s.ParaPoison, RodStaffWand: s.RodStaffWand, PetriPoly: s.PetriPoly, Breath: s.Breath, Spell: s.Spell}
}

func derivedDTO(d charactersvc.Derived) DerivedDTO {
	out := DerivedDTO{
		XPTotal: d.XPTotal, TargetLevel: d.TargetLevel, RealizedLevel: d.RealizedLevel, NextLevelXP: d.NextLevelXP, THAC0: d.THAC0,
		Saves: savesDTO(d.Saves), RaceSaveBonus: savesDTO(d.RaceSaveBonus),
		SpellSlots: []SlotCountDTO{}, SheetDifferences: []struct{}{},
		LevelUp: LevelUpDTO{
			Pending: wire.PendingDTO(d.LevelUp.Pending), RealizedAboveTarget: d.LevelUp.RealizedAboveTarget,
		},
		Specializes: d.Specializes, SpellFailure: d.SpellFailure,
		SkillBudget: make([]BudgetWarningDTO, 0, len(d.SkillBudget)), Backstab: d.Backstab, SkillArmor: d.SkillArmor,
	}
	out.ThiefSkills, out.BardSkills = skillDTOs(d.ThiefSkills), skillDTOs(d.BardSkills)
	for _, w := range d.SkillBudget {
		out.SkillBudget = append(out.SkillBudget, BudgetWarningDTO(w))
	}
	if d.Turning != nil {
		out.Turning = make([]TurningTargetDTO, len(d.Turning))
		for i, r := range d.Turning {
			out.Turning[i] = TurningTargetDTO{Undead: r.Undead, HDMin: r.HDMin, HDMax: r.HDMax, Roll: r.Cell.Roll, Outcome: r.Cell.Outcome}
		}
	}
	if b := d.Spellbook; b != nil {
		out.Spellbook = &SpellbookRulesDTO{
			ClassMaxSpellLevel: b.ClassMaxSpellLevel, IntMaxSpellLevel: b.IntMaxSpellLevel, LearnChance: b.LearnChance,
			MaxSpellsPerLevel: b.MaxSpellsPerLevel, NoMaximum: b.NoMaximum,
		}
	}
	for _, level := range slices.Sorted(maps.Keys(d.SpellSlots)) {
		if n := d.SpellSlots[level]; n > 0 {
			out.SpellSlots = append(out.SpellSlots, SlotCountDTO{Level: level, Count: n})
		}
	}
	return out
}

func toDTO(c *charactersvc.Character) CharacterDTO {
	if c == nil {
		return CharacterDTO{}
	}
	return CharacterDTO{
		ID: c.ID, Name: c.Name,
		Str: c.Str, StrExceptional: c.StrExceptional, Dex: c.Dex, Con: c.Con, Int: c.Int, Wis: c.Wis, Cha: c.Cha,
		HPMax: c.HPMax, XPTotal: c.XPTotal, Class: c.Class, RaceCode: c.Race,
		Derived: derivedDTO(c.Derived), DetailsDTO: wire.DetailsToDTO(c.Details),
		Missing: c.Details.Missing(), Warnings: warningsDTO(c.Warnings),
	}
}

func warningsDTO(ws []charactersvc.Warning) []WarningDTO {
	out := make([]WarningDTO, 0, len(ws))
	for _, w := range ws {
		out = append(out, WarningDTO(w))
	}
	return out
}
