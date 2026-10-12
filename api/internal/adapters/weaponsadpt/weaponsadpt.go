// Package weaponsadpt is a weapons catalog adapter backed by
// <seedsDir>/weapons.json, loaded once at startup.
package weaponsadpt

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
)

// FileName is the catalog file looked up inside the seeds directory.
const FileName = "weapons.json"

// Catalog satisfies combatsvc.Weapons.
type Catalog struct {
	weapons map[string]combatsvc.Weapon
}

type weaponJSON struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	BaseDamage string `json:"base_damage"`
}

// New loads <seedsDir>/weapons.json. A missing or invalid file is an error.
func New(seedsDir string) (*Catalog, error) {
	path := filepath.Join(seedsDir, FileName)
	raw, err := os.ReadFile(path) //nolint:gosec // path is built from the operator-configured seeds dir
	if err != nil {
		return nil, fmt.Errorf("weapons catalog: read %s: %w", path, err)
	}
	var entries []weaponJSON
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, fmt.Errorf("weapons catalog: parse %s: %w", path, err)
	}
	weapons := make(map[string]combatsvc.Weapon, len(entries))
	for i, e := range entries {
		if e.ID == "" || e.Name == "" || e.BaseDamage == "" {
			return nil, fmt.Errorf("weapons catalog: %s entry %d: id, name and base_damage are required", path, i)
		}
		if _, dup := weapons[e.ID]; dup {
			return nil, fmt.Errorf("weapons catalog: %s: duplicate id %q", path, e.ID)
		}
		weapons[e.ID] = combatsvc.Weapon{ID: e.ID, Name: e.Name, BaseDamage: e.BaseDamage}
	}
	return &Catalog{weapons: weapons}, nil
}

// Get returns the weapon by id and a presence flag.
func (c *Catalog) Get(id string) (combatsvc.Weapon, bool) {
	w, ok := c.weapons[id]
	return w, ok
}
