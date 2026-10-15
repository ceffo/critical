// Package spellhdl exposes /api/spells.
package spellhdl

import (
	"context"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/spelleffectsvc"
	"github.com/ceffo/critical/api/internal/domain/service/spellsvc"
	"github.com/ceffo/critical/api/internal/domain/spellstats"
	"github.com/ceffo/critical/api/internal/render"
	"github.com/ceffo/critical/api/pkg/formula"
)

// Router is the chi subset we need.
type Router interface {
	Get(pattern string, h http.HandlerFunc)
	Post(pattern string, h http.HandlerFunc)
}

// CharacterIDFunc resolves the active character.
type CharacterIDFunc func(r *http.Request) int64

type spellSvc interface {
	GetSlots(ctx context.Context, characterID int64) ([]spellsvc.SpellSlot, error)
	ResetSlots(ctx context.Context, characterID int64) ([]spellsvc.SpellSlot, error)
	Search(ctx context.Context, f spellsvc.Filter, p spellsvc.PageRequest) (spellsvc.Page, error)
}

type characterGetter interface {
	GetCharacter(ctx context.Context, id int64) (*charactersvc.Character, error)
}

// RegisterRoutes wires spell endpoints onto r.
func RegisterRoutes(r Router, charID CharacterIDFunc, svc spellsvc.SVC, chars characterGetter) {
	r.Get("/api/spells/slots", render.Handle(GetSlots(charID, svc)))
	r.Get("/api/spells", render.Handle(Search(charID, svc, chars)))
	r.Get("/api/spells/filters", render.Handle(FilterOptions()))
}

// FilterOptionsDTO lists the choices of the lookup's school / sphere
// dropdown: the school group keys (the client labels them) and the Priest
// sphere names.
type FilterOptionsDTO struct {
	Schools []string `json:"schools"`
	Spheres []string `json:"spheres"`
}

// FilterOptions returns the school groups and spheres GET /api/spells takes.
func FilterOptions() render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		render.OK(r.Context(), w, FilterOptionsDTO{Schools: spellsvc.SchoolGroups(), Spheres: spellsvc.Spheres()})
		return nil
	}
}

// GetSlots returns the slot list.
func GetSlots(charID CharacterIDFunc, svc spellSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		slots, err := svc.GetSlots(r.Context(), charID(r))
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, SlotsToDTO(slots))
		return nil
	}
}

// Search runs the catalog query. The optional caster_level query parameter
// sets the level the computed values are for; it defaults to the character's.
func Search(charID CharacterIDFunc, svc spellSvc, chars characterGetter) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		f, err := parseFilter(r.URL.Query())
		if err != nil {
			return err
		}
		pr, err := parsePage(r.URL.Query())
		if err != nil {
			return err
		}
		page, err := svc.Search(r.Context(), f, pr)
		if err != nil {
			return err
		}
		level, err := casterLevel(r, charID, chars)
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, SpellPageDTO{
			Items: spellsToDTO(page.Items, level), Total: page.Total, Page: page.Page, PageSize: page.PageSize,
		})
		return nil
	}
}

// parsePage reads page (1-based, default 1) and page_size (default 50, max
// 200). A malformed or out-of-range value is a 400.
func parsePage(q url.Values) (spellsvc.PageRequest, error) {
	pr := spellsvc.PageRequest{Page: 1, PageSize: spellsvc.DefaultPageSize}
	if raw := q.Get("page"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			return pr, render.BadRequestError("page must be a positive integer")
		}
		pr.Page = n
	}
	if raw := q.Get("page_size"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > spellsvc.MaxPageSize {
			return pr, render.BadRequestError("page_size must be an integer from 1 to " + strconv.Itoa(spellsvc.MaxPageSize))
		}
		pr.PageSize = n
	}
	return pr, nil
}

