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
	"github.com/ceffo/critical/api/internal/rest/wire"
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

// RegisterRoutes wires the spell endpoints of the current character onto r.
func RegisterRoutes(r Router, charID CharacterIDFunc, svc spellsvc.SVC, chars characterGetter, damage damageReader) {
	r.Get("/api/spells/slots", render.Handle(GetSlots(charID, svc)))
	r.Get("/api/spells", render.Handle(Search(charID, svc, chars, damage)))
	r.Get("/api/spells/{id}", render.Handle(Get(charID, svc, chars, damage)))
}

// RegisterSharedRoutes wires the catalogue's reference reads, which no
// character owns (ADR 0029 step 7.5), onto r.
func RegisterSharedRoutes(r Router, svc spellsvc.SVC) {
	r.Get("/api/spells/filters", render.Handle(FilterOptions(svc)))
	r.Get("/api/spells/names", render.Handle(Names(svc)))
}

// SpellNameDTO is one spell's id with its name in every language the
// catalogue has: the client names a spell it knows only by id (spellbook,
// slots, journal) in the player's language.
type SpellNameDTO struct {
	ID   string         `json:"id"`
	Name localized.Text `json:"name"`
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
			out = append(out, SpellNameDTO{ID: n.ID, Name: wire.Text(n.Name)})
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
		render.OK(r.Context(), w, wire.SlotsToDTO(slots))
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
func enrich(ctx context.Context, items []wire.SpellDTO, level int, damage damageReader) error {
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
		items[i].Damage = d.Wire()
		modes, err := damage.ModesAt(ctx, items[i].ID, level)
		if err != nil {
			return err
		}
		items[i].Modes = spelleffectsvc.ModesWire(modes)
		x, err := damage.ExposureAt(ctx, items[i].ID, level)
		if err != nil {
			return err
		}
		items[i].Exposure = x.Wire()
	}
	return nil
}

// Get returns one spell by its id (GET /api/spells/{id}), the same wire.SpellDTO
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
		items := []wire.SpellDTO{wire.SpellToDTO(sp, level)}
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
	// The caster level is the realized level (ADR 0030 step 7).
	return c.Derived.RealizedLevel, nil
}

// SpellPageDTO is one page of GET /api/spells; Total counts every match.
type SpellPageDTO struct {
	Items    []wire.SpellDTO `json:"items"`
	Total    int             `json:"total"`
	Page     int             `json:"page"`
	PageSize int             `json:"page_size"`
}

func spellsToDTO(in []spellsvc.Spell, casterLevel int) []wire.SpellDTO {
	out := make([]wire.SpellDTO, 0, len(in))
	for _, s := range in {
		out = append(out, wire.SpellToDTO(s, casterLevel))
	}
	return out
}
