// Package charactersvc owns read access to a character: the entered values
// (abilities, HP max, class, race, starting XP) and the values the rules
// derive from them (level, THAC0, saving throws, spell slots; ADR 0030 steps 3
// to 8), and which characters a user owns and plays (ADR 0029 step 7):
// creating, switching, renaming and retiring them. The seed character is
// written by the storage layer's seeding.
package charactersvc

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/google/uuid"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/ruleset"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
	"github.com/ceffo/critical/api/internal/domain/skills"
	"github.com/ceffo/critical/api/internal/domain/spellbook"
)

// ErrNoCurrentCharacter is a per-character request of a user who has no
// current character (ADR 0029 step 7.2): a new player, before they create
// their first one. Its stable code tells the web to offer that.
var ErrNoCurrentCharacter = fmt.Errorf("no current character: %w", domain.ErrNotFound)

// ErrCharacterChanged is a request for a character that is not the user's
// current one any more (ADR 0029 step 7.2): the user switched elsewhere (the
// other server, another tab). Nothing was read or written; the web reloads to
// show the current character.
var ErrCharacterChanged = fmt.Errorf("current character changed: %w", domain.ErrConflict)

// SVC is the driver port.
type SVC interface {
	domain.Service
	GetCharacter(ctx context.Context, id int64) (*Character, error)
	// WeaponProficiency is what character id's weapon picks make of its
	// weapons: the type it is specialized in (ADR 0037 step 9) and the
	// non-proficiency penalty of each type (step 8), from the character row
	// alone: no XP ledger or armour read (ADR 0025 step 10).
	WeaponProficiency(ctx context.Context, id int64) (skills.WeaponProficiency, error)
	// ClassEntries are character id's class entries as stored, with no
	// derivation, XP ledger or armour read: what the XP award's split reads
	// (ADR 0042 step 5.1).
	ClassEntries(ctx context.Context, id int64) ([]classes.Entry, error)
	// NightsPSPRecovery is the PSPs a night's sleep recovers for character
	// id (ADR 0043 step 7), before the cap at the maximum: 0 for a character
	// with no psionics entry. No XP ledger or armour read.
	NightsPSPRecovery(ctx context.Context, id int64) (int, error)
	// List is the owner's characters they have not retired, in creation
	// order (ADR 0029 steps 7.1 and 7.3).
	List(ctx context.Context, owner uuid.UUID) ([]Summary, error)
	// SetCurrent makes the owner's character id their current character
	// (step 7.2). Another user's character, a retired one and an unknown id
	// are all domain.ErrCharacterNotFound (step 7.1: a foreign character is
	// not revealed).
	SetCurrent(ctx context.Context, owner uuid.UUID, id int64) error
	// Create stores a new character of the owner's and makes it their
	// current character (ADR 0029 step 7.2); it starts with no weapon or
	// item, and with the picks, skill points, languages and starting
	// spellbook entered (ADR 0030 step 20.7: no die is rolled). A
	// *domain.FieldError for invalid input (see NewCharacter).
	Create(ctx context.Context, owner uuid.UUID, n NewCharacter) (int64, error)
	// Draft reads a creation draft as far as it is entered (ADR 0034, amendment
	// of 2026-10-20): the character Create would store, derived and warned as
	// a read of it, Create's refusals, the fields still blank and the class
	// minimums missed; blank names the numbers not entered. It refuses
	// nothing and stores nothing.
	Draft(ctx context.Context, n NewCharacter, blank []string) (*Draft, error)
	// Change renames or retires the owner's character id (step 7.3).
	// Another user's character, a retired one and an unknown id are all
	// domain.ErrCharacterNotFound; retiring the current character makes the
	// owner's first one left current (none: no current character).
	// It returns the owner's current character after the change (0: none).
	Change(ctx context.Context, owner uuid.UUID, id int64, c Change) (current int64, err error)
	// CreationOptions are the classes a new character may have, the races
	// each admits and the entered values' ranges.
	CreationOptions(ctx context.Context) (CreationOptions, error)
}

// Reader is the read-side driven port. Per clean_architecture.md §5.4, reads
// don't need the full DAL detour; this port returns domain entities directly.
type Reader interface {
	GetCharacter(ctx context.Context, id int64) (*Character, error)
}

// XPLedger is what the XP ledger's entries add to the starting XP (ADR 0015
// step 1): each award's amount, base plus bonus, which a multi-class
// character splits award by award (ADR 0042 step 5); xpsvc.Totals implements
// it over the ledger's DAL.
type XPLedger interface {
	Awards(ctx context.Context, characterID int64) ([]Award, error)
}

