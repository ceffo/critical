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

	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/saves"
	"github.com/ceffo/critical/api/internal/domain/service/authsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/skills"
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

// RoundPower is the power used in the current round (ADR 0043 step 12), "" when the round is free;
// psionicsvc.SVC satisfies it.
type RoundPower interface {
	PowerThisRound(ctx context.Context, characterID int64) (string, error)
}

// RegisterRoutes wires the current character's endpoints onto r. round, when not nil, fills the
// known powers' `usedThisRound` of a psionicist's sheet.
func RegisterRoutes(r Router, charID CharacterIDFunc, svc charactersvc.SVC, round RoundPower) {
	r.Get("/api/character", render.Handle(GetCharacter(charID, svc, round)))
}

// RegisterUserRoutes wires the user's characters onto r: they need a
// session, not a current character (a new player has none yet).
func RegisterUserRoutes(r Router, svc charactersvc.SVC) {
	r.Get("/api/characters", render.Handle(List(svc)))
	r.Put("/api/characters/current", render.Handle(SetCurrent(svc)))
	r.Get("/api/characters/options", render.Handle(Options(svc)))
	r.Post("/api/characters", render.Handle(Create(svc)))
	r.Post("/api/characters/draft", render.Handle(Draft(svc)))
	r.Patch("/api/characters/{id}", render.Handle(Change(svc)))
}

type ownedLister interface {
	List(ctx context.Context, owner uuid.UUID) ([]charactersvc.Summary, error)
}

type currentSetter interface {
	ownedLister
	SetCurrent(ctx context.Context, owner uuid.UUID, id int64) error
}

// ClassEntryDTO is one class of a character (ADR 0042 step 1): the class
// code, the level taken in it, the XP entered for it, whether it is active or
// frozen and, for a frozen one, the level it was left at (0 otherwise).
type ClassEntryDTO struct {
	Class       string `json:"class"`
	Level       int    `json:"level"`
	StartingXP  int    `json:"startingXp"`
	State       string `json:"state"`
	LeftAtLevel int    `json:"leftAtLevel"`
}

func classEntriesDTO(entries []classes.Entry) []ClassEntryDTO {
	out := make([]ClassEntryDTO, len(entries))
	for i, e := range entries {
		out[i] = ClassEntryDTO{
			Class: e.Class, Level: e.RealizedLevel, StartingXP: e.StartingXP, State: string(e.State), LeftAtLevel: e.LeftAtLevel,
		}
	}
	return out
}

// SummaryDTO is a character in its owner's list.
type SummaryDTO struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	// Classes are the class entries in the order the classes were taken, each
	// with its realized level (ADR 0042 step 1, ADR 0030 step 3).
	Classes []ClassEntryDTO `json:"classes"`
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
			ID: c.ID, Name: c.Name, Classes: classEntriesDTO(c.Classes), Race: c.Race, Alignment: c.Alignment,
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
func GetCharacter(charID CharacterIDFunc, svc characterGetter, round RoundPower) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		c, err := svc.GetCharacter(r.Context(), charID(r))
		if err != nil {
			return err
		}
		dto := toDTO(c)
		// Only a psionicist pays the log read (ADR 0043 step 12: one power a round, a warning).
		if k := dto.Derived.KnownPowers; k != nil && round != nil {
			if k.UsedThisRound, err = round.PowerThisRound(r.Context(), c.ID); err != nil {
				return err
			}
		}
		render.OK(r.Context(), w, dto)
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
	StrExceptional int             `json:"strExceptional"`
	Dex            int             `json:"dex"`
	Con            int             `json:"con"`
	Int            int             `json:"int"`
	Wis            int             `json:"wis"`
	Cha            int             `json:"cha"`
	HPMax          int             `json:"hpMax"`
	XPTotal        int             `json:"xpTotal"`
	Classes        []ClassEntryDTO `json:"classes"`
	RaceCode       string          `json:"raceCode"`
	Derived        DerivedDTO      `json:"derived"`
	// Missing lists the required fields the character lacks (ADR 0032 step
	// 8.2: "alignment", "sex"); the card asks the player to pick them.
	Missing []string `json:"missing"`
	// Warnings are the rules the stored character breaks without being
	// refused (step 8.4: an alignment its class does not admit).
	Warnings []WarningDTO `json:"warnings"`
	wire.DetailsDTO
}

