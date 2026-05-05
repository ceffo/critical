// Package weaponsadpt is an in-memory weapons catalog adapter. Today the
// catalog is hardcoded; the adapter boundary means the service won't change
// when this is moved to a seed file or a database table.
package weaponsadpt

import "github.com/ceffo/critical/api/internal/domain/service/combatsvc"

// Catalog satisfies combatsvc.Weapons.
type Catalog struct {
	weapons map[string]combatsvc.Weapon
}

// New returns a Catalog seeded with the default weapons.
func New() *Catalog {
	return &Catalog{
		weapons: map[string]combatsvc.Weapon{
			"longsword": {ID: "longsword", Name: "Longsword", BaseDamage: "1d8"},
			"shortbow":  {ID: "shortbow", Name: "Shortbow", BaseDamage: "1d6"},
		},
	}
}

// Get returns the weapon by id and a presence flag.
func (c *Catalog) Get(id string) (combatsvc.Weapon, bool) {
	w, ok := c.weapons[id]
	return w, ok
}
