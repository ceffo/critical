package memory

import (
	"fmt"

	"github.com/google/uuid"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/authsvc"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/domain/service/skillsvc"
	"github.com/ceffo/critical/api/internal/domain/service/spellsvc"
	"github.com/ceffo/critical/api/internal/domain/service/weaponsvc"
	"github.com/ceffo/critical/api/internal/domain/service/xpsvc"
)

// tables are the store's data, one field per table of the database schema.
// A value in a map or a slice is never changed in place: a write stores a new
// one (save relies on it). Ids come from a sequence per table that is never
// reused, as the database's AUTOINCREMENT ids, and roll back with the
// transaction that took them.
type tables struct {
	characters   map[int64]charactersvc.Character
	characterSeq int64

	slots  map[int64][]spellsvc.SpellSlot
	buffs  map[int64][]buffsvc.BuffEffect
	rounds map[int64]roundState

	savedBuffs   map[int64]savedBuff
	savedBuffSeq int64

	// log and corrections are append-only: the id of the row at index i is
	// i+1.
	log         []actionlog.Entry
	corrections []actionlog.Correction

	// spells are the catalogue's, by key; catalogue is the imported
	// artifact's format and version.
	spells    map[string]spellsvc.Spell
	catalogue *catalogueVersion

	// weapons and items belong to a character: keyed by the character
	// and the id, so two characters may hold the same id.
	weapons map[owned]storedWeapon
	items   map[owned]storedItem
	// rowSeq orders items and weapons inserted at the same position as the
	// database's rowid does.
	rowSeq int64

	// crit holds the house crit table by ascending range start.
	crit  []combatsvc.CritRow
	forms map[int64]combatsvc.AttackForm

	xp    map[int64]xpEntry
	xpSeq int64

	// proficiencies, bardSkills and spellbooks are each character's, by
	// character id, in sheet order (a spellbook holds catalogue keys).
	proficiencies map[int64][]skillsvc.ProficiencyDef
	bardSkills    map[int64][]skillsvc.BardSkillDef
	spellbooks    map[int64][]string

	dice    map[int64]diceFavourite
	diceSeq int64

	// users, invites and sessions of the login (ADR 0029); sessions by
	// their stored id (the token's hash).
	users    map[uuid.UUID]authsvc.User
	invites  map[uuid.UUID]storedInvite
	sessions map[string]authsvc.Session
}

func newTables() *tables {
	return &tables{
		characters:    map[int64]charactersvc.Character{},
		slots:         map[int64][]spellsvc.SpellSlot{},
		buffs:         map[int64][]buffsvc.BuffEffect{},
		rounds:        map[int64]roundState{},
		savedBuffs:    map[int64]savedBuff{},
		spells:        map[string]spellsvc.Spell{},
		weapons:       map[owned]storedWeapon{},
		items:         map[owned]storedItem{},
		proficiencies: map[int64][]skillsvc.ProficiencyDef{},
		bardSkills:    map[int64][]skillsvc.BardSkillDef{},
		spellbooks:    map[int64][]string{},
		forms:         map[int64]combatsvc.AttackForm{},
		xp:            map[int64]xpEntry{},
		dice:          map[int64]diceFavourite{},
		users:         map[uuid.UUID]authsvc.User{},
		invites:       map[uuid.UUID]storedInvite{},
		sessions:      map[string]authsvc.Session{},
	}
}

// owned is the key of a row a character owns: the character and the row's id.
type owned struct {
	characterID int64
	id          string
}

// roundState is the round counter and the round's initiative of a character.
type roundState struct {
	round      int
	initiative *buffsvc.Initiative
}

type savedBuff struct {
	characterID int64
	buffsvc.SavedBuff
}

type catalogueVersion struct {
	format  int
	version string
}

// storedWeapon is a weapon with the insertion order that breaks a position
// tie.
type storedWeapon struct {
	weaponsvc.Weapon
	row int64
}

// storedItem is an item with its powers in itemsvc's stored encoding (read
// back through itemsvc.DecodePowers, as the database adapter does) and the
// insertion order that breaks a position tie.
type storedItem struct {
	itemsvc.Item // Powers is nil: they are in powers
	powers       string
	row          int64
}

type xpEntry struct {
	characterID int64
	xpsvc.Entry
}

type diceFavourite struct {
	characterID int64
	id          int64
	formula     string
	position    int64
}

// nextID is the next id of a sequence: one more than the highest taken.
func nextID(seq *int64) int64 {
	*seq++
	return *seq
}

// takeID records an id given by the caller (a row restored under its own
// id) in its sequence.
func takeID(seq *int64, id int64) {
	*seq = max(*seq, id)
}

// errDuplicate is a unique-key violation, as the database refuses one.
func errDuplicate(table string, key any) error {
	return fmt.Errorf("%s: duplicate key %v", table, key)
}