// parseFilter reads the filter parameters. A malformed value is a 400,
// never silently ignored.
//
//   - q: text matched against the English name (exact, prefix, substring).
//   - match_exact, match_prefix, match_contains: comma-separated catalogue
//     keys the client matched by another name (French), with that tier; a
//     spell matching q or any of them is kept, ranked by its best tier.
//   - class: mine (the Bard's spellbook), wizard, priest, or all/both/empty.
//   - level: comma-separated levels, e.g. "1,3"; empty means any.
//   - school: a school group key (GET /api/spells/filters); sphere: a sphere.
//   - components, save, casting_time: typed-stat filters.
func parseFilter(q url.Values) (spellsvc.Filter, error) {
	f := spellsvc.Filter{Query: strings.TrimSpace(q.Get("q"))}
	for _, tier := range matchTiers {
		for _, key := range splitCSV(q.Get(tier.param)) {
			f.Names = append(f.Names, spellsvc.NameMatch{Key: key, Tier: tier.tier})
		}
	}
	switch c := strings.ToLower(q.Get("class")); c {
	case "", "both", "all":
	case "wizard", "priest":
		f.Class = c
	case "mine":
		f.Class, f.Mine = "wizard", true
	default:
		return f, render.BadRequestError("class must be mine, wizard, priest or all")
	}
	for _, raw := range splitCSV(q.Get("level")) {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 || n > 9 {
			return f, render.BadRequestError("level must be a list of levels from 0 to 9")
		}
		if !slices.Contains(f.Levels, n) {
			f.Levels = append(f.Levels, n)
		}
	}
	if raw := strings.ToLower(q.Get("school")); raw != "" {
		if !slices.Contains(spellsvc.SchoolGroups(), raw) {
			return f, render.BadRequestError("school must be one of " + strings.Join(spellsvc.SchoolGroups(), ", "))
		}
		f.School = raw
	}
	if raw := q.Get("sphere"); raw != "" {
		s, ok := spellsvc.IsSphere(raw)
		if !ok {
			return f, render.BadRequestError("sphere must be one of " + strings.Join(spellsvc.Spheres(), ", "))
		}
		f.Sphere = s
	}
	for _, c := range strings.FieldsFunc(q.Get("components"), func(r rune) bool { return r == ',' || r == ' ' }) {
		c = strings.ToUpper(c)
		if c != "V" && c != "S" && c != "M" {
			return f, render.BadRequestError("components must be a list of V, S, M")
		}
		f.Components = append(f.Components, c)
	}
	if raw := q.Get("save"); raw != "" {
		kind, ok := saveKinds[strings.ToLower(strings.TrimRight(raw, "."))]
		if !ok {
			return f, render.BadRequestError("save must be none, neg, 1/2 or special")
		}
		f.Save = kind
	}
	if raw := strings.ToLower(q.Get("casting_time")); raw != "" {
		if !slices.Contains(castingTimes, raw) {
			return f, render.BadRequestError("casting_time must be one of " + strings.Join(castingTimes, ", "))
		}
		f.CastingTime = raw
	}
	return f, nil
}

var matchTiers = []struct {
	param string
	tier  spellsvc.Tier
}{
	{"match_exact", spellsvc.TierExact},
	{"match_prefix", spellsvc.TierPrefix},
	{"match_contains", spellsvc.TierContains},
}

// splitCSV splits "a, b,,c" into a, b, c.
func splitCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

var saveKinds = map[string]string{
	"none": spellstats.SaveNone, "neg": spellstats.SaveNegates, "negates": spellstats.SaveNegates,
	"1/2": spellstats.SaveHalf, "half": spellstats.SaveHalf, "special": spellstats.SaveSpecial,
}

var castingTimes = []string{"segment", "round", "turn", "hour", "day", "week", "special"}

func casterLevel(r *http.Request, charID CharacterIDFunc, chars characterGetter) (int, error) {
	if raw := r.URL.Query().Get("caster_level"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			return 0, render.BadRequestError("caster_level must be a positive integer")
		}
		return n, nil
	}
	// No character (or a failed lookup) must not break the catalog: the
	// spells are returned without computed values.
	c, err := chars.GetCharacter(r.Context(), charID(r))
	if err != nil || c == nil {
		return 0, nil //nolint:nilerr // deliberate fallback, see above
	}
	return c.Level, nil
}

// SpellSlotDTO matches the previous wire shape.
type SpellSlotDTO struct {
	Level             int    `json:"level"`
	PreparedSpellID   string `json:"prepared_spell_id"`
	PreparedSpellName string `json:"prepared_spell_name"`
	Used              bool   `json:"used"`
	// Extra: a slot granted by a spell (grant_slots), blank until cast, gone at the long rest.
	Extra bool `json:"extra,omitempty"`
}