// HPLedger is the action log's HP and PSP totals: the sums of the effective
// HP and PSP deltas of a character's entries (ADR 0007 step 2, ADR 0043 step
// 5; actionlogsvc.SVC implements it).
type HPLedger interface {
	HPTotal(ctx context.Context, characterID int64) (int, error)
	PSPTotal(ctx context.Context, characterID int64) (int, error)
}

// Owners is the driven port over who owns and plays which character.
type Owners interface {
	// CharactersOf is the owner's characters they have not retired, by id.
	CharactersOf(ctx context.Context, owner uuid.UUID) ([]Summary, error)
	// SetCurrentCharacter sets the user's current character when the user
	// owns it and has not retired it, else returns
	// domain.ErrCharacterNotFound and changes nothing (one statement: no
	// check-then-write).
	SetCurrentCharacter(ctx context.Context, owner uuid.UUID, id int64) error
	// CreateCharacter stores c (its ID is ignored) as the owner's, with no
	// spell prepared, no buff and round 0, and returns its id.
	CreateCharacter(ctx context.Context, owner uuid.UUID, c Character) (int64, error)
	// RenameCharacter and RetireCharacter change the owner's character id
	// when the owner has it and has not retired it, else return
	// domain.ErrCharacterNotFound and change nothing (one statement each).
	RenameCharacter(ctx context.Context, owner uuid.UUID, id int64, name string) error
	RetireCharacter(ctx context.Context, owner uuid.UUID, id int64) error
	// RestoreSkillArmor writes a character's entered skill armour back for
	// the undo or redo of a skill_armor entry logged before ADR 0033 step 14
	// (no owner: the log already ties the entry to the character). Nothing
	// reads the column any more.
	RestoreSkillArmor(ctx context.Context, id int64, armor string) error
	// RepairCurrentCharacter makes the owner's first character not retired
	// current unless their current one is such a character (none left: no
	// current character).
	RepairCurrentCharacter(ctx context.Context, owner uuid.UUID) error
	// CurrentCharacter is the owner's current character, read through the
	// ownership: 0 when it is none, retired or not theirs.
	CurrentCharacter(ctx context.Context, owner uuid.UUID) (int64, error)
}

// Armour reads the body armour a character wears (ADR 0033 step 14): its
// PHB Table 29 column (one of ruletablesvc.SkillColumns), "" when it wears
// none. The composition root implements it over the equipment's store.
type Armour interface {
	WornSkillColumn(ctx context.Context, characterID int64) (string, error)
}

// FreeSpells is what ADR 0030 step 20.8 reads beyond the character row, for
// a class whose levels grant free spells (a mage): which levels were taken in
// the app and which free spells the spellbook already holds. It follows the
// ctx transaction.
type FreeSpells interface {
	// LevelsTaken are the realized levels the level-up roll reached (ADR
	// 0030 step 7), its entries the journal has not undone, each with the
	// class entry it was taken in (ADR 0042 step 7; "" for a roll recorded
	// before the entries, the first active one's); a character's starting
	// level and the levels before the first roll are not among them.
	LevelsTaken(ctx context.Context, characterID int64) ([]TakenLevel, error)
	// Claimed are the spellbook's spellbook.OriginLevelGrant entries, each
	// as its grant level and its spell's level.
	Claimed(ctx context.Context, characterID int64) ([]spellbook.FreeSpell, error)
}

// TakenLevel is a level a level-up roll reached in a class entry ("" for the
// first active one).
type TakenLevel struct {
	Class string
	Level int
}

// Sessions is the login's session cache (authsvc.SVC satisfies it): a user
// whose current character changed is read again by their next request.
type Sessions interface {
	ForgetUser(userID uuid.UUID)
}