// WarningDTO is a warning of the character (wire.WarningDTO).
type WarningDTO = wire.WarningDTO

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
	LevelUp       LevelUpDTO     `json:"levelUp"`
	// Classes are the class entries' own figures (ADR 0042 steps 5 and 7), in
	// the order of the character's `classes`; absent for a single-class
	// character, whose figures are the ones above.
	Classes []EntryDerivedDTO `json:"classes,omitempty"`
	// Specializes is ADR 0030 step 12: the class may specialize in a weapon.
	Specializes bool `json:"specializes"`
	// ExceptionalStrength is ADR 0030 steps 9 and 10: the class enters an
	// 18/xx and reads the warrior's scale over 18 (its exceptional_strength
	// feature, ADR 0040 step 3).
	ExceptionalStrength bool `json:"exceptionalStrength"`
	// Spellbook is ADR 0030 step 20.1: the learning rules of a class with a
	// spellbook (mage, bard); null for the others.
	Spellbook *SpellbookRulesDTO `json:"spellbook"`
	// PSP is ADR 0043 step 5: the PSP pool of a character with the psionics
	// feature (current and max); null for the others.
	PSP *PSPDTO `json:"psp"`
	// KnownPowers is ADR 0043 step 9: Table 4's allowance at the psionics
	// entry's level, what the picks take, the primary discipline and the picks
	// in order; null for a character with no psionics feature.
	KnownPowers *KnownPowersDTO `json:"knownPowers"`
	// SpellFailure is ADR 0030 step 14: the percent chance a cast fails (0:
	// no die).
	SpellFailure int `json:"spellFailure"`
	// Turning is ADR 0030 step 16: PHB Table 61 at the realized level, the
	// book's rows in order; null for a class that does not turn undead.
	Turning []TurningTargetDTO `json:"turning"`
	// SkillSets is ADR 0030 steps 17 and 18 (ADR 0040 step 3.5): one set
	// per rogue_skills feature the class grants at the realized level, each
	// with its own budget; empty for a class without skill sets.
	SkillSets []SkillSetDTO `json:"skillSets"`
	// Backstab is ADR 0030 step 19: PHB Table 30's damage multiplier at the
	// realized level, 0 for a class without the backstab option.
	Backstab int `json:"backstab"`
	// ProficiencySlots is ADR 0037 steps 3 to 5 and 7: the non-weapon and the
	// weapon pool with their terms and each pick's figures; null without the
	// catalogue's Table 34.
	ProficiencySlots *wire.ProficiencySlotsDTO `json:"proficiencySlots"`
}

// EntryDerivedDTO is one class entry's figures: `xp` its starting XP plus its
// share of the ledger, `targetLevel` the level that XP gives and `nextLevelXp`
// the XP that reaches the next one (0 and null for a frozen entry), `level`
// the level taken.
type EntryDerivedDTO struct {
	Class       string `json:"class"`
	State       string `json:"state"`
	XP          int    `json:"xp"`
	TargetLevel int    `json:"targetLevel"`
	Level       int    `json:"level"`
	NextLevelXP *int   `json:"nextLevelXp"`
}

// SkillSetDTO is one skill set of the sheet (ADR 0040 steps 3.5 and 15): set the rogue_skills set code, name its heading (a language map),
// skillArmor the skill armour it reads (ADR 0033 step 14: from the worn body armour's Table 29
// column; none, leather, padded_studded_leather, elven_chain or metal), skills each of its skills
// with its terms and percentage, warnings the entered points' breach of its budget (never null) and
// budget the budget with the terms of its formula (GET /api/skills' set `budget`), null before
// the level its points start.
type SkillSetDTO struct {
	Set        string                     `json:"set"`
	Name       localized.Text             `json:"name"`
	SkillArmor string                     `json:"skillArmor"`
	Skills     []SkillTermsDTO            `json:"skills"`
	Warnings   []BudgetWarningDTO         `json:"warnings"`
	Budget     *wire.SkillPointsBudgetDTO `json:"budget"`
}

