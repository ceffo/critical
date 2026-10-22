package characterhdl

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/psionics"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
	"github.com/ceffo/critical/api/internal/render"
	"github.com/ceffo/critical/api/internal/rest/wire"
)

type creator interface {
	ownedLister
	Create(ctx context.Context, owner uuid.UUID, n charactersvc.NewCharacter) (int64, error)
}

type changer interface {
	ownedLister
	Change(ctx context.Context, owner uuid.UUID, id int64, c charactersvc.Change) (int64, error)
}

type optionsReader interface {
	CreationOptions(ctx context.Context) (charactersvc.CreationOptions, error)
}

func minimumDTOs(ms []ruletablesvc.AbilityMinimum) []AbilityMinimumDTO {
	out := make([]AbilityMinimumDTO, 0, len(ms))
	for _, m := range ms {
		out = append(out, AbilityMinimumDTO{Ability: m.Ability, Min: m.Min})
	}
	return out
}

// ClassOptionDTO is a creatable class and the race codes it admits.
type ClassOptionDTO struct {
	Class string   `json:"class"`
	Races []string `json:"races"`
	// Skills are the thieving skills the class enters points in (ADR 0030
	// steps 1 and 17); empty for a class without them.
	Skills []string `json:"skills"`
	// Alignments are the alignment codes the class admits (ADR 0032 step 2),
	// so the form offers only those.
	Alignments []string `json:"alignments"`
	// HitDie is the sides of the class's hit die (ADR 0030 step 7).
	HitDie int `json:"hitDie"`
	// LevelXP is ADR 0034 step 4.1: levels 1 to 20, each with the lowest
	// XP of that level for the class; a picked level is sent as its XP.
	LevelXP []LevelXPDTO `json:"levelXp"`
	// Minimums are the class's ability minimums (PHB Table 13), shown on
	// its card and not enforced; empty for none. Additive.
	Minimums []AbilityMinimumDTO `json:"minimums"`
	// WeaponTypes are the weapon types the class may pick (ADR 0037 step 7)
	// in display order, each with its specialization's slots in all (step 9),
	// 0 for a class that does not specialize. Additive; never null.
	WeaponTypes []WeaponTypeOptionDTO `json:"weaponTypes"`
	// Spellbook is the class's starting spellbook (ADR 0030 step 20.7):
	// `startingSpells`, the spells it is offered pre-ticked (removable);
	// null for a class without a spellbook.
	Spellbook *SpellbookOptionDTO `json:"spellbook"`
}

// SpellbookOptionDTO is a class's starting spellbook option.
type SpellbookOptionDTO struct {
	StartingSpells []string `json:"startingSpells"`
}

// SpellOptionDTO is a catalogue wizard spell the starting spellbook offers.
type SpellOptionDTO struct {
	ID    string `json:"id"`
	Level int    `json:"level"`
}

// WeaponTypeOptionDTO is one weapon type a class may pick and the slots its
// specialization spends (0: the class does not specialize).
type WeaponTypeOptionDTO struct {
	Type                string `json:"type"`
	SpecializationSlots int    `json:"specializationSlots"`
}

// AbilityMinimumDTO is the lowest score a class asks in one ability.
type AbilityMinimumDTO struct {
	Ability string `json:"ability"`
	Min     int    `json:"min"`
}

// RangeDTO is the inclusive range an entered number must fall in.
type RangeDTO struct {
	Min int `json:"min"`
	Max int `json:"max"`
}

