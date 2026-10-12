// Package spelleffectsvc holds the spell-effect domain: what a Wizard spell
// does (buff, damage, utility), expressed as formulas of the caster level, and
// the rules an entry must satisfy. The catalog is a driven port; the seed-file
// adapter implements it. Consumers (buff application, casting) depend on
// Catalog only.
package spelleffectsvc

// Catalog is the driven port: read access to the loaded spell effects, keyed by
// spell id (see Slug).
type Catalog interface {
	// Get returns the effect for a spell id and whether it exists.
	Get(id string) (Entry, bool)
	// All returns every entry, in a stable order.
	All() []Entry
	// Len is the number of entries.
	Len() int
}