// skillSetDTOs maps a character's skill sets; none is an empty list.
func skillSetDTOs(sets []skills.SkillSet) []SkillSetDTO {
	out := make([]SkillSetDTO, 0, len(sets))
	for _, set := range sets {
		dto := SkillSetDTO{
			Set: set.Set, Name: set.Name, SkillArmor: set.Armor, Budget: wire.SkillPointsBudgetToDTO(set.Points),
			Skills: make([]SkillTermsDTO, len(set.Skills)), Warnings: make([]BudgetWarningDTO, len(set.Budget)),
		}
		for i, sk := range set.Skills {
			dto.Skills[i] = SkillTermsDTO(sk)
		}
		for i, w := range set.Budget {
			dto.Warnings[i] = BudgetWarningDTO(w)
		}
		out = append(out, dto)
	}
	return out
}

// SkillTermsDTO is one skill of a skill set (ADR 0030 steps 17 and 18): percent = base + points + race + dexterity + armor, capped at 95 and
// floored at 0.
type SkillTermsDTO struct {
	Skill     string `json:"skill"`
	Base      int    `json:"base"`
	Points    int    `json:"points"`
	Race      int    `json:"race"`
	Dexterity int    `json:"dexterity"`
	Armor     int    `json:"armor"`
	// RawPercent is the sum before the cap and the floor.
	Raw     int `json:"rawPercent"`
	Percent int `json:"percent"`
	// Blocked: the worn body armour blocks the skill (ADR 0042 step 16.3,
	// skill_blocked_by_armour); absent when it does not.
	Blocked bool `json:"blocked,omitempty"`
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

// PSPDTO is a character's PSPs (ADR 0043 step 5): the current PSPs (the
// stored maximum plus the ledger's total) and the maximum.
type PSPDTO struct {
	Current int `json:"current"`
	Max     int `json:"max"`
	// Recovery is CPsiH Table 6 (ADR 0047 item 6): each activity's PSPs an hour and, for a
	// gated one, the proficiency it needs.
	Recovery []PSPRecoveryDTO `json:"recovery"`
}

// PSPRecoveryDTO is one row of Table 6.
type PSPRecoveryDTO struct {
	Activity            string `json:"activity"`
	PerHour             int    `json:"perHour"`
	RequiresProficiency string `json:"requiresProficiency,omitempty"`
}

// KnownPowersDTO is a psionicist's known powers (ADR 0043 step 9).
type KnownPowersDTO struct {
	Allowance PowerCountsDTO `json:"allowance"`
	Used      PowerCountsDTO `json:"used"`
	Primary   string         `json:"primary"`
	Picks     []PowerPickDTO `json:"picks"`
	// UsedThisRound is the id of the power used in the current round (ADR 0043 step 12), omitted
	// when the round is free or out of combat. Only the character read fills it.
	UsedThisRound string `json:"usedThisRound,omitempty"`
}

// PowerCountsDTO counts disciplines, sciences, devotions and defence modes.
type PowerCountsDTO struct {
	Disciplines  int `json:"disciplines"`
	Sciences     int `json:"sciences"`
	Devotions    int `json:"devotions"`
	DefenceModes int `json:"defenceModes"`
}

// PowerPickDTO is one pick: kind discipline, power or relearn, and its id.
type PowerPickDTO struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}

// LevelUpDTO is ADR 0030 step 7: `pending` the levels from the realized one
// to the target, lowest first (the prompt, one per level),
// `realizedAboveTarget` an XP correction brought the target under the
// realized level (step 7.4, the card warns), `toSpend` the slots and skill
// points left at the realized level (ADR 0037 step 15, 0 when none or over:
// the card's reminder).
type LevelUpDTO struct {
	Pending             []wire.PendingLevelDTO `json:"pending"`
	RealizedAboveTarget bool                   `json:"realizedAboveTarget"`
	ToSpend             wire.LevelGrantsDTO    `json:"toSpend"`
}