// OptionsDTO is GET /api/characters/options: what the creation form offers
// and the ranges the server checks.
type OptionsDTO struct {
	Classes []ClassOptionDTO `json:"classes"`
	Ability RangeDTO         `json:"ability"`
	HPMax   RangeDTO         `json:"hpMax"`
	XP      RangeDTO         `json:"xp"`
	// StrExceptional is the range of an exceptional strength (1 = 18/01,
	// 100 = 18/00).
	StrExceptional RangeDTO `json:"strExceptional"`
	// SkillPoints is the range of a thief's points in one skill.
	SkillPoints RangeDTO `json:"skillPoints"`
	MaxNameLen  int      `json:"maxNameLength"`
	// MaxClasses is the most active classes of a multi-class character and MinLeftAtLevel the lowest
	// level a class can be left at (ADR 0042 step 1). The racial lift (ADR 0041 step 2.2) lets any
	// race be multi-class (`multiclassAnyRace`) or dual-class (`dualclassAnyRace`); with it off the
	// multi-class races are the `restrictedRaces`. A dual-class character needs `dualClassOldPrime` in
	// every prime requisite of a class it left and `dualClassNewPrime` in its new class's. Additive.
	MaxClasses        int      `json:"maxClasses"`
	MinLeftAtLevel    int      `json:"minLeftAtLevel"`
	MulticlassAnyRace bool     `json:"multiclassAnyRace"`
	DualclassAnyRace  bool     `json:"dualclassAnyRace"`
	RestrictedRaces   []string `json:"restrictedRaces"`
	DualClassOldPrime int      `json:"dualClassOldPrime"`
	DualClassNewPrime int      `json:"dualClassNewPrime"`
	// Sexes are the sex codes (ADR 0032 step 3).
	Sexes []string `json:"sexes"`
	// Languages is the language catalogue with its names by language (ADR
	// 0032 step 4.3: the form offers all of it), RaceLanguages each race's
	// own language and PHB list, which the form marks. Never null; both are
	// empty before catalogue format 8.
	Languages     []LanguageDTO      `json:"languages"`
	RaceLanguages []RaceLanguagesDTO `json:"raceLanguages"`
	// Proficiencies is the non-weapon catalogue (PHB Table 37), ProficiencyGroups
	// the groups each class learns at the table's cost (Table 38) (ADR 0037
	// steps 1 and 2). A name is the web's i18n key by id. Additive; never null.
	Proficiencies     []ProficiencyDTO    `json:"proficiencies"`
	ProficiencyGroups map[string][]string `json:"proficiencyGroups"`
	// ProficiencyCosts is each proficiency's cost by class and id (ADR 0037
	// step 4, the crossover's +1 included): the editor shows and sends it. A
	// proficiency with no cost is not picked on its own (step 6): the editor
	// leaves it out. Additive; never null.
	ProficiencyCosts map[string]map[string]int `json:"proficiencyCosts"`
	// ScoreMethods are ADR 0032 step 6's methods in the book's order (the
	// score step's chips); RaceAdjustments PHB Table 8 by race (the race
	// cards). Additions of ADR 0034: a previous web ignores them.
	ScoreMethods    []ScoreMethodDTO     `json:"scoreMethods"`
	RaceAdjustments []RaceAdjustmentsDTO `json:"raceAdjustments"`
	// RaceIdentity is ADR 0032 step 5's dice and ranges by race: the
	// identity step's roll buttons and the race cards show them. Additive.
	RaceIdentity []RaceIdentityDTO `json:"raceIdentity"`
	// WizardSpells are the catalogue's wizard spells by spell level then
	// id, the starting spellbook's choices (ADR 0030 step 20.7); the names
	// are GET /api/spells/names'.
	WizardSpells []SpellOptionDTO `json:"wizardSpells"`
}

// FigureDTO is one identity figure: `count` dice of `sides` rolled, and the
// range (`min`, `max`, stored units: cm, kg or years) outside which a value
// warns. `sex` names a height or weight row.
type FigureDTO struct {
	Sex   string  `json:"sex,omitempty"`
	Count int     `json:"count"`
	Sides int     `json:"sides"`
	Min   float64 `json:"min"`
	Max   float64 `json:"max"`
}

// RaceIdentityDTO is a race's height and weight per sex, its age, and the
// range of starting ages a roll gives (`startAge`, inside `age`).
type RaceIdentityDTO struct {
	Race     string      `json:"race"`
	Height   []FigureDTO `json:"height"`
	Weight   []FigureDTO `json:"weight"`
	Age      FigureDTO   `json:"age"`
	StartAge RangeFDTO   `json:"startAge"`
}

// RangeFDTO is an inclusive range of decimal numbers.
type RangeFDTO struct {
	Min float64 `json:"min"`
	Max float64 `json:"max"`
}

func figureDTOs(fs []charactersvc.FigureOption) []FigureDTO {
	out := make([]FigureDTO, 0, len(fs))
	for _, f := range fs {
		out = append(out, figureDTO(f))
	}
	return out
}

func figureDTO(f charactersvc.FigureOption) FigureDTO {
	return FigureDTO{Sex: f.Sex, Count: f.Count, Sides: f.Sides, Min: f.Range.Min, Max: f.Range.Max}
}

// LevelXPDTO is a level and the XP that reaches it.
type LevelXPDTO struct {
	Level int `json:"level"`
	XP    int `json:"xp"`
}