// Deps are the service's collaborators.
type Deps struct {
	Reader   Reader
	XP       XPLedger
	Rules    Rules
	Owners   Owners
	Tx       domain.TxRunner
	Sessions Sessions
	// Armour is the worn body armour a thief's or a bard's skills read
	// (ADR 0033 step 14).
	Armour Armour
	// Languages is the language catalogue (ADR 0032 step 4); nil is the empty
	// one of a catalogue before format 8, which knows no language.
	Languages LanguageBook
	// Proficiencies is the non-weapon catalogue (ADR 0037 steps 1 and 2); nil
	// is the empty one.
	Proficiencies ProficiencyBook
	// HP is the HP ledger, for the current HP of the list (ADR 0007 step 6).
	HP HPLedger
	// Spells is the spell catalogue and Book the spellbook's store, for a new
	// mage's or bard's starting spellbook (ADR 0030 step 20.7).
	Spells SpellCatalogue
	Book   StartingBook
	// FreeSpells are the levels taken and the free spells claimed (ADR 0030
	// step 20.8).
	FreeSpells FreeSpells
	// Rulesets gives the ruleset a new character is created under (ADR 0041 step 3): its racial
	// lift (ADR 0042 step 2). nil is the default ruleset.
	Rulesets ruleset.Resolver
}

// defaultRulesets resolves every character to the default ruleset.
type defaultRulesets struct{}

func (defaultRulesets) ForCharacter(context.Context, int64) (ruleset.Ruleset, error) {
	return ruleset.Default(), nil
}

type service struct {
	reader   Reader
	xp       XPLedger
	rules    Rules
	owners   Owners
	tx       domain.TxRunner
	sessions Sessions
	armour   Armour
	// languages is never nil: the empty catalogue when Deps holds none.
	languages LanguageBook
	profs     ProficiencyBook
	hp        HPLedger
	spells    SpellCatalogue
	book      StartingBook
	free      FreeSpells
	rulesets  ruleset.Resolver
}

// New constructs the service.
func New(d Deps) (SVC, error) {
	if d.Reader == nil || d.XP == nil || d.Rules == nil || d.Owners == nil || d.Tx == nil || d.Sessions == nil || d.Armour == nil || d.HP == nil ||
		d.Spells == nil || d.Book == nil || d.FreeSpells == nil {
		return nil, errors.New("charactersvc: nil dependency")
	}
	if d.Languages == nil {
		d.Languages = (*ruletablesvc.Languages)(nil)
	}
	if d.Proficiencies == nil {
		d.Proficiencies = (*ruletablesvc.Proficiencies)(nil)
	}
	if d.Rulesets == nil {
		d.Rulesets = defaultRulesets{}
	}
	return &service{
		rulesets: d.Rulesets, reader: d.Reader, xp: d.XP, rules: d.Rules, owners: d.Owners, tx: d.Tx, sessions: d.Sessions, armour: d.Armour,
		languages: d.Languages, profs: d.Proficiencies, hp: d.HP, spells: d.Spells, book: d.Book, free: d.FreeSpells,
	}, nil
}

// GetCharacter reads the character and derives its level, THAC0, saves and
// slots from its class, race, abilities and XP total (ADR 0030 steps 3 to 8):
// every reader of a character (attacks, saves, preparation, casting, the XP
// ledger) gets the derived values, so an XP award that crosses a threshold
// changes them all at once.
func (s *service) GetCharacter(ctx context.Context, id int64) (*Character, error) {
	return s.read(ctx, id)
}

func (s *service) WeaponProficiency(ctx context.Context, id int64) (skills.WeaponProficiency, error) {
	c, err := s.reader.GetCharacter(ctx, id)
	if err != nil {
		return skills.WeaponProficiency{}, err
	}
	return skills.WeaponProficiencyOf(s.profs, c.StoredClasses(), c.Proficiencies), nil
}

func (s *service) ClassEntries(ctx context.Context, id int64) ([]classes.Entry, error) {
	c, err := s.reader.GetCharacter(ctx, id)
	if err != nil {
		return nil, err
	}
	return c.Entries(), nil
}

// read is GetCharacter: the stored character, derived and warned.
func (s *service) read(ctx context.Context, id int64) (*Character, error) {
	c, err := s.reader.GetCharacter(ctx, id)
	if err != nil {
		return nil, err
	}
	awards, err := s.xp.Awards(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("character %d XP: %w", id, err)
	}
	// ADR 0033 step 14: a thief's or a bard's skills read the worn body
	// armour (no other class reads it, so no other class pays the read);
	// any entry's class (ADR 0042 step 12).
	var column string
	for _, e := range c.Entries() {
		def, err := s.rules.Class(e.Class)
		if err != nil {
			return nil, fmt.Errorf("character %d: %w", id, err)
		}
		if _, ok := def.Lists(classes.KindRogueSkills); ok {
			if column, err = s.armour.WornSkillColumn(ctx, id); err != nil {
				return nil, fmt.Errorf("character %d worn armour: %w", id, err)
			}
			break
		}
	}
	if c.Derived, err = Derive(c, awards, column, s.rules); err != nil {
		return nil, fmt.Errorf("character %d: %w", id, err)
	}
	defs, err := s.rules.Definitions()
	if err != nil {
		return nil, fmt.Errorf("character %d: %w", id, err)
	}
	s.readWarnings(c, defs)
	if err := s.setPSP(ctx, c); err != nil {
		return nil, fmt.Errorf("character %d PSPs: %w", id, err)
	}
	if err := s.setFreeSpells(ctx, c); err != nil {
		return nil, fmt.Errorf("character %d free spells: %w", id, err)
	}
	return c, nil
}

