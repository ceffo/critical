// Package spellhdl exposes /api/spells.
package spellhdl

import (
	"context"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/ceffo/critical/api/internal/domain/localized"
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
	Search(ctx context.Context, characterID int64, f spellsvc.Filter, p spellsvc.PageRequest) (spellsvc.Page, error)
	Get(ctx context.Context, id string) (spellsvc.Spell, error)
	Names(ctx context.Context) ([]spellsvc.SpellName, error)
	Levels(ctx context.Context) ([]int, error)
}

type characterGetter interface {
	GetCharacter(ctx context.Context, id int64) (*charactersvc.Character, error)
}

// damageReader serves the damage a cast rolls at a caster level and whether
// a spell can be added as an effect (castsvc satisfies it).
type damageReader interface {
	AddsEffect(ctx context.Context, spellID string) bool
	DamageAt(ctx context.Context, spellID string, casterLevel int) (*spelleffectsvc.DamageAt, error)
	ModesAt(ctx context.Context, spellID string, casterLevel int) ([]spelleffectsvc.ModeAt, error)
	ExposureAt(ctx context.Context, spellID string, casterLevel int) (*spelleffectsvc.ExposureAt, error)
}

// RegisterRoutes wires spell endpoints onto r.
func RegisterRoutes(r Router, charID CharacterIDFunc, svc spellsvc.SVC, chars characterGetter, damage damageReader) {
	r.Get("/api/spells/slots", render.Handle(GetSlots(charID, svc)))
	r.Get("/api/spells", render.Handle(Search(charID, svc, chars, damage)))
	r.Get("/api/spells/filters", render.Handle(FilterOptions(svc)))
	r.Get("/api/spells/names", render.Handle(Names(svc)))
	r.Get("/api/spells/{id}", render.Handle(Get(charID, svc, chars, damage)))
}

// TextDTO is a text in human languages (ADR 0023 step 6): body by language
// code, only the languages the catalogue has (`{}` for a text in none, such
// as an absent sphere). The client picks its language and falls back to
// "en" per field.
type TextDTO map[string]string

// textDTO is t as a TextDTO, never null on the wire.
func textDTO(t localized.Text) TextDTO {
	out := make(TextDTO, len(t))
	for lang, body := range t {
		out[lang] = body
	}
	return out
}

// SpellNameDTO is one spell's id with its name in every language the
// catalogue has: the client names a spell it knows only by id (spellbook,
// slots, journal) in the player's language.
type SpellNameDTO struct {
	ID   string  `json:"id"`
	Name TextDTO `json:"name"`
}

// Names returns every catalogue spell's names (GET /api/spells/names).
func Names(svc spellSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		names, err := svc.Names(r.Context())
		if err != nil {
			return err
		}
		out := make([]SpellNameDTO, 0, len(names))
		for _, n := range names {
			out = append(out, SpellNameDTO{ID: n.ID, Name: textDTO(n.Name)})
		}
		render.OK(r.Context(), w, out)
		return nil
	}
}

// FilterOptionsDTO lists the choices of the lookup's filters: the school
// group keys (the client labels them), the Priest sphere names and the spell
// levels the catalogue holds (ascending).
type FilterOptionsDTO struct {
	Schools []string `json:"schools"`
	Spheres []string `json:"spheres"`
	Levels  []int    `json:"levels"`
}

// FilterOptions returns the school groups, spheres and levels GET /api/spells takes.
func FilterOptions(svc spellSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		levels, err := svc.Levels(r.Context())
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, FilterOptionsDTO{Schools: spellsvc.SchoolGroups(), Spheres: spellsvc.Spheres(), Levels: levels})
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
// (1 to spelleffectsvc.MaxCasterLevel) sets the level the computed values and
// the damage dice are for; it defaults to the character's.
func Search(charID CharacterIDFunc, svc spellSvc, chars characterGetter, damage damageReader) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		f, err := parseFilter(r.URL.Query())
		if err != nil {
			return err
		}
		pr, err := parsePage(r.URL.Query())
		if err != nil {
			return err
		}
		page, err := svc.Search(r.Context(), charID(r), f, pr)
		if err != nil {
			return err
		}
		level, err := casterLevel(r, charID, chars)
		if err != nil {
			return err
		}
		items := spellsToDTO(page.Items, level)
		if err := enrich(r.Context(), items, level, damage); err != nil {
			return err
		}
		render.OK(r.Context(), w, SpellPageDTO{
			Items: items, Total: page.Total, Page: page.Page, PageSize: page.PageSize,
		})
		return nil
	}
}