// ScoreMethodDTO is one of ADR 0032 step 6's methods, as the roll answers
// it (creationrollhdl.MethodDTO): `assign` is how the totals become scores.
type ScoreMethodDTO struct {
	Code       string `json:"code"`
	Rolls      int    `json:"rolls"`
	Dice       int    `json:"dice"`
	DropLowest bool   `json:"dropLowest"`
	Assign     string `json:"assign"`
	Keep       int    `json:"keep"`
	Base       int    `json:"base"`
	Max        int    `json:"max"`
}

// RaceAdjustmentsDTO is PHB Table 8 for a race: each ability's adjustment.
type RaceAdjustmentsDTO struct {
	Race        string         `json:"race"`
	Adjustments map[string]int `json:"adjustments"`
}

// LanguageDTO is a catalogue language: its code and its name by language.
type LanguageDTO struct {
	Code string         `json:"code"`
	Name localized.Text `json:"name"`
}

// ProficiencyDTO is a catalogue proficiency: the slots it costs at the table's
// price, the ability and modifier of its check (absent without one) and every
// group it is listed under.
type ProficiencyDTO struct {
	ID       string   `json:"id"`
	Ability  string   `json:"ability,omitempty"`
	Modifier int      `json:"modifier"`
	Slots    int      `json:"slots"`
	Groups   []string `json:"groups"`
}

// RaceLanguagesDTO is a race's own language and the languages of its PHB list.
type RaceLanguagesDTO struct {
	Race      string   `json:"race"`
	Native    string   `json:"native"`
	Languages []string `json:"languages"`
}

// Options is GET /api/characters/options.
func Options(svc optionsReader) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		o, err := svc.CreationOptions(r.Context())
		if err != nil {
			return err
		}
		out := OptionsDTO{
			Classes:        make([]ClassOptionDTO, 0, len(o.Classes)),
			Ability:        RangeDTO{Min: o.MinAbility, Max: o.MaxAbility},
			HPMax:          RangeDTO{Min: o.MinHPMax, Max: o.MaxHPMax},
			XP:             RangeDTO{Min: o.MinXP, Max: o.MaxXP},
			StrExceptional: RangeDTO{Min: o.MinStrExceptional, Max: o.MaxStrExceptional},
			SkillPoints:    RangeDTO{Min: 0, Max: o.MaxSkillPoints},
			MaxNameLen:     o.MaxNameLen,
			MaxClasses:     o.MaxClasses, MinLeftAtLevel: o.MinLeftAtLevel, MulticlassAnyRace: o.MulticlassAnyRace,
			DualclassAnyRace: o.DualclassAnyRace, RestrictedRaces: o.RestrictedRaces,
			DualClassOldPrime: o.DualClassOldPrime, DualClassNewPrime: o.DualClassNewPrime,
			Sexes:             o.Sexes,
			Languages:         make([]LanguageDTO, 0, len(o.Languages)),
			RaceLanguages:     make([]RaceLanguagesDTO, 0, len(o.RaceLanguages)),
			Proficiencies:     make([]ProficiencyDTO, 0, len(o.Proficiencies)),
			ProficiencyGroups: map[string][]string{},
			ProficiencyCosts:  map[string]map[string]int{},
			ScoreMethods:      make([]ScoreMethodDTO, 0, len(o.ScoreMethods)),
			RaceAdjustments:   make([]RaceAdjustmentsDTO, 0, len(o.RaceAdjustments)),
			RaceIdentity:      make([]RaceIdentityDTO, 0, len(o.RaceIdentity)),
			WizardSpells:      make([]SpellOptionDTO, 0, len(o.WizardSpells)),
		}
		for _, sp := range o.WizardSpells {
			out.WizardSpells = append(out.WizardSpells, SpellOptionDTO(sp))
		}
		for _, ri := range o.RaceIdentity {
			out.RaceIdentity = append(out.RaceIdentity, RaceIdentityDTO{
				Race: ri.Race, Height: figureDTOs(ri.Height), Weight: figureDTOs(ri.Weight), Age: figureDTO(ri.Age),
				StartAge: RangeFDTO{Min: ri.StartAge.Min, Max: ri.StartAge.Max},
			})
		}
		for _, m := range o.ScoreMethods {
			out.ScoreMethods = append(out.ScoreMethods, ScoreMethodDTO{
				Code: m.Code, Rolls: m.Rolls, Dice: m.Dice, DropLowest: m.DropLowest, Assign: m.Assign, Keep: m.Keep, Base: m.Base, Max: m.Max,
			})
		}
		for _, ra := range o.RaceAdjustments {
			out.RaceAdjustments = append(out.RaceAdjustments, RaceAdjustmentsDTO{Race: ra.Race, Adjustments: ra.Adjustments})
		}
		for _, l := range o.Languages {
			out.Languages = append(out.Languages, LanguageDTO{Code: l.Code, Name: l.Name})
		}
		for _, pr := range o.Proficiencies {
			out.Proficiencies = append(out.Proficiencies, ProficiencyDTO(pr))
		}
		for class, groups := range o.ProficiencyGroups {
			out.ProficiencyGroups[class] = groups
		}
		for class, costs := range o.ProficiencyCosts {
			out.ProficiencyCosts[class] = costs
		}
		for _, rl := range o.RaceLanguages {
			out.RaceLanguages = append(out.RaceLanguages, RaceLanguagesDTO{Race: rl.Race, Native: rl.Native, Languages: rl.List})
		}
		for _, c := range o.Classes {
			out.Classes = append(out.Classes, ClassOptionDTO{
				Class: c.Class, Races: c.Races,
				Skills: c.Skills, Alignments: c.Alignments,
				HitDie: c.HitDie, LevelXP: levelXPDTO(c.LevelXP), Minimums: minimumDTOs(c.Minimums),
				WeaponTypes: weaponTypeDTOs(c.WeaponTypes), Spellbook: spellbookOptionDTO(c.Spellbook),
			})
		}
		render.OK(r.Context(), w, out)
		return nil
	}
}