// SpellDTO is the wire shape of a spell. The six stat fields keep their raw
// PHB text; Stats is their typed form and Values the computed values at
// CasterLevel (domain types carry the JSON tags).
type SpellDTO struct {
	// ID is the spell's unique key; cast and receive take it.
	ID           string `json:"id"`
	Name         string `json:"name"`
	Class        string `json:"class"`
	Level        int    `json:"level"`
	School       string `json:"school"`
	Sphere       string `json:"sphere"`
	CastingTime  string `json:"casting_time"`
	Range        string `json:"range"`
	Duration     string `json:"duration"`
	AreaOfEffect string `json:"area_of_effect"`
	Components   string `json:"components"`
	SavingThrow  string `json:"saving_throw"`
	Description  string `json:"description"`

	Stats       spellstats.Stats   `json:"stats"`
	CasterLevel int                `json:"caster_level,omitempty"`
	Values      *spellstats.Values `json:"values,omitempty"`
}

// DiceGroupDTO is Count dice of Sides sides.
type DiceGroupDTO struct {
	Count int `json:"count"`
	Sides int `json:"sides"`
}

// DiceSumDTO is a formula evaluated at a caster level down to what is left
// to roll: dice groups, in the order the spell's data writes them, plus a
// constant. No groups: a plain number.
type DiceSumDTO struct {
	Groups   []DiceGroupDTO `json:"groups"`
	Modifier int            `json:"modifier"`
}

// SpellDamageDTO is the damage a cast of a spell rolls at CasterLevel: Dice
// for one instance, Instances separate instances every round, for Rounds
// rounds; Type is the damage type key ("fire"). The spell lookup and the
// spellbook serve this same shape; the client words it.
type SpellDamageDTO struct {
	CasterLevel int        `json:"caster_level"`
	Dice        DiceSumDTO `json:"dice"`
	Instances   DiceSumDTO `json:"instances"`
	Rounds      DiceSumDTO `json:"rounds"`
	Type        string     `json:"type,omitempty"`
}

// DamageToDTO maps the damage at a caster level to its wire shape (nil: none).
func DamageToDTO(d *spelleffectsvc.DamageAt) *SpellDamageDTO {
	if d == nil {
		return nil
	}
	return &SpellDamageDTO{
		CasterLevel: d.CasterLevel, Dice: sumToDTO(d.Dice), Instances: sumToDTO(d.Instances),
		Rounds: sumToDTO(d.Rounds), Type: d.Type,
	}
}

func sumToDTO(s formula.Sum) DiceSumDTO {
	out := DiceSumDTO{Groups: make([]DiceGroupDTO, 0, len(s.Dice)), Modifier: s.Constant}
	for _, g := range s.Dice {
		out.Groups = append(out.Groups, DiceGroupDTO{Count: g.Count, Sides: g.Sides})
	}
	return out
}

// SpellPageDTO is one page of GET /api/spells; Total counts every match.
type SpellPageDTO struct {
	Items    []SpellDTO `json:"items"`
	Total    int        `json:"total"`
	Page     int        `json:"page"`
	PageSize int        `json:"page_size"`
}

// SlotsToDTO maps slots to their wire shape.
func SlotsToDTO(in []spellsvc.SpellSlot) []SpellSlotDTO {
	out := make([]SpellSlotDTO, 0, len(in))
	for _, s := range in {
		out = append(out, SpellSlotDTO{Level: s.Level, PreparedSpellID: s.SpellID, PreparedSpellName: s.PreparedSpellName, Used: s.Used, Extra: s.Extra})
	}
	return out
}

func spellsToDTO(in []spellsvc.Spell, casterLevel int) []SpellDTO {
	out := make([]SpellDTO, 0, len(in))
	for _, s := range in {
		out = append(out, SpellToDTO(s, casterLevel))
	}
	return out
}

// SpellToDTO maps one spell to its wire shape, with values at casterLevel.
func SpellToDTO(s spellsvc.Spell, casterLevel int) SpellDTO {
	dto := SpellDTO{
		ID: s.ID, Name: s.Name, Class: s.Class, Level: s.Level, School: s.School, Sphere: s.Sphere,
		CastingTime: s.CastingTime, Range: s.Range, Duration: s.Duration,
		AreaOfEffect: s.AreaOfEffect, Components: s.Components, SavingThrow: s.SavingThrow,
		Description: s.Description,
		Stats:       s.Stats, CasterLevel: casterLevel,
	}
	if casterLevel > 0 {
		v := s.Stats.At(casterLevel)
		dto.Values = &v
	}
	return dto
}
