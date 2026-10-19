package composition

import (
	"context"

	"github.com/ceffo/critical/api/internal/adapters/actionlogdal"
	logq "github.com/ceffo/critical/api/internal/adapters/actionlogdal/queries"
	"github.com/ceffo/critical/api/internal/adapters/attackformdal"
	formq "github.com/ceffo/critical/api/internal/adapters/attackformdal/queries"
	"github.com/ceffo/critical/api/internal/adapters/authdal"
	authq "github.com/ceffo/critical/api/internal/adapters/authdal/queries"
	"github.com/ceffo/critical/api/internal/adapters/buffdal"
	buffq "github.com/ceffo/critical/api/internal/adapters/buffdal/queries"
	"github.com/ceffo/critical/api/internal/adapters/characterdal"
	charq "github.com/ceffo/critical/api/internal/adapters/characterdal/queries"
	"github.com/ceffo/critical/api/internal/adapters/critdal"
	critq "github.com/ceffo/critical/api/internal/adapters/critdal/queries"
	"github.com/ceffo/critical/api/internal/adapters/dicedal"
	diceq "github.com/ceffo/critical/api/internal/adapters/dicedal/queries"
	"github.com/ceffo/critical/api/internal/adapters/hpdal"
	hpq "github.com/ceffo/critical/api/internal/adapters/hpdal/queries"
	"github.com/ceffo/critical/api/internal/adapters/itemdal"
	itemq "github.com/ceffo/critical/api/internal/adapters/itemdal/queries"
	"github.com/ceffo/critical/api/internal/adapters/skilldal"
	skillq "github.com/ceffo/critical/api/internal/adapters/skilldal/queries"
	"github.com/ceffo/critical/api/internal/adapters/spellbookadpt"
	"github.com/ceffo/critical/api/internal/adapters/spellbookdal"
	spellbookq "github.com/ceffo/critical/api/internal/adapters/spellbookdal/queries"
	"github.com/ceffo/critical/api/internal/adapters/spelldal"
	spellq "github.com/ceffo/critical/api/internal/adapters/spelldal/queries"
	"github.com/ceffo/critical/api/internal/adapters/textdal"
	textq "github.com/ceffo/critical/api/internal/adapters/textdal/queries"
	"github.com/ceffo/critical/api/internal/adapters/xpdal"
	xpq "github.com/ceffo/critical/api/internal/adapters/xpdal/queries"
	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/catalogue"
	"github.com/ceffo/critical/api/internal/domain/service/actionlogsvc"
	"github.com/ceffo/critical/api/internal/domain/service/authsvc"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/castsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
	"github.com/ceffo/critical/api/internal/domain/service/detailssvc"
	"github.com/ceffo/critical/api/internal/domain/service/dicesvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/domain/service/learnsvc"
	"github.com/ceffo/critical/api/internal/domain/service/levelupsvc"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
	"github.com/ceffo/critical/api/internal/domain/service/skillsvc"
	"github.com/ceffo/critical/api/internal/domain/service/spellsvc"
	"github.com/ceffo/critical/api/internal/domain/service/xpsvc"
	"github.com/ceffo/critical/api/internal/storage/memory"
	"github.com/ceffo/critical/api/internal/storage/postgres"
)

// Store is the database handle as the startup sequence uses it: it runs the
// transactions of every service and the schema steps of ADR 0025 step 3
// (migrations with the unknown-migration guard), then the seeds and the
// catalogue import. Close releases it.
type Store interface {
	domain.TxRunner
	// Migrate applies the pending migrations; seedsDir's character.json names
	// the seed character, the owner of rows a migration finds without one;
	// catalogue is the catalogue's weapons the equipment conversion matches
	// (ADR 0033 step 26.4).
	Migrate(ctx context.Context, seedsDir string, catalogue []ruletablesvc.Weapon) error
	// Seed loads the seed files of seedsDir; spellbook is the spellbook
	// seed's catalogue keys, checked against the catalogue; catalogue the
	// catalogue's weapons the seeded weapons are matched to.
	Seed(ctx context.Context, seedsDir string, spellbook []string, catalogue []ruletablesvc.Weapon) error
	ImportCatalogue(ctx context.Context, cat catalogue.Catalogue, seedRefs []catalogue.Ref) error
	Close() error
}