// CreateBody is the body of POST /api/characters: the entered values only
// (ADR 0030 step 1). An unknown field (a level, a THAC0) is refused.
type CreateBody struct {
	Name  string `json:"name"`
	Class string `json:"class"`
	Race  string `json:"raceCode"`
	Str   *int   `json:"str"`
	// StrExceptional is a fighter's 18/xx percentile (ADR 0030 step 9);
	// absent or 0 for none.
	StrExceptional int  `json:"strExceptional"`
	Dex            *int `json:"dex"`
	Con            *int `json:"con"`
	Int            *int `json:"int"`
	Wis            *int `json:"wis"`
	Cha            *int `json:"cha"`
	HPMax          *int `json:"hpMax"`
	// PSPMax is the PSP maximum entered from the sheet (ADR 0043 step 4.4); absent or 0 for the
	// rules' own.
	PSPMax *int `json:"pspMax"`
	XP     *int `json:"xp"`
	// SkillPoints are a thief's or a bard's (ADR 0030 steps 1, 17 and 18):
	// points by skill code; absent for the other classes. The skills' armour
	// is the worn one (ADR 0033 step 14), no entry.
	SkillPoints map[string]int `json:"skillPoints"`
	// Proficiencies are the non-weapon and the weapon picks (ADR 0037 step
	// 14), checked as PUT /api/proficiencies checks them with nothing picked
	// before; absent (or a pool absent) for none.
	Proficiencies *wire.PicksDTO `json:"proficiencies"`
	// Details are the free-text details, in the shape GET /api/character
	// shows them (age null for none); absent for none. A refusal names its
	// field under "details." (details.age).
	Details *wire.DetailsDTO `json:"details"`
	// Spellbook is a mage's or a bard's starting spellbook (ADR 0030 step
	// 20.7): the catalogue ids of the spells the DM gave, written without a
	// roll; absent or empty for none. A refused spell names its field
	// "spellbook.<id>"; a spellbook for another class, "spellbook".
	Spellbook []string `json:"spellbook"`
	// Classes are the class entries (ADR 0042 steps 1 and 4), one to three active ones, or active ones
	// and the classes a dual-class character left; they replace `class` and `xp`, which a body naming
	// them too is refused for. A refusal names `classes` or `classes.<i>.<field>`.
	Classes []ClassEntryBody `json:"classes"`
	// PsionicPicks are a psionicist's known powers (ADR 0043 step 9), in PUT /api/psionics/picks'
	// pick shape and order, checked as it checks them at the starting level; absent or empty for
	// none. A list over the rules is that route's 409 with its code and figures; a malformed one, or
	// one for a character with no psionics, a refusal on `psionicPicks`.
	PsionicPicks []PsionicPickBody `json:"psionicPicks"`
}

