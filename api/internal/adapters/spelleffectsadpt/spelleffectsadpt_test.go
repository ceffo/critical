package spelleffectsadpt_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/spelleffectsadpt"
	"github.com/ceffo/critical/api/internal/domain/service/spelleffectsvc"
)

var seedsDir = filepath.Join("..", "..", "..", "testdata", "seeds")

func loadRepo(t *testing.T) *spelleffectsadpt.Catalog {
	t.Helper()
	c, err := spelleffectsadpt.New(seedsDir)
	require.NoError(t, err)
	return c
}

func writeSeed(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, spelleffectsadpt.FileName), []byte(content), 0o600))
	return dir
}

// entry builds a one-entry seed with a utility spell plus extra JSON fields.
func entry(fields string) string {
	return `[{"id":"light","name":"Light","level":1,"kind":"utility","confidence":"high","target":"none","duration":{"kind":"instant"}` + fields + `}]`
}

func TestCatalog_StrengthEncodesRogueBonus(t *testing.T) {
	c := loadRepo(t)
	e, ok := c.Get("strength")
	require.True(t, ok)
	require.Equal(t, spelleffectsvc.KindBuff, e.Kind)
	g := e.Buff.AbilityGain["str"]
	assert.Equal(t, 18, g.Max)
	for group, want := range map[string]int{"rogue": 6, "priest": 6, "warrior": 8, "wizard": 4} {
		f := g.ByGroup[group]
		require.NotNil(t, f, group)
		hi, err := f.Max(5)
		require.NoError(t, err)
		assert.Equal(t, want, hi, group)
	}
	assert.Equal(t, "1d6", g.ByGroup["rogue"].String(), "a bard casts as a rogue: +1d6")
	rounds, err := e.Duration.Rounds.Eval(5, func(int) int { return 1 })
	require.NoError(t, err)
	assert.Equal(t, 300, rounds, "1 hour/level = 60 rounds/level")

	imp, ok := c.Get("improved_strength")
	require.True(t, ok)
	assert.Equal(t, "1d6+4", imp.Buff.AbilityGain["str"].ByGroup["rogue"].String())
	assert.Equal(t, 23, imp.Buff.AbilityGain["str"].MaxByGroup["rogue"])
}

func TestCatalog_EnlargeEncodesDamageMultiplier(t *testing.T) {
	e, ok := loadRepo(t).Get("enlarge")
	require.True(t, ok)
	require.Equal(t, spelleffectsvc.KindBuff, e.Kind)
	pct, err := e.Buff.Modifiers["damage_multiplier_pct"].Eval(6, nil)
	require.NoError(t, err)
	assert.Equal(t, 160, pct, "a 6th-level caster grows the target to 160%")
}

func TestCatalog_Formulas(t *testing.T) {
	c := loadRepo(t)
	fb, _ := c.Get("fireball")
	require.NotNil(t, fb.Damage)
	assert.Equal(t, spelleffectsvc.SaveHalf, fb.Save.Effect)
	lo, _ := fb.Damage.Dice.Min(12)
	hi, _ := fb.Damage.Dice.Max(12)
	assert.Equal(t, 10, lo, "capped at 10 dice")
	assert.Equal(t, 60, hi)

	mm, _ := c.Get("magic_missile")
	for level, want := range map[int]int{1: 1, 2: 1, 3: 2, 5: 3, 7: 4, 9: 5, 20: 5} {
		got, err := mm.Damage.Instances.Eval(level, nil)
		require.NoError(t, err)
		assert.Equal(t, want, got, "missiles at level %d", level)
	}

	ss, _ := c.Get("stoneskin")
	assert.Equal(t, spelleffectsvc.DurationUntilDischarged, ss.Duration.Kind)
	lo, _ = ss.Charges.Count.Min(9)
	hi, _ = ss.Charges.Count.Max(9)
	assert.Equal(t, 5, lo, "1d4 + 9/2")
	assert.Equal(t, 8, hi)

	mi, _ := c.Get("mirror_image")
	hi, _ = mi.Charges.Count.Max(30)
	assert.Equal(t, 8, hi, "max 8 images")

	// Cast mechanics are data: the orb spends a gem and rolls a to-hit, Rary's
	// grants 3 spell levels of slots.
	co, _ := c.Get("chromatic_orb")
	require.NotNil(t, co.ConsumeItem)
	assert.Equal(t, "gem", co.ConsumeItem.Type)
	assert.True(t, co.AttackRoll)
	rm, _ := c.Get("rarys_mnemonic_enhancer")
	require.NotNil(t, rm.GrantSlots)
	assert.Equal(t, 3, rm.GrantSlots.Levels)

	h, _ := c.Get("haste")
	d, _ := h.Duration.Rounds.Eval(7, nil)
	assert.Equal(t, 10, d, "3 rounds + 1 per level")
}