// savesDTO maps the saves owner's type field by field (ADR 0046 steps 3.4
// and 6.2).
func savesDTO(s saves.Saves) SavesDTO {
	return SavesDTO{ParaPoison: s.ParaPoison, RodStaffWand: s.RodStaffWand, PetriPoly: s.PetriPoly, Breath: s.Breath, Spell: s.Spell}
}

func derivedDTO(d charactersvc.Derived) DerivedDTO {
	out := DerivedDTO{
		XPTotal: d.XPTotal, TargetLevel: d.TargetLevel, RealizedLevel: d.RealizedLevel, NextLevelXP: d.NextLevelXP, THAC0: d.THAC0,
		Saves: savesDTO(d.Saves), RaceSaveBonus: savesDTO(d.RaceSaveBonus),
		SpellSlots: []SlotCountDTO{},
		LevelUp: LevelUpDTO{
			Pending: wire.PendingDTO(d.LevelUp.Pending), RealizedAboveTarget: d.LevelUp.RealizedAboveTarget,
			ToSpend: wire.GrantsDTO(d.LevelUp.ToSpend, d.LevelUp.FreeSpellsOwed),
		},
		Specializes: d.Specializes, ExceptionalStrength: d.ExceptionalStrength, SpellFailure: d.SpellFailure,
		SkillSets: skillSetDTOs(d.SkillSets), Backstab: d.Backstab,
		ProficiencySlots: wire.ProficiencySlotsToDTO(d.Proficiencies),
	}
	for _, e := range d.Classes {
		if len(d.Classes) < 2 {
			break
		}
		out.Classes = append(out.Classes, EntryDerivedDTO{
			Class: e.Class, State: string(e.State), XP: e.XP, TargetLevel: e.TargetLevel,
			Level: e.RealizedLevel, NextLevelXP: e.NextLevelXP,
		})
	}
	if d.Turning != nil {
		out.Turning = make([]TurningTargetDTO, len(d.Turning))
		for i, r := range d.Turning {
			out.Turning[i] = TurningTargetDTO{Undead: r.Undead, HDMin: r.HDMin, HDMax: r.HDMax, Roll: r.Cell.Roll, Outcome: r.Cell.Outcome}
		}
	}
	if p := d.PSP; p != nil {
		out.PSP = &PSPDTO{Current: p.Current, Max: p.Max, Recovery: make([]PSPRecoveryDTO, len(p.Recovery))}
		for i, r := range p.Recovery {
			out.PSP.Recovery[i] = PSPRecoveryDTO{Activity: r.Activity, PerHour: r.PerHour, RequiresProficiency: r.RequiresProficiency}
		}
	}
	if k := d.Powers; k != nil {
		out.KnownPowers = &KnownPowersDTO{
			Allowance: PowerCountsDTO{Disciplines: k.Allowance.Disciplines, Sciences: k.Allowance.Sciences, Devotions: k.Allowance.Devotions, DefenceModes: k.Allowance.DefenceModes},
			Used:      PowerCountsDTO{Disciplines: k.Used.Disciplines, Sciences: k.Used.Sciences, Devotions: k.Used.Devotions, DefenceModes: k.Used.DefenceModes},
			Primary:   k.Primary, Picks: make([]PowerPickDTO, len(k.Picks)),
		}
		for i, pk := range k.Picks {
			out.KnownPowers.Picks[i] = PowerPickDTO{Kind: pk.Kind, ID: pk.ID}
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
		HPMax: c.HPMax, XPTotal: c.XPTotal, Classes: classEntriesDTO(c.Entries()), RaceCode: c.Race,
		Derived: derivedDTO(c.Derived), DetailsDTO: wire.DetailsToDTO(c.Details),
		Missing: c.Details.Missing(), Warnings: warningsDTO(c.Warnings),
	}
}

func warningsDTO(ws []charactersvc.Warning) []WarningDTO { return wire.WarningsDTO(ws) }