// PsionicPickBody is one known-powers pick: `kind` discipline, power or relearn, and its id.
type PsionicPickBody struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}

// ClassEntryBody is one class of a new character. `xp` is required for a class advanced in (a draft
// lists `classes.<i>.xp` as missing), optional for a class left; `level` is the level taken, absent
// for the level the XP reaches; `leftAtLevel` is set for a class left (2 to 20) and absent for a
// class advanced in.
type ClassEntryBody struct {
	Class       string `json:"class"`
	XP          *int   `json:"xp"`
	Level       int    `json:"level"`
	LeftAtLevel int    `json:"leftAtLevel"`
}

// detailsPatch is the details as the patch a new character applies: every text
// field set (empty is none) and the age when given; the languages go apart
// (NewCharacter.Languages).
func detailsPatch(d *wire.DetailsDTO) (charactersvc.DetailsPatch, error) {
	if d == nil {
		return charactersvc.DetailsPatch{}, nil
	}
	p := charactersvc.DetailsPatch{
		Alignment: &d.Alignment, Age: d.Age, Sex: &d.Sex, Hair: &d.Hair, Eyes: &d.Eyes,
	}
	var cm, kg *float64
	if d.HeightCm != nil {
		v := float64(*d.HeightCm)
		cm = &v
	}
	if d.WeightKg != nil {
		v := float64(*d.WeightKg)
		kg = &v
	}
	p.Height, p.ClearHeight = wire.ResolveMeasure(cm, d.HeightNull)
	p.Weight, p.ClearWeight = wire.ResolveMeasure(kg, d.WeightNull)
	return p, nil
}

func (b CreateBody) toNew() (charactersvc.NewCharacter, error) {
	n, blank, err := b.entered()
	if err != nil {
		return n, err
	}
	if len(blank) > 0 {
		return n, &domain.FieldError{Field: blank[0], Message: charactersvc.CodeEmpty}
	}
	return n, nil
}

// entered is the body as a new character and the numbers it leaves out (blank, in the form's
// order), which are 0 in it: the creation refuses them, a draft lists them as missing.
func (b CreateBody) entered() (charactersvc.NewCharacter, []string, error) {
	details, err := detailsPatch(b.Details)
	if err != nil {
		// A height or weight that is no number names its field under details.
		if fe, ok := errors.AsType[*domain.FieldError](err); ok {
			return charactersvc.NewCharacter{}, nil, &domain.FieldError{Field: charactersvc.DetailsField + fe.Field, Message: fe.Message}
		}
		return charactersvc.NewCharacter{}, nil, err
	}
	n := charactersvc.NewCharacter{
		Name: b.Name, Class: b.Class, Race: b.Race, StrExceptional: b.StrExceptional,
		SkillPoints: b.SkillPoints, Proficiencies: b.Proficiencies.Picks(), Details: details,
		Spellbook: b.Spellbook,
	}
	// The creation body still carries the languages under details (ADR 0037 step 18 moves
	// their editor, not this body).
	if b.Details != nil {
		n.Languages = b.Details.Languages
	}
	if len(b.Classes) > 0 {
		if b.Class != "" || b.XP != nil {
			return charactersvc.NewCharacter{}, nil, render.BadRequestError("send class and xp or classes, not both")
		}
		n.Class = ""
		for _, c := range b.Classes {
			e := charactersvc.NewClass{Class: c.Class, Level: c.Level, LeftAtLevel: c.LeftAtLevel}
			if c.XP != nil {
				e.XP = *c.XP
			}
			n.Classes = append(n.Classes, e)
		}
	}
	if b.PSPMax != nil {
		n.PSPMax = *b.PSPMax
	}
	for _, p := range b.PsionicPicks {
		n.PsionicPicks = append(n.PsionicPicks, psionics.Pick(p))
	}
	var blank []string
	for _, f := range []struct {
		field string
		dst   *int
		v     *int
	}{
		{"str", &n.Str, b.Str},
		{"dex", &n.Dex, b.Dex},
		{"con", &n.Con, b.Con},
		{"int", &n.Int, b.Int},
		{"wis", &n.Wis, b.Wis},
		{"cha", &n.Cha, b.Cha},
		{"hpMax", &n.HPMax, b.HPMax},
		{"xp", &n.XP, b.XP},
	} {
		if f.v == nil && f.field == "xp" && len(b.Classes) > 0 {
			continue
		}
		if f.v == nil {
			blank = append(blank, f.field)
			continue
		}
		*f.dst = *f.v
	}
	for i, c := range b.Classes {
		if c.XP == nil && c.LeftAtLevel == 0 {
			blank = append(blank, fmt.Sprintf("%s.%d.xp", charactersvc.FieldClasses, i))
		}
	}
	return n, blank, nil
}