// setFreeSpells is ADR 0030 step 20.8 on a read of c: each pending level's
// free spells, and the ones owed, granted by the levels taken in the app and
// not yet claimed. Only the entry whose class's levels grant them (its
// spellbook feature's free_spell_per_new_spell_level, ADR 0040 step 3; ADR
// 0042 step 12) pays the reads, and only its levels count.
func (s *service) setFreeSpells(ctx context.Context, c *Character) error {
	g, ok := c.Derived.Grant(classes.KindSpellbook)
	if !ok || c.Derived.Spellbook == nil {
		return nil
	}
	def, err := s.rules.Class(g.Class)
	if err != nil {
		return err
	}
	if f, _ := def.Lists(classes.KindSpellbook); !f.Spellbook.FreeSpellPerNewSpellLevel {
		return nil
	}
	entry, _ := c.Derived.Entry(g.Class)
	for i := range c.Derived.LevelUp.Pending {
		p := &c.Derived.LevelUp.Pending[i]
		if p.Class != g.Class {
			continue
		}
		if p.FreeSpells, err = s.freeSpellsAt(c, def, p.Level); err != nil {
			return err
		}
	}
	taken, err := s.free.LevelsTaken(ctx, c.ID)
	if err != nil {
		return err
	}
	var levels []int
	for _, t := range taken {
		if t.Class == g.Class || (t.Class == "" && g.Class == c.Class) {
			levels = append(levels, t.Level)
		}
	}
	slices.Sort(levels)
	var granted []spellbook.FreeSpell
	for _, level := range slices.Compact(levels) {
		if level > entry.RealizedLevel {
			continue // not realized any more: its grant went with it
		}
		got, err := s.freeSpellsAt(c, def, level)
		if err != nil {
			return err
		}
		granted = append(granted, got...)
	}
	claimed, err := s.free.Claimed(ctx, c.ID)
	if err != nil {
		return err
	}
	c.Derived.LevelUp.FreeSpellsOwed = spellbook.Owed(granted, claimed)
	return nil
}

// freeSpellsAt are the free spells level of the class def grants (step
// 20.8): the spell levels its slot table (step 8) opens there, up to the
// Intelligence's maximum spell level (PHB Table 4).
func (s *service) freeSpellsAt(c *Character, def classes.Class, level int) ([]spellbook.FreeSpell, error) {
	at, err := s.rules.SpellsPerDay(def, level, c.Wis)
	if err != nil {
		return nil, err
	}
	var below map[int]int
	if level > 1 {
		if below, err = s.rules.SpellsPerDay(def, level-1, c.Wis); err != nil {
			return nil, err
		}
	}
	return spellbook.FreeSpellsAt(level, below, at, c.Derived.Spellbook.IntMaxSpellLevel), nil
}

