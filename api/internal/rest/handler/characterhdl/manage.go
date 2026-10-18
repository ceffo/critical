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
	// ExceptionalStrength is whether the class enters an 18/xx at Strength
	// 18 (ADR 0030 step 9).
	ExceptionalStrength bool `json:"exceptionalStrength"`
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
	MaxNameLen     int      `json:"maxNameLength"`
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
			MaxNameLen:     o.MaxNameLen,
		}
		for _, c := range o.Classes {
			out.Classes = append(out.Classes, ClassOptionDTO{Class: c.Class, Races: c.Races, ExceptionalStrength: c.ExceptionalStrength})
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
}

// required is the field's value, or the field error of a missing one.
func required(field string, v *int) (int, error) {
	if v == nil {
		return 0, &domain.FieldError{Field: field, Message: charactersvc.CodeEmpty}
	}
	return *v, nil
}

func (b CreateBody) toNew() (charactersvc.NewCharacter, error) {
	n := charactersvc.NewCharacter{Name: b.Name, Class: b.Class, Race: b.Race, StrExceptional: b.StrExceptional}
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

// ChangeBody is the body of PATCH /api/characters/{id}: a new name, or
// retired true (ADR 0029 step 7.3; a retired character never comes back
// here).
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