func TestParse_MalformedFormulaFailsWithClearError(t *testing.T) {
	// the original English duration is not a valid formula
	bad := `[{"id":"haste","name":"Haste","level":3,"kind":"buff","confidence":"high","target":"area",
	  "duration":{"kind":"rounds","rounds":"3 rds. + 1 rd./level"},"modifiers":{"to_hit":1}}]`
	_, err := spelleffectsadpt.New(writeSeed(t, bad))
	require.Error(t, err)
	assert.ErrorContains(t, err, `spell "haste"`)
	assert.ErrorContains(t, err, "duration.rounds")
	assert.ErrorContains(t, err, "3 rds. + 1 rd./level")
	assert.ErrorContains(t, err, spelleffectsadpt.FileName)
}

func TestParse_Errors(t *testing.T) {
	tests := map[string]struct{ seed, want string }{
		"invalid json":       {`{not json`, "parse"},
		"wrong top level":    {`{"id":"x"}`, "parse"},
		"unknown field":      {entry(`,"bogus":1`), "bogus"},
		"bad kind":           {`[{"id":"light","name":"Light","level":1,"kind":"heal","confidence":"high","target":"none","duration":{"kind":"instant"}}]`, "kind"},
		"bad level":          {`[{"id":"light","name":"Light","level":10,"kind":"utility","confidence":"high","target":"none","duration":{"kind":"instant"}}]`, "level"},
		"id not slug":        {`[{"id":"Light","name":"Light","level":1,"kind":"utility","confidence":"high","target":"none","duration":{"kind":"instant"}}]`, "slug"},
		"low without note":   {`[{"id":"light","name":"Light","level":1,"kind":"utility","confidence":"low","target":"none","duration":{"kind":"instant"}}]`, "note"},
		"bad confidence":     {`[{"id":"light","name":"Light","level":1,"kind":"utility","confidence":"maybe","target":"none","duration":{"kind":"instant"}}]`, "confidence"},
		"bad target":         {`[{"id":"light","name":"Light","level":1,"kind":"utility","confidence":"high","target":"moon","duration":{"kind":"instant"}}]`, "target"},
		"duplicate":          {`[` + entry("")[1:len(entry(""))-1] + `,` + entry("")[1:len(entry(""))-1] + `]`, "duplicate"},
		"rounds no formula":  {`[{"id":"light","name":"Light","level":1,"kind":"utility","confidence":"high","target":"none","duration":{"kind":"rounds"}}]`, "duration.rounds"},
		"formula on instant": {`[{"id":"light","name":"Light","level":1,"kind":"utility","confidence":"high","target":"none","duration":{"kind":"instant","rounds":"3"}}]`, "duration.rounds"},
		"utility with dmg":   {entry(`,"damage":{"dice":"1d6"}`), "utility"},
		"buff empty":         {`[{"id":"light","name":"Light","level":1,"kind":"buff","confidence":"high","target":"self","duration":{"kind":"instant"}}]`, "buff needs"},
		"unknown modifier":   {`[{"id":"light","name":"Light","level":1,"kind":"buff","confidence":"high","target":"self","duration":{"kind":"instant"},"modifiers":{"luck":1}}]`, "modifiers key"},
		"bad modifier":       {`[{"id":"light","name":"Light","level":1,"kind":"buff","confidence":"high","target":"self","duration":{"kind":"instant"},"modifiers":{"to_hit":"+ +"}}]`, "modifiers.to_hit"},
		"damage missing":     {`[{"id":"light","name":"Light","level":1,"kind":"damage","confidence":"high","target":"none","duration":{"kind":"instant"}}]`, "needs damage"},
		"damage both forms":  {`[{"id":"light","name":"Light","level":1,"kind":"damage","confidence":"high","target":"none","duration":{"kind":"instant"},"damage":{"dice":"1d6","by_level":[{"min_level":1,"dice":"1d4"}]}}]`, "exactly one"},
		"damage bad dice":    {`[{"id":"light","name":"Light","level":1,"kind":"damage","confidence":"high","target":"none","duration":{"kind":"instant"},"damage":{"dice":"1d6/0"}}]`, "damage.dice"},
		"by_level start":     {`[{"id":"light","name":"Light","level":1,"kind":"damage","confidence":"high","target":"none","duration":{"kind":"instant"},"damage":{"by_level":[{"min_level":3,"dice":"1d4"}]}}]`, "min_level 1"},
		"bad save":           {entry(`,"save":{"type":"luck","effect":"half"}`), "save.type"},
		"charges no unit":    {`[{"id":"light","name":"Light","level":1,"kind":"buff","confidence":"high","target":"self","duration":{"kind":"instant"},"charges":{"count":"3"}}]`, "charges.unit"},
		"bad ability":        {`[{"id":"light","name":"Light","level":1,"kind":"buff","confidence":"high","target":"self","duration":{"kind":"instant"},"ability_gain":{"luck":{"gain":"1"}}}]`, "ability_gain key"},
		"bad group":          {`[{"id":"light","name":"Light","level":1,"kind":"buff","confidence":"high","target":"self","duration":{"kind":"instant"},"ability_gain":{"str":{"by_group":{"bard":"1d6"}}}}]`, "by_group key"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := spelleffectsadpt.New(writeSeed(t, tt.seed))
			require.Error(t, err)
			assert.ErrorContains(t, err, tt.want)
		})
	}
}

