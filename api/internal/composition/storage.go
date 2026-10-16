package composition

import (
	"context"

	"github.com/ceffo/critical/api/internal/adapters/actionlogdal"
	"github.com/ceffo/critical/api/internal/adapters/attackformdal"
	formq "github.com/ceffo/critical/api/internal/adapters/attackformdal/queries"
	"github.com/ceffo/critical/api/internal/adapters/buffdal"
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
	"github.com/ceffo/critical/api/internal/adapters/spelldal"
	"github.com/ceffo/critical/api/internal/adapters/textdal"
	textq "github.com/ceffo/critical/api/internal/adapters/textdal/queries"
	"github.com/ceffo/critical/api/internal/adapters/weapondal"
	weaponq "github.com/ceffo/critical/api/internal/adapters/weapondal/queries"
	"github.com/ceffo/critical/api/internal/adapters/xpdal"
	xpq "github.com/ceffo/critical/api/internal/adapters/xpdal/queries"
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

// sqliteStorage wires the SQLite adapters over db. The sqlc-generated queries
// run on the transaction the context carries, else on the pool (ADR 0025
// step 5.2).
func sqliteStorage(db *sqlite.DB) Storage {
	routed := sqlite.Routed(db)
	// Item and weapon names are localized texts (ADR 0023 step 9).
	texts := textdal.New(textq.New(routed))
	return Storage{
		Store:        db,
		Character:    characterdal.New(charq.New(routed)),
		HP:           hpdal.New(hpq.New(routed)),
		Log:          actionlogdal.New(db),
		Buff:         buffdal.New(db),
		Spell:        spelldal.New(db),
		Weapons:      weapondal.New(weaponq.New(routed), texts),
		WeaponEditor: weapondal.NewEditor(weaponq.New(routed), texts),
		Crit:         critdal.New(critq.New(routed)),
		Items:        itemdal.New(itemq.New(routed), texts),
		Forms:        attackformdal.New(formq.New(routed)),
		XP:           xpdal.New(xpq.New(routed)),
		Skill:        skilldal.New(skillq.New(routed)),
		Dice:         dicedal.New(diceq.New(routed)),
	}
}
