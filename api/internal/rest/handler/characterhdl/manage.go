package characterhdl

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/localized"
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

type previewer interface {
	Preview(ctx context.Context, n charactersvc.NewCharacter) (*charactersvc.Character, error)
}

type optionsReader interface {
	CreationOptions(ctx context.Context) (charactersvc.CreationOptions, error)
}

type draftChecker interface {
	CreationChecks(ctx context.Context, in charactersvc.DraftInput) (charactersvc.DraftChecks, error)
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
	// ExceptionalStrengthRaces are the races that enter an 18/xx at
	// Strength 18 with the class (ADR 0030 step 9).
	ExceptionalStrengthRaces []string `json:"exceptionalStrengthRaces"`
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
			Classes:           make([]ClassOptionDTO, 0, len(o.Classes)),
			Ability:           RangeDTO{Min: o.MinAbility, Max: o.MaxAbility},
			HPMax:             RangeDTO{Min: o.MinHPMax, Max: o.MaxHPMax},
			XP:                RangeDTO{Min: o.MinXP, Max: o.MaxXP},
			StrExceptional:    RangeDTO{Min: o.MinStrExceptional, Max: o.MaxStrExceptional},
			SkillPoints:       RangeDTO{Min: 0, Max: o.MaxSkillPoints},
			MaxNameLen:        o.MaxNameLen,
			Sexes:             o.Sexes,
			Languages:         make([]LanguageDTO, 0, len(o.Languages)),
			RaceLanguages:     make([]RaceLanguagesDTO, 0, len(o.RaceLanguages)),
			Proficiencies:     make([]ProficiencyDTO, 0, len(o.Proficiencies)),
			ProficiencyGroups: map[string][]string{},
			ProficiencyCosts:  map[string]map[string]int{},
			ScoreMethods:      make([]ScoreMethodDTO, 0, len(o.ScoreMethods)),
			RaceAdjustments:   make([]RaceAdjustmentsDTO, 0, len(o.RaceAdjustments)),
			RaceIdentity:      make([]RaceIdentityDTO, 0, len(o.RaceIdentity)),
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
				Class: c.Class, Races: c.Races, ExceptionalStrengthRaces: c.ExceptionalStrengthRaces,
				Skills: c.Skills, Alignments: c.Alignments,
				HitDie: c.HitDie, LevelXP: levelXPDTO(c.LevelXP), Minimums: minimumDTOs(c.Minimums),
				WeaponTypes: weaponTypeDTOs(c.WeaponTypes),
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
	XP             *int `json:"xp"`
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

// required is the field's value, or the field error of a missing one.
func required(field string, v *int) (int, error) {
	if v == nil {
		return 0, &domain.FieldError{Field: field, Message: charactersvc.CodeEmpty}
	}
	return *v, nil
}

func (b CreateBody) toNew() (charactersvc.NewCharacter, error) {
	details, err := detailsPatch(b.Details)
	if err != nil {
		// A height or weight that is no number names its field under details.
		if fe, ok := errors.AsType[*domain.FieldError](err); ok {
			return charactersvc.NewCharacter{}, &domain.FieldError{Field: charactersvc.DetailsField + fe.Field, Message: fe.Message}
		}
		return charactersvc.NewCharacter{}, err
	}
	n := charactersvc.NewCharacter{
		Name: b.Name, Class: b.Class, Race: b.Race, StrExceptional: b.StrExceptional,
		SkillPoints: b.SkillPoints, Proficiencies: b.Proficiencies.Picks(), Details: details,
	}
	// The creation body still carries the languages under details (ADR 0037 step 18 moves
	// their editor, not this body).
	if b.Details != nil {
		n.Languages = b.Details.Languages
	}
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
		v, err := required(f.field, f.v)
		if err != nil {
			return n, err
		}
		*f.dst = v
	}
	return n, nil
}

// decodeCreate reads a POST /api/characters body (the creation's and the
// preview's).
func decodeCreate(r *http.Request) (charactersvc.NewCharacter, error) {
	var b CreateBody
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&b); err != nil {
		// A height or weight that is no number names its field under details.
		if fe, ok := errors.AsType[*domain.FieldError](err); ok {
			return charactersvc.NewCharacter{}, &domain.FieldError{Field: charactersvc.DetailsField + fe.Field, Message: fe.Message}
		}
		return charactersvc.NewCharacter{}, render.BadRequestError("invalid JSON body")
	}
	return b.toNew()
}

// PreviewDTO is POST /api/characters/preview: the derived block and the
// warnings the character would be read with once created (as GET
// /api/character), its languages (the native one added), and `missing`, the
// fields still blank that the creation requires (`name`, `alignment`, `sex`).
type PreviewDTO struct {
	Derived   DerivedDTO   `json:"derived"`
	Warnings  []WarningDTO `json:"warnings"`
	Languages []string     `json:"languages"`
	Missing   []string     `json:"missing"`
}

