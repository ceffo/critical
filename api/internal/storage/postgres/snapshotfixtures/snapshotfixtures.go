// Package snapshotfixtures holds one stored snapshot per old encoding of the action log (the nine
// services' snapshot files of before the codec and the AC override) with the envelope the
// migration data step re-encodes it into (ADR 0018 step 9). storage/postgres
// TestReencodeSnapshots_EveryOldEncodingBecomesTheEnvelope proves each re-encoding, and composition
// TestUndo_EveryStateKeyIsRestoredThroughItsServicesStorage restores the same envelopes through each
// service's storage, so the two cannot drift apart. Test data only: nothing in production reads it.
package snapshotfixtures

import "fmt"

// Snapshot is one stored state of key in an old encoding, and its envelope after the data step.
// The item renamed by the conversion is renamed for Owner (OwnerRenames).
type Snapshot struct {
	Name             string
	Key              string
	Stored, Envelope string
}

// Owner is the character every fixture belongs to; OwnerRenames are its item ids the equipment
// conversion renamed (item_id_renames, ADR 0033 step 26.3).
const Owner = int64(7)

// OwnerRenames maps Owner's old item ids to the ids the conversion gave them.
var OwnerRenames = map[string]string{"dagger": "dagger_2"}

// Envelope is the re-encoded envelope of the fixture named name; it panics on an unknown name (a
// test's typo).
func Envelope(name string) string {
	for _, s := range Old {
		if s.Name == name {
			return s.Envelope
		}
	}
	panic(fmt.Sprintf("no snapshot fixture named %q", name))
}

