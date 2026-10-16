package composition

import (
	"context"

	"github.com/ceffo/critical/api/internal/adapters/actionlogdal"
	"github.com/ceffo/critical/api/internal/adapters/attackformdal"
	"github.com/ceffo/critical/api/internal/adapters/buffdal"
	"github.com/ceffo/critical/api/internal/adapters/characterdal"
	"github.com/ceffo/critical/api/internal/adapters/critdal"
	"github.com/ceffo/critical/api/internal/adapters/dicedal"
	"github.com/ceffo/critical/api/internal/adapters/hpdal"
	"github.com/ceffo/critical/api/internal/adapters/itemdal"
	"github.com/ceffo/critical/api/internal/adapters/skilldal"
	"github.com/ceffo/critical/api/internal/adapters/spelldal"
	"github.com/ceffo/critical/api/internal/adapters/weapondal"
	"github.com/ceffo/critical/api/internal/adapters/xpdal"
	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/catalogue"
	"github.com/ceffo/critical/api/internal/domain/service/actionlogsvc"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/castsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
	"github.com/ceffo/critical/api/internal/domain/service/detailssvc"
	"github.com/ceffo/critical/api/internal/domain/service/dicesvc"
	"github.com/ceffo/critical/api/internal/domain/service/hpsvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/domain/service/skillsvc"
	"github.com/ceffo/critical/api/internal/domain/service/spellsvc"
	"github.com/ceffo/critical/api/internal/domain/service/weaponsvc"
	"github.com/ceffo/critical/api/internal/domain/service/xpsvc"
	"github.com/ceffo/critical/api/internal/storage/sqlite"
)

// Store is the database handle as the startup sequence uses it: it runs the
// transactions of every service and the schema steps of ADR 0025 step 3
// (migrations with the unknown-migration guard), then the seeds and the
// catalogue import. Close releases it.
type Store interface {
	domain.TxRunner
	Migrate(ctx context.Context) error
	Seed(ctx context.Context, seedsDir string) error
	ImportCatalogue(ctx context.Context, cat catalogue.Catalogue, seedRefs []catalogue.Ref) error
	Close() error
}

// CharacterStore is the character adapter: the sheet reads, the details edit
// and the single-character mode's id.
type CharacterStore interface {
	charactersvc.Reader
	detailssvc.DAL
	FirstID(ctx context.Context) (int64, error)
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

// Storage is every storage port the root wires into the services (ADR 0025
// step 6.1: the services, this root and the router are tested on mocks of
// these).
type Storage struct {
	Store        Store
	Character    CharacterStore
	HP           hpsvc.DAL
	Log          actionlogsvc.DAL
	Buff         BuffStore
	Spell        SpellStore
	Weapons      combatsvc.Weapons
	WeaponEditor weaponsvc.DAL
	Crit         combatsvc.CritStore
	Items        itemsvc.DAL
	Forms        combatsvc.FormStore
	XP           xpsvc.DAL
	Skill        skillsvc.DAL
	Dice         dicesvc.DAL
}

// sqliteStorage wires the SQLite adapters over db.
func sqliteStorage(db *sqlite.DB) Storage {
	return Storage{
		Store:        db,
		Character:    characterdal.New(db),
		HP:           hpdal.New(db),
		Log:          actionlogdal.New(db),
		Buff:         buffdal.New(db),
		Spell:        spelldal.New(db),
		Weapons:      weapondal.New(db),
		WeaponEditor: weapondal.NewEditor(db),
		Crit:         critdal.New(db),
		Items:        itemdal.New(db),
		Forms:        attackformdal.New(db),
		XP:           xpdal.New(db),
		Skill:        skilldal.New(db),
		Dice:         dicedal.New(db),
	}
}