// Preview is POST /api/characters/preview: the creation's body, checked and
// derived as POST /api/characters would, storing nothing (ADR 0034, the
// identity step and the review). A blank name, alignment or sex is listed in
// `missing`; any other invalid input is the creation's 400, field for field.
func Preview(svc previewer) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		if _, err := user(r); err != nil {
			return err
		}
		n, err := decodeCreate(r)
		if err != nil {
			return err
		}
		c, err := svc.Preview(r.Context(), n)
		if err != nil {
			return err
		}
		missing := []string{}
		if c.Name == "" {
			missing = append(missing, "name")
		}
		missing = append(missing, c.Details.Missing()...)
		langs := c.Details.Languages
		if langs == nil {
			langs = []string{}
		}
		render.OK(r.Context(), w, PreviewDTO{
			Derived: derivedDTO(c.Derived), Warnings: warningsDTO(c.Warnings), Languages: langs, Missing: missing,
		})
		return nil
	}
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

// ChecksBody is the body of POST /api/characters/checks: a draft as far as
// it is entered. `scores` holds the abilities entered so far (by code),
// `xp` the starting XP when it reads, `skillPoints` the points by skill.
type ChecksBody struct {
	Class       string         `json:"class"`
	Scores      map[string]int `json:"scores"`
	XP          *int           `json:"xp"`
	SkillPoints map[string]int `json:"skillPoints"`
	// Proficiencies are the non-weapon and the weapon picks so far; absent
	// for none.
	Proficiencies *wire.PicksDTO `json:"proficiencies"`
	// Race and Languages are the draft's race and languages so far (ADR 0037
	// step 6: each beyond the native one and Common spends a non-weapon
	// slot); absent before the identity step. Additive.
	Race      string   `json:"race"`
	Languages []string `json:"languages"`
}

// ClassUnmetDTO is a class and the abilities whose entered score is under its
// minimum.
type ClassUnmetDTO struct {
	Class     string   `json:"class"`
	Abilities []string `json:"abilities"`
}

// SkillBudgetFigureDTO is a thief's or a bard's skill point budget at the
// starting level, with the terms of its formula, named as GET /api/skills'
// `budget` names them: `available` the level allows (`first` + `perLevel` x
// (level - 1)), `spent` entered, `left` (negative when over), `perSkillCap` the
// most one skill holds (`perSkillFirst` + `perSkillPerLevel` x (level - 1); 0:
// no such limit).
type SkillBudgetFigureDTO struct {
	Level            int `json:"level"`
	First            int `json:"first"`
	PerLevel         int `json:"perLevel"`
	Available        int `json:"available"`
	Spent            int `json:"spent"`
	Left             int `json:"left"`
	PerSkillFirst    int `json:"perSkillFirst"`
	PerSkillPerLevel int `json:"perSkillPerLevel"`
	PerSkillCap      int `json:"perSkillCap"`
}

// ChecksDTO is POST /api/characters/checks: `unmetMinimums` for every
// creatable class, and `skillBudget` for the draft's class (null for a class
// without skill points or an XP that gives no level).
type ChecksDTO struct {
	UnmetMinimums []ClassUnmetDTO       `json:"unmetMinimums"`
	SkillBudget   *SkillBudgetFigureDTO `json:"skillBudget"`
	// ProficiencySlots is the draft's pools and picks (ADR 0037 steps 3 to 5,
	// 7 and 14), as GET /api/character's derived block names
	// them; null while the class, the XP or the Intelligence is missing.
	ProficiencySlots *wire.ProficiencySlotsDTO `json:"proficiencySlots"`
}

// Checks is POST /api/characters/checks: the verdicts the class and skills
// steps show on a draft too raw to preview (ADR 0034). It refuses no value
// and stores nothing.
func Checks(svc draftChecker) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		var b ChecksBody
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		if err := dec.Decode(&b); err != nil {
			return render.BadRequestError("invalid JSON body")
		}
		c, err := svc.CreationChecks(r.Context(), charactersvc.DraftInput{
			Class: b.Class, Scores: b.Scores, XP: b.XP, SkillPoints: b.SkillPoints, Proficiencies: b.Proficiencies.Picks(),
			Race: b.Race, Languages: b.Languages,
		})
		if err != nil {
			return err
		}
		out := ChecksDTO{UnmetMinimums: make([]ClassUnmetDTO, 0, len(c.UnmetMinimums)), ProficiencySlots: wire.ProficiencySlotsToDTO(c.ProficiencySlots)}
		for _, u := range c.UnmetMinimums {
			out.UnmetMinimums = append(out.UnmetMinimums, ClassUnmetDTO(u))
		}
		if c.SkillBudget != nil {
			b := c.SkillBudget
			out.SkillBudget = &SkillBudgetFigureDTO{
				Level: b.Level, First: b.First, PerLevel: b.PerLevel, Available: b.Total, Spent: b.Spent, Left: b.Left,
				PerSkillFirst: b.PerSkillFirst, PerSkillPerLevel: b.PerSkillPerLevel, PerSkillCap: b.PerSkill,
			}
		}
		render.OK(r.Context(), w, out)
		return nil
	}
}

// weaponTypeDTOs maps a class's weapon type options; never nil.
func weaponTypeDTOs(ts []charactersvc.WeaponTypeOption) []WeaponTypeOptionDTO {
	out := make([]WeaponTypeOptionDTO, 0, len(ts))
	for _, t := range ts {
		out = append(out, WeaponTypeOptionDTO(t))
	}
	return out
}