func TestParse_AcceptsNumbersAsFormulas(t *testing.T) {
	seed := `[{"id":"armor","name":"Armor","level":1,"kind":"buff","confidence":"high","target":"self",
	  "duration":{"kind":"until_discharged"},"modifiers":{"ac_set":6},"charges":{"count":"8+level","unit":"hp"}}]`
	c, err := spelleffectsadpt.New(writeSeed(t, seed))
	require.NoError(t, err)
	e, ok := c.Get("armor")
	require.True(t, ok)
	assert.Equal(t, "6", e.Buff.Modifiers["ac_set"].String())
}

func TestNew_MissingFile(t *testing.T) {
	_, err := spelleffectsadpt.New(t.TempDir())
	require.ErrorContains(t, err, "spell_effects.json")
}

// damageSeed builds a one-entry seed with a damage spell, its save and its
// damage JSON.
func damageSeed(save, damage string) string {
	return `[{"id":"bolt","name":"Bolt","level":3,"kind":"damage","confidence":"high","target":"other",
	  "duration":{"kind":"instant"}` + save + `,"damage":` + damage + `}]`
}

// ADR 0008 step 13: the loader maps damage parts, each with its dice (or
// by-level table) and its save effect.
func TestParse_DamageParts(t *testing.T) {
	seed := damageSeed(`,"save":{"type":"spell","effect":"negates"}`, `{"parts":[
	  {"dice":"2d4","save":"none"},
	  {"by_level":[{"min_level":1,"dice":"1"},{"min_level":10,"dice":"level"}]}],
	  "instances":"3","type":"force"}`)
	c, err := spelleffectsadpt.New(writeSeed(t, seed))
	require.NoError(t, err)
	e, ok := c.Get("bolt")
	require.True(t, ok)
	require.Len(t, e.Damage.Parts, 2)
	assert.Nil(t, e.Damage.Dice)
	assert.Equal(t, "2d4", e.Damage.Parts[0].Dice.String())
	assert.Equal(t, spelleffectsvc.SaveNone, e.PartSave(e.Damage.Parts[0]))
	assert.Equal(t, "level", e.Damage.Parts[1].DiceAt(12).String())
	assert.Equal(t, spelleffectsvc.SaveNegates, e.PartSave(e.Damage.Parts[1]), "no save of its own: the entry's")
}

// ADR 0008 step 13: invalid damage parts are refused at startup.
func TestParse_DamagePartsRefused(t *testing.T) {
	special := `,"save":{"type":"spell","effect":"special"}`
	tests := map[string]struct{ save, damage, want string }{
		"dice and parts":         {"", `{"dice":"1d6","parts":[{"dice":"1d4"}]}`, "exactly one of dice, by_level and parts"},
		"by_level and parts":     {"", `{"by_level":[{"min_level":1,"dice":"1d4"}],"parts":[{"dice":"1d4"}]}`, "exactly one of dice, by_level and parts"},
		"empty list":             {"", `{"parts":[]}`, "damage.parts: the list is empty"},
		"part without dice":      {"", `{"parts":[{"save":"none"}]}`, "damage.parts[0]: set exactly one of dice and by_level"},
		"part with both forms":   {"", `{"parts":[{"dice":"1d4","by_level":[{"min_level":1,"dice":"1d4"}]}]}`, "damage.parts[0]: set exactly one"},
		"part dice multiplied":   {"", `{"parts":[{"dice":"1d4"},{"dice":"2*1d4"}]}`, "damage.parts[1]: expand 2*1d4"},
		"part dice malformed":    {"", `{"parts":[{"dice":"1d4/0"}]}`, "damage.parts[0].dice"},
		"part by_level start":    {"", `{"parts":[{"by_level":[{"min_level":3,"dice":"1d4"}]}]}`, "damage.parts[0].by_level must start at min_level 1"},
		"part save half":         {"", `{"parts":[{"dice":"1d4","save":"half"}]}`, "damage.parts[0]: save effect half"},
		"part save unknown":      {"", `{"parts":[{"dice":"1d4","save":"luck"}]}`, "damage.parts[0]: save luck"},
		"entry's special save":   {special, `{"parts":[{"dice":"1d4","save":"none"},{"dice":"level"}]}`, "damage.parts[1]: save effect special"},
		"part dice count rolled": {"", `{"parts":[{"dice":"1d4"},{"dice":"(1d4)d6"}]}`, "damage.parts[1]: expand (1d4)d6"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := spelleffectsadpt.New(writeSeed(t, damageSeed(tt.save, tt.damage)))
			require.Error(t, err)
			assert.ErrorContains(t, err, `spell "bolt"`)
			assert.ErrorContains(t, err, tt.want)
		})
	}
}