// CharacterStore is the character adapter: the sheet reads, who owns and
// plays which character, the details edit and the level-up's hit points.
type CharacterStore interface {
	charactersvc.Reader
	charactersvc.Owners
	detailssvc.DAL
	levelupsvc.DAL
	skillsvc.Points
}

// BuffStore is the buff adapter plus the round read the action log stamps
// entries with.
type BuffStore interface {
	buffsvc.DAL
	GetRoundNumber(ctx context.Context, characterID int64) (int, error)
}

// SpellStore is the spell adapter: slots, the catalogue reads and the spell
// texts.
type SpellStore interface {
	spellsvc.DAL
	spellsvc.Reader
	castsvc.Texts
}

// SpellbookStore is the spellbook adapter: each character's spellbook, read
// by spellbookadpt's Book, and written by learning a spell (learnsvc).
type SpellbookStore interface {
	spellbookadpt.Store
	learnsvc.DAL
}

// Storage is every storage port the root wires into the services (ADR 0025
// step 6.1: the services, this root and the router are tested on mocks of
// these).
type Storage struct {
	Store     Store
	Character CharacterStore
	HP        actionlogsvc.MaxHP
	Log       actionlogsvc.DAL
	Buff      BuffStore
	Spell     SpellStore
	Crit      combatsvc.CritStore
	Items     itemsvc.DAL
	Forms     combatsvc.FormStore
	XP        xpsvc.DAL
	Skill     skillsvc.DAL
	Spellbook SpellbookStore
	Dice      dicesvc.DAL
	Auth      authsvc.DAL
}

// postgresStorage wires the PostgreSQL adapters over db. The sqlc-generated
// queries run on the transaction the context carries, else on the pool (ADR
// 0025 step 5.2).
func postgresStorage(db *postgres.DB) Storage { return storageOn(db, postgres.Routed(db.Pool())) }

// storageOn builds every adapter's queries over conn, the one connection the
// root routes: a query package built over anything else would escape the
// caller's transaction.
func storageOn(store Store, conn postgres.DBTX) Storage {
	// Item names are localized texts (ADR 0023 step 9).
	texts := textdal.New(textq.New(conn))
	return Storage{
		Store:     store,
		Character: characterdal.New(charq.New(conn)),
		HP:        hpdal.New(hpq.New(conn)),
		Log:       actionlogdal.New(logq.New(conn)),
		Buff:      buffdal.New(buffq.New(conn)),
		Spell:     spelldal.New(spellq.New(conn)),
		Crit:      critdal.New(critq.New(conn)),
		Items:     itemdal.New(itemq.New(conn), texts),
		Forms:     attackformdal.New(formq.New(conn)),
		XP:        xpdal.New(xpq.New(conn)),
		Skill:     skilldal.New(skillq.New(conn)),
		Spellbook: spellbookdal.New(spellbookq.New(conn)),
		Dice:      dicedal.New(diceq.New(conn)),
		Auth:      authdal.New(authq.New(conn)),
	}
}

// memoryStorage wires the in-memory implementation of every storage port, all
// over the one store m, which runs the transactions (ADR 0025 step 8.1).
func memoryStorage(m *memory.Store) Storage {
	return Storage{
		Store:     m,
		Character: m.Characters(),
		HP:        m.Characters(),
		Log:       m.ActionLog(),
		Buff:      m.Buffs(),
		Spell:     m.Spells(),
		Crit:      m.Crit(),
		Items:     m.Items(),
		Forms:     m.Forms(),
		XP:        m.XP(),
		Skill:     m.Skills(),
		Spellbook: m.Spellbooks(),
		Dice:      m.Dice(),
		Auth:      m.Auth(),
	}
}
