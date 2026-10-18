package characterhdl

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/render"
	"github.com/ceffo/critical/api/internal/rest/handler/detailshdl"
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

// ClassOptionDTO is a creatable class and the race codes it admits.
type ClassOptionDTO struct {
	Class string   `json:"class"`
	Races []string `json:"races"`
	// ExceptionalStrengthRaces are the races that enter an 18/xx at
	// Strength 18 with the class (ADR 0030 step 9).
	ExceptionalStrengthRaces []string `json:"exceptionalStrengthRaces"`
	// Skills are the thieving skills the class enters points in and
	// SkillArmors the armours it picks for them (ADR 0030 steps 1 and 17);
	// empty for a class without them.
	Skills      []string `json:"skills"`
	SkillArmors []string `json:"skillArmors"`
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
		}
		for _, c := range o.Classes {
			out.Classes = append(out.Classes, ClassOptionDTO{
				Class: c.Class, Races: c.Races, ExceptionalStrengthRaces: c.ExceptionalStrengthRaces,
				Skills: c.Skills, SkillArmors: c.SkillArmors,
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
	// SkillPoints and SkillArmor are a thief's (ADR 0030 steps 1 and 17):
	// points by skill code, the armour worn for the skills; absent for the
	// other classes.
	SkillPoints map[string]int `json:"skillPoints"`
	SkillArmor  string         `json:"skillArmor"`
	// Details are the free-text details, in the shape GET /api/character
	// shows them (age null for none); absent for none. A refusal names its
	// field under "details." (details.age).
	Details *detailshdl.DetailsDTO `json:"details"`
}

// detailsPatch is the details as the patch a new character applies: every text
// field set (empty is none), the age and the languages when given.
func detailsPatch(d *detailshdl.DetailsDTO) charactersvc.DetailsPatch {
	if d == nil {
		return charactersvc.DetailsPatch{}
	}
	p := charactersvc.DetailsPatch{
		Race: &d.Race, Alignment: &d.Alignment, Age: d.Age, Sex: &d.Sex, Height: &d.Height,
		Weight: &d.Weight, Hair: &d.Hair, Eyes: &d.Eyes,
	}
	if d.Languages != nil {
		p.Languages = &d.Languages
	}
	return p
}

// required is the field's value, or the field error of a missing one.
func required(field string, v *int) (int, error) {
	if v == nil {
		return 0, &domain.FieldError{Field: field, Message: charactersvc.CodeEmpty}
	}
	return *v, nil
}

func (b CreateBody) toNew() (charactersvc.NewCharacter, error) {
	n := charactersvc.NewCharacter{
		Name: b.Name, Class: b.Class, Race: b.Race, StrExceptional: b.StrExceptional,
		SkillPoints: b.SkillPoints, SkillArmor: b.SkillArmor, Details: detailsPatch(b.Details),
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

// Create is POST /api/characters: a new character of the user's, which
// becomes their current one (ADR 0029 step 7.2). It answers 201 with the
// list, as GET /api/characters. Invalid input is 400 with the field.
func Create(svc creator) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		u, err := user(r)
		if err != nil {
			return err
		}
		var b CreateBody
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		if err := dec.Decode(&b); err != nil {
			return render.BadRequestError("invalid JSON body")
		}
		n, err := b.toNew()
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

// ChangeBody is the body of PATCH /api/characters/{id}: a new name, a thief's new
// skill armour, or retired true (ADR 0029 step 7.3; a retired character never comes back
// here).
type ChangeBody struct {
	Name    *string `json:"name"`
	Retired *bool   `json:"retired"`
	// SkillArmor is a thief's new skill armour (ADR 0030 step 1).
	SkillArmor *string `json:"skillArmor"`
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
		case b.Name == nil && b.Retired == nil && b.SkillArmor == nil:
			return render.BadRequestError("name, retired or skillArmor is required")
		case b.Retired != nil && !*b.Retired:
			return render.BadRequestError("a character is only ever retired")
		}
		current, err := svc.Change(r.Context(), u.ID, id, charactersvc.Change{Name: b.Name, Retired: b.Retired, SkillArmor: b.SkillArmor})
		if err != nil {
			return err
		}
		return renderCharacters(w, r, svc, u.ID, current)
	}
}