// Old is every fixture, in the order the data step's test stores them.
var Old = []Snapshot{
	{
		"active buffs (buffsvc)", "buffs",
		`[{"id":"b1","name":"Stoneskin","charges":7,"duration":0,"effects":{},"spell_id":"stoneskin"}]`,
		`{"state":[{"id":"b1","name":"Stoneskin","charges":7,"duration":0,"effects":{},"spell_id":"stoneskin"}]}`,
	},
	{"no buff", "buffs", `[]`, `{"state":[]}`},
	{
		"saved templates (buffsvc)", "saved_buffs",
		`[{"id":1,"name":"Gem","duration":360,"charges":0,"effects":{"weapon_buff":5}}]`,
		`{"state":[{"id":1,"name":"Gem","duration":360,"charges":0,"effects":{"weapon_buff":5}}]}`,
	},
	{"round (buffsvc)", "round", `3`, `{"state":3}`},
	{"round 0 is a state", "round", `0`, `{"state":0}`},
	{"no initiative (buffsvc): null is absent", "initiative", `null`, `{"absent":true}`},
	{"initiative", "initiative", `{"d10":4,"entryId":12}`, `{"state":{"d10":4,"entryId":12}}`},
	{"initiative not yet naming its entry", "initiative", `{"d10":9}`, `{"state":{"d10":9}}`},
	{"spell slots (prepsvc)", "spell_slots", `[{"SpellID":"alarm"}]`, `{"state":[{"SpellID":"alarm"}]}`},
	{
		"realized level (levelupsvc)", "realized_level", `{"realizedLevel":15,"hpMax":85}`,
		`{"state":{"realizedLevel":15,"hpMax":85}}`,
	},
	{"skill points (skillsvc)", "skill_points", `{"points":{"climb_walls":80}}`, `{"state":{"points":{"climb_walls":80}}}`},
	{"skill armour (charactersvc)", "skill_armor", `{"armor":"none"}`, `{"state":{"armor":"none"}}`},
	{"AC override (acsvc)", "ac_override", `{"override":null}`, `{"state":{"override":null}}`},
	{
		"spellbook (learnsvc): the spell goes to the envelope", "spellbook:sleep",
		`{"spellId":"sleep","known":false,"failedAtLevel":0}`, `{"id":"sleep","state":{"known":false,"failedAtLevel":0}}`,
	},
	{
		"spellbook, a known spell", "spellbook:sleep",
		`{"spellId":"sleep","known":true,"failedAtLevel":0}`, `{"id":"sleep","state":{"known":true,"failedAtLevel":0}}`,
	},
	{
		"spellbook, a failure", "spellbook:light", `{"spellId":"light","known":false,"failedAtLevel":4}`,
		`{"id":"light","state":{"known":false,"failedAtLevel":4}}`,
	},
	{
		"XP entry added (xpsvc): absent, with its zeroed fields", "xp:12",
		`{"id":12,"absent":true,"date":"","session":"","reason":"","base":0,"bonus":false}`, `{"id":"12","absent":true}`,
	},
	{
		"XP entry", "xp:3", `{"id":3,"date":"2026-10-18","session":"S1","reason":"troll","base":2000,"bonus":true}`,
		`{"id":"3","state":{"date":"2026-10-18","session":"S1","reason":"troll","base":2000,"bonus":true}}`,
	},
	{
		"weapon added before the equipment model (weaponsvc): absent", "weapons:dagger2", `{"id":"dagger2","absent":true}`,
		`{"id":"dagger2","absent":true}`,
	},
	{
		"weapon with a name saved before localisation (weaponsvc)", "weapons:longsword",
		`{"id":"longsword","position":1,"name":"Long Sword +1","damageSm":"1d8","damageL":"1d12","magicToHit":1,"magicDamage":1,` +
			`"kind":"melee","twoHanded":false,"ammoKind":"","weaponType":"long_sword"}`,
		`{"id":"longsword","state":{"position":1,"name":{"en":"Long Sword +1"},"namePartial":true,"damageSm":"1d8","damageL":"1d12",` +
			`"magicToHit":1,"magicDamage":1,"kind":"melee","twoHanded":false,"ammoKind":"","weaponType":"long_sword"}}`,
	},
	{
		"item count of the equipment model (itemsvc)", "items:wand_lightning", `[{"id":"wand_lightning","count":4,"model":2}]`,
		`{"id":"wand_lightning","state":{"count":4}}`,
	},
	{
		"item a use removed: absent", "items:wand_lightning", `[{"id":"wand_lightning","count":0,"removed":true,"model":2}]`,
		`{"id":"wand_lightning","absent":true}`,
	},
	{
		"item added by hand: absent", "items:new", `[{"id":"new","count":0,"removed":true,"absent":true,"model":2}]`,
		`{"id":"new","absent":true}`,
	},
	{
		"item edited, a localized name", "items:rope",
		`[{"id":"rope","count":1,"replace":true,"model":2,"item":{"position":2,"name":{"en":"Rope","fr":"Corde"},"powers":[],` +
			`"piece":{"category":"gear","state":"carried"}}}]`,
		`{"id":"rope","state":{"count":1,"replace":true,"item":{"position":2,"name":{"en":"Rope","fr":"Corde"},"powers":[],` +
			`"piece":{"category":"gear","state":"carried"}}}}`,
	},
	{
		"item with a name saved before localisation, before powers and the model", "items:wand_lightning",
		`[{"id":"wand_lightning","count":5,"item":{"position":4,"name":"Wand of lightning","type":"wand","behaviour":"charges"}}]`,
		`{"id":"wand_lightning","state":{"count":5,"item":{"position":4,"name":{"en":"Wand of lightning"},"namePartial":true,` +
			`"type":"wand","behaviour":"charges"}}}`,
	},
	{
		"item count before the model, renamed by the conversion (ADR 0033 step 26.3)", "items:dagger",
		`[{"id":"dagger","count":3}]`, `{"id":"dagger_2","state":{"count":3}}`,
	},
	{
		"item count of the model: never renamed", "items:dagger", `[{"id":"dagger","count":3,"model":2}]`,
		`{"id":"dagger","state":{"count":3}}`,
	},
}
