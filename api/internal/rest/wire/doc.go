// Package wire holds the wire shapes (DTOs) that more than one handler
// package serves, defined once: the spell, its slots, the item, the buff
// effects, the character details and the pending levels. Handler packages
// share shapes through it, the domain's wire types (combatsvc's attack and
// save breakdowns, castsvc's cast breakdown, spelleffectsvc's damage at a
// caster level) or localized.Text, and never import one another (depguard,
// api/.golangci.yml).
package wire