// decodeCreate reads a POST /api/characters body.
func decodeCreate(r *http.Request) (charactersvc.NewCharacter, error) {
	b, err := decodeBody(r)
	if err != nil {
		return charactersvc.NewCharacter{}, err
	}
	return b.toNew()
}

// decodeBody reads a creation body (the creation's and the draft's). An unknown field or a body
// that is no JSON is a bad request; a height or weight that is no number is a field error under
// details.
func decodeBody(r *http.Request) (CreateBody, error) {
	var b CreateBody
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&b); err != nil {
		if fe, ok := errors.AsType[*domain.FieldError](err); ok {
			return CreateBody{}, &domain.FieldError{Field: charactersvc.DetailsField + fe.Field, Message: fe.Message}
		}
		return CreateBody{}, render.BadRequestError("invalid JSON body")
	}
	return b, nil
}

// Create is POST /api/characters: a new character of the user's, which
// becomes their current one (ADR 0029 step 7.2). It answers 201 with the
// list, as GET /api/characters. Invalid input is 400 with the field.
func Create(svc creator) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		u, err := user(r)
		if err != nil {
			return err
		}
		n, err := decodeCreate(r)
		if err != nil {
			return err
		}
		id, err := svc.Create(r.Context(), u.ID, n)
		if err != nil {
			return err
		}
		return renderCharactersStatus(w, r, svc, u.ID, id, http.StatusCreated)
	}
}

// ChangeBody is the body of PATCH /api/characters/{id}: a new name, or
// retired true (ADR 0029 step 7.3; a retired character never comes back
// here). The skill armour is no entry (ADR 0033 step 14): a body naming it is
// an unknown field, refused.
type ChangeBody struct {
	Name    *string `json:"name"`
	Retired *bool   `json:"retired"`
}

// Change is PATCH /api/characters/{id}: renames or retires one of the user's
// characters. Another user's, a retired one and an unknown id are 404
// (step 7.1). It answers with the list and the current character after the
// change (retiring the current one makes the first one left current).
func Change(svc changer) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		u, err := user(r)
		if err != nil {
			return err
		}
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil || id < 1 {
			return render.BadRequestError("id is not a character id")
		}
		var b ChangeBody
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		if err := dec.Decode(&b); err != nil {
			return render.BadRequestError("invalid JSON body")
		}
		switch {
		case b.Name == nil && b.Retired == nil:
			return render.BadRequestError("name or retired is required")
		case b.Retired != nil && !*b.Retired:
			return render.BadRequestError("a character is only ever retired")
		}
		current, err := svc.Change(r.Context(), u.ID, id, charactersvc.Change{Name: b.Name, Retired: b.Retired})
		if err != nil {
			return err
		}
		return renderCharacters(w, r, svc, u.ID, current)
	}
}

func levelXPDTO(rows []charactersvc.LevelXP) []LevelXPDTO {
	out := make([]LevelXPDTO, 0, len(rows))
	for _, r := range rows {
		out = append(out, LevelXPDTO{Level: r.Level, XP: r.XP})
	}
	return out
}

// ClassUnmetDTO is a class and the abilities whose entered score is under its
// minimum.
type ClassUnmetDTO struct {
	Class     string   `json:"class"`
	Abilities []string `json:"abilities"`
}

// spellbookOptionDTO maps a class's starting spellbook option; nil for none.
func spellbookOptionDTO(o *charactersvc.SpellbookOption) *SpellbookOptionDTO {
	if o == nil {
		return nil
	}
	return &SpellbookOptionDTO{StartingSpells: append([]string{}, o.StartingSpells...)}
}

// weaponTypeDTOs maps a class's weapon type options; never nil.
func weaponTypeDTOs(ts []charactersvc.WeaponTypeOption) []WeaponTypeOptionDTO {
	out := make([]WeaponTypeOptionDTO, 0, len(ts))
	for _, t := range ts {
		out = append(out, WeaponTypeOptionDTO(t))
	}
	return out
}
