package memory

import (
	"fmt"

	"github.com/google/uuid"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/acsvc"
	"github.com/ceffo/critical/api/internal/domain/service/authsvc"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/domain/service/spellsvc"
	"github.com/ceffo/critical/api/internal/domain/service/xpsvc"
	"github.com/ceffo/critical/api/internal/domain/spellbook"
)

// tables are the store's data, one field per table of the database schema.
// A value in a map or a slice is never changed in place: a write stores a new
// one (save relies on it). Ids come from a sequence per table that is never
// reused, as the database's AUTOINCREMENT ids, and roll back with the
// transaction that took them.
type tables struct {
	characters   map[int64]charactersvc.Character
	characterSeq int64
	// owners and retired are the characters' owner (none: nobody's yet) and
	// retired flag (ADR 0029 step 7); current is each user's current
	// character, read through the ownership.
	owners  map[int64]uuid.UUID
	retired map[int64]bool
	current map[uuid.UUID]int64

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

	// items (every piece of equipment, ADR 0033) belong to a character:
	// keyed by the character and the id, so two characters may hold the same
	// id. reviews is the review list (step 27), one key per row.
	items   map[owned]storedItem
	reviews map[review]bool
	// rowSeq orders items inserted at the same position as the database's
	// rowid does.
	rowSeq int64

	// crit holds the house crit table by ascending range start.
	crit  []combatsvc.CritRow
	forms map[int64]combatsvc.AttackForm

	xp    map[int64]xpEntry
	xpSeq int64

	// spellbooks are each character's, by character id,
	// in sheet order (a spellbook holds catalogue keys, each with its
	// origin).
	spellbooks map[int64][]spellbook.Entry
	// learnFailures are the spells each character failed to learn, by
	// catalogue key, at the realized level of the failure (ADR 0030 step 20.2).
	learnFailures map[int64]map[string]int

	// acOverrides are the characters' AC overrides (ADR 0033 step 13); no
	// entry: none.
	acOverrides map[int64]acsvc.Override
	// skillArmors are the skill armours entered before ADR 0033 step 14
	// (characters.skill_armor): written back by the undo of an old
	// skill_armor entry, read by nothing; none is none entered.
	skillArmors map[int64]string

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
		owners:        map[int64]uuid.UUID{},
		retired:       map[int64]bool{},
		current:       map[uuid.UUID]int64{},
		slots:         map[int64][]spellsvc.SpellSlot{},
		buffs:         map[int64][]buffsvc.BuffEffect{},
		rounds:        map[int64]roundState{},
		savedBuffs:    map[int64]savedBuff{},
		spells:        map[string]spellsvc.Spell{},
		items:         map[owned]storedItem{},
		reviews:       map[review]bool{},
		spellbooks:    map[int64][]spellbook.Entry{},
		learnFailures: map[int64]map[string]int{},
		forms:         map[int64]combatsvc.AttackForm{},
		xp:            map[int64]xpEntry{},
		acOverrides:   map[int64]acsvc.Override{},
		skillArmors:   map[int64]string{},
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

// review is one review row: the character, the piece and the reason.
type review struct {
	characterID    int64
	itemID, reason string
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