// readWarnings sets c's languages, pools, level grants and warnings as a read shows them. The
// native language is always among the languages (ADR 0032 step 4.1), a character stored before
// it was added included; the warnings follow what is read (steps 4.2, 5.2 and 8.4). The pools,
// their warnings and the level grants are the skills module's (ADR 0037 step 17).
func (s *service) readWarnings(c *Character, defs *classes.Registry) {
	native := s.languages.Native(c.Race)
	c.Details.Languages = skills.KnownLanguages(s.languages, c.Details.Languages)
	c.Details = c.Details.WithNative(native)
	var slotWarnings []skills.Warning
	c.Derived.Proficiencies, slotWarnings = skills.Read(s.profs, skills.Sheet{
		Class: c.Class, Level: c.Derived.RealizedLevel, Classes: c.ProficiencyClasses(), Intelligence: c.Int,
		Picks: c.Proficiencies, Languages: c.Details.Languages, Native: native,
		Score: func(a string) (int, bool) { return c.Score(Ability(a)) },
	})
	c.Warnings = c.Details.Warnings(alignmentsOfEntries(defs, c.Entries()))
	var identity *ruletablesvc.IdentityRow
	if row, ok := ruletablesvc.IdentityFor(defs, c.Race); ok {
		identity = &row
	}
	c.Warnings = append(c.Warnings, c.Details.RangeWarnings(identity)...)
	c.Warnings = append(c.Warnings, fromSkills(slotWarnings)...)
	c.Warnings = append(c.Warnings, LevelLimitWarnings(c.Race, c.Derived.Classes, s.rules.LevelLimit)...)
	// ADR 0037 step 15, once the pools are known: a level of one entry
	// among the character's (ADR 0042 steps 7 and 15).
	cls := c.ProficiencyClasses()
	for i := range c.Derived.LevelUp.Pending {
		p := &c.Derived.LevelUp.Pending[i]
		p.Grants = skills.GrantsOf(s.profs, cls, p.Class, p.Level)
	}
	c.Derived.LevelUp.ToSpend = skills.ToSpend(c.Derived.Proficiencies, c.Derived.SkillSets)
}

// LevelLimitWarnings are ADR 0042 step 7's per-entry check of the race's
// level limit (DMG Table 7, ADR 0041 step 5): each entry of entries, frozen
// ones included, whose realized level is above limit's cell for the race in
// its class warns level_limit with the class, the level (Value) and the
// limit (Max). The ruleset's level_limits has one mode, warn, so the check
// never refuses nor caps.
func LevelLimitWarnings(race string, entries []EntryDerived, limit func(race, class string) (ruletablesvc.LevelLimitRow, bool)) []Warning {
	var out []Warning
	for _, e := range entries {
		row, ok := limit(race, e.Class)
		if !ok || row.Unlimited || e.RealizedLevel <= row.MaxLevel {
			continue
		}
		level, limit := float64(e.RealizedLevel), float64(row.MaxLevel)
		out = append(out, Warning{Code: CodeLevelLimit, Field: FieldLevel, Class: e.Class, Value: &level, Max: &limit})
	}
	return out
}

// fromSkills are the skills module's warnings (ADR 0037 step 11) as the sheet's.
func fromSkills(ws []skills.Warning) []Warning {
	out := make([]Warning, 0, len(ws))
	for _, w := range ws {
		out = append(out, Warning{Code: w.Code, Field: w.Field, Pool: w.Pool, Spent: w.Spent, Available: w.Available, Left: w.Left})
	}
	return out
}

// List names each character with its realized level (ADR 0030 step 3), its
// race, alignment and sex, the fields still to choose (ADR 0032 step 8.2) and
// its HP: the max HP and the current HP, derived from the ledger as the HP
// panel derives it (ADR 0007 step 6).
func (s *service) List(ctx context.Context, owner uuid.UUID) ([]Summary, error) {
	list, err := s.owners.CharactersOf(ctx, owner)
	if err != nil {
		return nil, err
	}
	for i := range list {
		c, err := s.GetCharacter(ctx, list[i].ID)
		if err != nil {
			return nil, err
		}
		total, err := s.hp.HPTotal(ctx, list[i].ID)
		if err != nil {
			return nil, fmt.Errorf("character %d HP: %w", list[i].ID, err)
		}
		d := c.Details.Coded()
		list[i].Classes = c.Entries()
		list[i].Race = c.Race
		list[i].Alignment, list[i].Sex = d.Alignment, d.Sex
		list[i].Missing = c.Details.Missing()
		list[i].MaxHP = c.HPMax
		list[i].CurrentHP = actionlog.CurrentHP(c.HPMax, total)
	}
	return list, nil
}

// SetCurrent writes under the writer lock like every mutation (ADR 0025 step
// 4), outside the action log: choosing a character is no move of the game and
// is not undone. This server forgets the owner's sessions once it committed, so
// their next request plays the new character (ADR 0029 step 4.7).
func (s *service) SetCurrent(ctx context.Context, owner uuid.UUID, id int64) error {
	if err := s.tx.InTx(ctx, func(ctx context.Context) error {
		return s.owners.SetCurrentCharacter(ctx, owner, id)
	}); err != nil {
		return err
	}
	s.sessions.ForgetUser(owner)
	return nil
}

func (s *service) Health(_ context.Context) error { return nil }
func (s *service) Close(_ context.Context) error  { return nil }