// enrich adds what the catalogue rows do not carry to items: whether a cast
// adds an effect, and, at a caster level the formulas are checked at, the
// damage, modes and exposure at that level. An explicit caster_level is
// bounded above; the character's level is not, and a level the formulas are
// not checked at gets no damage rather than failing the request.
func enrich(ctx context.Context, items []SpellDTO, level int, damage damageReader) error {
	for i := range items {
		items[i].Effect = damage.AddsEffect(ctx, items[i].ID)
	}
	if level < 1 || level > spelleffectsvc.MaxCasterLevel {
		return nil
	}
	for i := range items {
		d, err := damage.DamageAt(ctx, items[i].ID, level)
		if err != nil {
			return err
		}
		items[i].Damage = DamageToDTO(d)
		modes, err := damage.ModesAt(ctx, items[i].ID, level)
		if err != nil {
			return err
		}
		items[i].Modes = ModesToDTO(modes)
		x, err := damage.ExposureAt(ctx, items[i].ID, level)
		if err != nil {
			return err
		}
		items[i].Exposure = ExposureToDTO(x)
	}
	return nil
}

// Get returns one spell by its id (GET /api/spells/{id}), the same SpellDTO
// as a search item, at the optional caster_level (default: the character's).
// An id the catalogue does not hold is a 404 (code spell_not_found).
func Get(charID CharacterIDFunc, svc spellSvc, chars characterGetter, damage damageReader) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		sp, err := svc.Get(r.Context(), chi.URLParam(r, "id"))
		if err != nil {
			return err
		}
		level, err := casterLevel(r, charID, chars)
		if err != nil {
			return err
		}
		items := []SpellDTO{SpellToDTO(sp, level)}
		if err := enrich(r.Context(), items, level, damage); err != nil {
			return err
		}
		render.OK(r.Context(), w, items[0])
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
//   - q: text matched against the English and the French name, case and
//     accents ignored (exact, prefix, substring; the best tier ranks).
//   - class: mine (the Bard's spellbook), wizard, priest, or all/both/empty.
//   - level: comma-separated levels, e.g. "1,3"; empty means any.
//   - school: a school group key (GET /api/spells/filters); sphere: a sphere.
//   - components, save, casting_time: typed-stat filters.
func parseFilter(q url.Values) (spellsvc.Filter, error) {
	f := spellsvc.Filter{Query: strings.TrimSpace(q.Get("q"))}
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
		if err != nil || n < 1 || n > spelleffectsvc.MaxCasterLevel {
			return 0, render.BadRequestError("caster_level must be an integer from 1 to " + strconv.Itoa(spelleffectsvc.MaxCasterLevel))
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

// SpellDTO is the wire shape of a spell. Every human-language text is a
// language map (TextDTO, ADR 0023): the client picks its language and falls
// back to English per field. The six stat fields keep their raw PHB text
// (English only); Stats is their typed form and Values the computed values
// at CasterLevel (domain types carry the JSON tags). Descriptions are in the
// catalogue's Markdown subset (paragraphs, bullet lists, pipe tables).
type SpellDTO struct {
	// ID is the spell's catalogue key; cast and receive take it.
	ID           string  `json:"id"`
	Name         TextDTO `json:"name"`
	Class        string  `json:"class"`
	Level        int     `json:"level"`
	School       TextDTO `json:"school"`
	Sphere       TextDTO `json:"sphere"`
	CastingTime  TextDTO `json:"casting_time"`
	Range        TextDTO `json:"range"`
	Duration     TextDTO `json:"duration"`
	AreaOfEffect TextDTO `json:"area_of_effect"`
	Components   TextDTO `json:"components"`
	SavingThrow  TextDTO `json:"saving_throw"`
	Description  TextDTO `json:"description"`
	// Source is the book code of the English text (PHB, TOM, ...; WEB when
	// no book was matched).
	Source string `json:"source"`

	Stats       spellstats.Stats   `json:"stats"`
	CasterLevel int                `json:"caster_level,omitempty"`
	Values      *spellstats.Values `json:"values,omitempty"`
	// Damage is what a cast rolls at CasterLevel (absent: no caster level,
	// or a cast rolls no damage).
	Damage *SpellDamageDTO `json:"damage,omitempty"`
	// Modes are the cast-time modes a cast or a receive picks from, each with
	// the damage a cast in it rolls at CasterLevel (the first is the default;
	// Damage is its damage). Absent: the spell has none.
	Modes []SpellModeDTO `json:"modes,omitempty"`
	// Exposure is the damage of one round of exposure to the spell's area
	// at CasterLevel (absent: none, or no caster level; ADR 0008 step 16).
	Exposure *SpellExposureDTO `json:"exposure,omitempty"`
	// Effect: POST /api/spells/effect takes the spell, it adds a buff
	// (absent: it adds none; ADR 0004 step 19).
	Effect bool `json:"effect,omitempty"`
}

// SpellExposureDTO is a spell's exposure damage at a caster level: one
// damage, or the modes picked at each exposure (the first is the default),
// each with its damage.
type SpellExposureDTO struct {
	Damage *SpellDamageDTO `json:"damage,omitempty"`
	Modes  []SpellModeDTO  `json:"modes,omitempty"`
}

// ExposureToDTO maps an exposure damage to its wire shape (nil: none).
func ExposureToDTO(x *spelleffectsvc.ExposureAt) *SpellExposureDTO {
	if x == nil {
		return nil
	}
	return &SpellExposureDTO{Damage: DamageToDTO(x.Damage), Modes: ModesToDTO(x.Modes)}
}

// SpellModeDTO is one cast-time mode: its key (the client words it) and the
// damage a cast in it rolls (absent: none).
type SpellModeDTO struct {
	Key    string          `json:"key"`
	Damage *SpellDamageDTO `json:"damage,omitempty"`
}

// ModesToDTO maps cast-time modes to their wire shape (nil: none).
func ModesToDTO(modes []spelleffectsvc.ModeAt) []SpellModeDTO {
	var out []SpellModeDTO
	for _, m := range modes {
		out = append(out, SpellModeDTO{Key: m.Key, Damage: DamageToDTO(m.Damage)})
	}
	return out
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
	// Parts are the parts of one instance, rolled separately, each with its
	// save effect key (none, negates); Dice adds them up. Absent when the
	// damage has no parts.
	Parts []SpellDamagePartDTO `json:"parts,omitempty"`
	// StepsPer ("round", "instance") and Steps are the dice of every round or
	// instance of a damage whose dice change, in order; Dice is then empty
	// and Rounds or Instances counts the steps. Absent without steps.
	StepsPer string               `json:"steps_per,omitempty"`
	Steps    []SpellDamageStepDTO `json:"steps,omitempty"`
}

// SpellDamageStepDTO is one dice step at the caster level: its 1-based round
// or instance number and its dice (no groups and 0: no damage that step).
type SpellDamageStepDTO struct {
	Step int        `json:"step"`
	Dice DiceSumDTO `json:"dice"`
}

// SpellDamagePartDTO is one damage part at the caster level.
type SpellDamagePartDTO struct {
	Dice DiceSumDTO `json:"dice"`
	Save string     `json:"save"`
}

// DamageToDTO maps the damage at a caster level to its wire shape (nil: none).
func DamageToDTO(d *spelleffectsvc.DamageAt) *SpellDamageDTO {
	if d == nil {
		return nil
	}
	out := &SpellDamageDTO{
		CasterLevel: d.CasterLevel, Dice: sumToDTO(d.Dice), Instances: sumToDTO(d.Instances),
		Rounds: sumToDTO(d.Rounds), Type: d.Type, StepsPer: string(d.StepsPer),
	}
	for _, st := range d.Steps {
		out.Steps = append(out.Steps, SpellDamageStepDTO{Step: st.Step, Dice: sumToDTO(st.Dice)})
	}
	for _, p := range d.Parts {
		out.Parts = append(out.Parts, SpellDamagePartDTO{Dice: sumToDTO(p.Dice), Save: string(p.Save)})
	}
	return out
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
		ID: s.ID, Name: textDTO(s.Name), Class: s.Class, Level: s.Level,
		School: textDTO(s.School), Sphere: textDTO(s.Sphere),
		CastingTime: textDTO(s.CastingTime), Range: textDTO(s.Range), Duration: textDTO(s.Duration),
		AreaOfEffect: textDTO(s.AreaOfEffect), Components: textDTO(s.Components),
		SavingThrow: textDTO(s.SavingThrow), Description: textDTO(s.Description), Source: s.Source,
		Stats: s.Stats, CasterLevel: casterLevel,
	}
	if casterLevel > 0 {
		v := s.Stats.At(casterLevel)
		dto.Values = &v
	}
	return dto
}
