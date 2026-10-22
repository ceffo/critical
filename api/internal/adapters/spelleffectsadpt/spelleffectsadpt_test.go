package spelleffectsadpt_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/spelleffectsadpt"
	"github.com/ceffo/critical/api/internal/domain/catalogue"
	"github.com/ceffo/critical/api/internal/domain/service/spelleffectsvc"
)

var seedsDir = filepath.Join("..", "..", "..", "testdata", "seeds")

func loadRepo(t *testing.T) *spelleffectsadpt.Catalog {
	t.Helper()
	c, err := spelleffectsadpt.New(seedsDir, testGroups)
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
	_, err := spelleffectsadpt.New(writeSeed(t, bad), testGroups)
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
			_, err := spelleffectsadpt.New(writeSeed(t, tt.seed), testGroups)
			require.Error(t, err)
			assert.ErrorContains(t, err, tt.want)
		})
	}
}

func TestParse_AcceptsNumbersAsFormulas(t *testing.T) {
	seed := `[{"id":"armor","name":"Armor","level":1,"kind":"buff","confidence":"high","target":"self",
	  "duration":{"kind":"until_discharged"},"modifiers":{"ac_set":6},"charges":{"count":"8+level","unit":"hp"}}]`
	c, err := spelleffectsadpt.New(writeSeed(t, seed), testGroups)
	require.NoError(t, err)
	e, ok := c.Get("armor")
	require.True(t, ok)
	assert.Equal(t, "6", e.Buff.Modifiers["ac_set"].String())
}

func TestNew_MissingFile(t *testing.T) {
	_, err := spelleffectsadpt.New(t.TempDir(), testGroups)
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
	c, err := spelleffectsadpt.New(writeSeed(t, seed), testGroups)
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
			_, err := spelleffectsadpt.New(writeSeed(t, damageSeed(tt.save, tt.damage)), testGroups)
			require.Error(t, err)
			assert.ErrorContains(t, err, `spell "bolt"`)
			assert.ErrorContains(t, err, tt.want)
		})
	}
}

// modesSeed builds a one-entry seed of kind kind with its save and the rest
// of its fields (modes, damage) as JSON.
func modesSeed(kind, save, rest string) string {
	return `[{"id":"bolt","name":"Bolt","level":3,"kind":"` + kind + `","confidence":"high","target":"area",
	  "duration":{"kind":"instant"}` + save + `,` + rest + `}]`
}

// ADR 0008 step 14: the loader maps cast-time modes in the data's order, each
// with its damage (parts allowed) or none.
func TestParse_Modes(t *testing.T) {
	seed := modesSeed("damage", `,"save":{"type":"spell","effect":"negates"}`, `"modes":[
	  {"key":"plane"},
	  {"key":"sheet","damage":{"dice":"3d10","instances":"2","type":"cold"}},
	  {"key":"burst","damage":{"parts":[{"dice":"2d4","save":"none"},{"dice":"level"}]}}]`)
	c, err := spelleffectsadpt.New(writeSeed(t, seed), testGroups)
	require.NoError(t, err)
	e, ok := c.Get("bolt")
	require.True(t, ok)
	assert.Nil(t, e.Damage)
	require.Len(t, e.Modes, 3)
	assert.Equal(t, "plane", e.Modes[0].Key)
	assert.Nil(t, e.Modes[0].Damage)
	assert.Equal(t, "sheet", e.Modes[1].Key)
	assert.Equal(t, "3d10", e.Modes[1].Damage.Dice.String())
	assert.Equal(t, "2", e.Modes[1].Damage.Instances.String())
	assert.Equal(t, "cold", e.Modes[1].Damage.Type)
	require.Len(t, e.Modes[2].Damage.Parts, 2)
	assert.Equal(t, spelleffectsvc.SaveNegates, e.PartSave(e.Modes[2].Damage.Parts[1]), "the entry's save")
}

// ADR 0008 step 14: an invalid mode list is refused at startup.
func TestParse_ModesRefused(t *testing.T) {
	sheet := `{"key":"sheet","damage":{"dice":"3d10"}}`
	special := `,"save":{"type":"spell","effect":"special"}`
	tests := map[string]struct{ kind, save, rest, want string }{
		"empty list":          {"damage", "", `"modes":[]`, "modes: at least two are needed, got 0"},
		"one mode":            {"damage", "", `"modes":[` + sheet + `]`, "modes: at least two are needed, got 1"},
		"duplicate key":       {"damage", "", `"modes":[` + sheet + `,` + sheet + `]`, `modes[1]: duplicate key "sheet"`},
		"key not snake_case":  {"damage", "", `"modes":[{"key":"Ice Plane"},` + sheet + `]`, `modes[0]: key "Ice Plane" is not snake_case`},
		"empty key":           {"damage", "", `"modes":[{"key":""},` + sheet + `]`, `modes[0]: key "" is not snake_case`},
		"no mode with damage": {"damage", "", `"modes":[{"key":"a"},{"key":"b"}]`, "modes: no mode has damage"},
		"damage and modes":    {"damage", "", `"damage":{"dice":"1d6"},"modes":[{"key":"a"},` + sheet + `]`, "set damage or modes, not both"},
		"not a damage spell":  {"utility", "", `"modes":[{"key":"a"},` + sheet + `]`, "only a damage spell can have modes"},
		"mode dice multiplied": {
			"damage", "", `"modes":[{"key":"a"},{"key":"b","damage":{"dice":"2*1d4"}}]`,
			"modes[1].damage: expand 2*1d4",
		},
		"mode dice malformed": {"damage", "", `"modes":[{"key":"a"},{"key":"b","damage":{"dice":"1d4/0"}}]`, "modes[1].damage.dice"},
		"mode without dice":   {"damage", "", `"modes":[{"key":"a"},{"key":"b","damage":{"type":"fire"}}]`, "modes[1].damage: set exactly one of dice, by_level and parts"},
		"mode part save":      {"damage", special, `"modes":[{"key":"a"},{"key":"b","damage":{"parts":[{"dice":"1d4"}]}}]`, "modes[1].damage.parts[0]: save effect special"},
		"mode unknown field":  {"damage", "", `"modes":[{"key":"a","dice":"1d4"},` + sheet + `]`, "unknown field"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := spelleffectsadpt.New(writeSeed(t, modesSeed(tt.kind, tt.save, tt.rest)), testGroups)
			require.Error(t, err)
			if name != "mode unknown field" {
				assert.ErrorContains(t, err, `spell "bolt"`)
			}
			assert.ErrorContains(t, err, tt.want)
		})
	}
}

// ADR 0008 step 15: the loader maps dice steps, one formula for every step
// (with its count) or a list, the step variable allowed in their formulas;
// a mode's damage may have steps too.
func TestParse_DamageSteps(t *testing.T) {
	seed := damageSeed(`,"save":{"type":"spell","effect":"half"}`, `{"steps":{
	  "per":"instance","dice":"(max(min(level,12)-(step-1),0))d6","count":"level"},"type":"electricity"}`)
	c, err := spelleffectsadpt.New(writeSeed(t, seed), testGroups)
	require.NoError(t, err)
	e, ok := c.Get("bolt")
	require.True(t, ok)
	st := e.Damage.Steps
	require.NotNil(t, st)
	assert.Nil(t, e.Damage.Dice)
	assert.Equal(t, spelleffectsvc.StepPerInstance, st.Per)
	assert.Equal(t, "(max(min(level,12)-(step-1),0))d6", st.Dice.String())
	assert.Equal(t, "level", st.Count.String())
	assert.Nil(t, st.List)

	seed = damageSeed("", `{"steps":{"per":"round","list":[0,"0","leveld2"]},"type":"fire"}`)
	c, err = spelleffectsadpt.New(writeSeed(t, seed), testGroups)
	require.NoError(t, err)
	e, _ = c.Get("bolt")
	require.Len(t, e.Damage.Steps.List, 3)
	assert.Equal(t, "0", e.Damage.Steps.List[0].String(), "a bare number is a formula")
	assert.Equal(t, "leveld2", e.Damage.Steps.List[2].String())

	seed = modesSeed("damage", "", `"modes":[{"key":"a","damage":{"dice":"1d6"}},
	  {"key":"b","damage":{"steps":{"per":"round","list":["2d6","1d6"]}}}]`)
	c, err = spelleffectsadpt.New(writeSeed(t, seed), testGroups)
	require.NoError(t, err)
	e, _ = c.Get("bolt")
	require.NotNil(t, e.Modes[1].Damage.Steps)
}

// ADR 0008 step 15: invalid dice steps are refused at startup.
func TestParse_DamageStepsRefused(t *testing.T) {
	tests := map[string]struct{ damage, want string }{
		"dice and steps":        {`{"dice":"1d6","steps":{"per":"round","list":["1d6"]}}`, "exactly one of dice, by_level and parts (or steps)"},
		"parts and steps":       {`{"parts":[{"dice":"1d4"}],"steps":{"per":"round","list":["1d6"]}}`, "exactly one of dice, by_level and parts (or steps)"},
		"steps and rounds":      {`{"steps":{"per":"round","list":["1d6"]},"rounds":"3"}`, "steps replace instances and rounds"},
		"steps and instances":   {`{"steps":{"per":"instance","list":["1d6"]},"instances":"3"}`, "steps replace instances and rounds"},
		"unknown per":           {`{"steps":{"per":"turn","list":["1d6"]}}`, "damage.steps: per turn is not one of"},
		"no per":                {`{"steps":{"list":["1d6"]}}`, "damage.steps: per  is not one of"},
		"dice and list":         {`{"steps":{"per":"round","dice":"1d6","count":"2","list":["1d6"]}}`, "damage.steps: set exactly one of dice and list"},
		"neither dice nor list": {`{"steps":{"per":"round"}}`, "damage.steps: set exactly one of dice and list"},
		"dice without count":    {`{"steps":{"per":"round","dice":"1d6"}}`, "damage.steps: dice needs a count"},
		"list with a count":     {`{"steps":{"per":"round","list":["1d6"],"count":"2"}}`, "damage.steps: a list has no count"},
		"empty list":            {`{"steps":{"per":"round","list":[]}}`, "damage.steps.list: the list is empty"},
		"no damage at all":      {`{"steps":{"per":"round","list":["0","0"]}}`, "damage.steps: no step has damage at caster level 1"},
		"no damage at a level":  {`{"steps":{"per":"round","dice":"(max(level-3,0))d6","count":"2"}}`, "no step has damage at caster level 1"},
		"step dice multiplied":  {`{"steps":{"per":"round","list":["1d6","2*1d6"]}}`, "damage.steps: step 2: expand 2*1d6"},
		"dice count rolled":     {`{"steps":{"per":"round","dice":"(1d4)d6","count":"2"}}`, "expand (1d4)d6"},
		"dice count negative":   {`{"steps":{"per":"round","dice":"(12-2*step)d4","count":"6"}}`, "damage.steps.dice: formula"},
		"count rolled":          {`{"steps":{"per":"round","dice":"1d6","count":"1d4"}}`, "steps.count must be a number from 1 to 100"},
		"count zero":            {`{"steps":{"per":"round","dice":"1d6","count":"level-1"}}`, "steps.count must be a number from 1 to 100 at caster level 1"},
		"count too high":        {`{"steps":{"per":"round","dice":"1d6","count":"101"}}`, "steps.count must be a number from 1 to 100"},
		"count uses step":       {`{"steps":{"per":"round","dice":"1d6","count":"step"}}`, "damage.steps.count: formula"},
		"step outside steps":    {`{"dice":"(step)d6"}`, "step is only allowed in a dice step"},
		"rounds use step":       {`{"dice":"1d6","rounds":"step"}`, "step is only allowed in a dice step"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := spelleffectsadpt.New(writeSeed(t, damageSeed("", tt.damage)), testGroups)
			require.Error(t, err)
			assert.ErrorContains(t, err, `spell "bolt"`)
			assert.ErrorContains(t, err, tt.want)
		})
	}
}

// ADR 0008 step 16: the loader maps an exposure, one damage (steps per
// round allowed) or modes each with its damage, on a damage spell that may
// have no damage of its own.
func TestParse_Exposure(t *testing.T) {
	seed := `[` +
		`{"id":"fog","name":"Fog","level":6,"kind":"damage","confidence":"high","target":"area","duration":{"kind":"instant"},
		  "exposure":{"damage":{"steps":{"per":"round","list":["1","2","4","8"]},"type":"acid"}}},` +
		`{"id":"wall","name":"Wall","level":4,"kind":"damage","confidence":"high","target":"area","duration":{"kind":"instant"},
		  "exposure":{"modes":[{"key":"near","damage":{"dice":"2d4","type":"fire"}},{"key":"through","damage":{"dice":"2d6+level","type":"fire"}}]}},` +
		`{"id":"cloud","name":"Cloud","level":5,"kind":"damage","confidence":"high","target":"area","duration":{"kind":"instant"},
		  "damage":{"dice":"1d4"},"exposure":{"damage":{"parts":[{"dice":"1d10"}],"type":"poison"}}}]`
	c, err := spelleffectsadpt.New(writeSeed(t, seed), testGroups)
	require.NoError(t, err)

	fog, ok := c.Get("fog")
	require.True(t, ok)
	assert.Nil(t, fog.Damage, "no cast damage")
	require.NotNil(t, fog.Exposure)
	require.NotNil(t, fog.Exposure.Damage.Steps)
	assert.Equal(t, spelleffectsvc.StepPerRound, fog.Exposure.Damage.Steps.Per)
	assert.Len(t, fog.Exposure.Damage.Steps.List, 4)
	assert.Equal(t, "acid", fog.Exposure.Damage.Type)

	wall, ok := c.Get("wall")
	require.True(t, ok)
	require.Len(t, wall.Exposure.Modes, 2)
	assert.Equal(t, "near", wall.Exposure.Modes[0].Key)
	assert.Equal(t, "2d4", wall.Exposure.Modes[0].Damage.Dice.String())
	assert.Equal(t, "through", wall.Exposure.Modes[1].Key)
	assert.Equal(t, "2d6+level", wall.Exposure.Modes[1].Damage.Dice.String())
	assert.Nil(t, wall.Exposure.Damage)

	cloud, ok := c.Get("cloud")
	require.True(t, ok)
	assert.Equal(t, "1d4", cloud.Damage.Dice.String(), "a cast damage besides the exposure")
	require.Len(t, cloud.Exposure.Damage.Parts, 1)
}

// ADR 0008 step 16: an exposure that is not one roll per round of exposure,
// has both or neither of damage and modes, modes that break the rules of
// step 14 or lack damage, or sits on a spell that is not a damage spell is a
// startup error naming the spell and the field.
func TestParse_ExposureRefused(t *testing.T) {
	near := `{"key":"near","damage":{"dice":"2d4"}}`
	tests := map[string]struct{ kind, rest, want string }{
		"not a damage spell":     {"utility", `"exposure":{"damage":{"dice":"1d10"}}`, "only a damage spell can have an exposure"},
		"neither":                {"damage", `"exposure":{}`, "exposure: set exactly one of damage and modes"},
		"both":                   {"damage", `"exposure":{"damage":{"dice":"1d10"},"modes":[` + near + `,{"key":"far","damage":{"dice":"1d4"}}]}`, "exposure: set exactly one of damage and modes"},
		"instances":              {"damage", `"exposure":{"damage":{"dice":"1d10","instances":"2"}}`, "exposure.damage: one roll is one round of exposure"},
		"rounds":                 {"damage", `"exposure":{"damage":{"dice":"1d10","rounds":"level"}}`, "exposure.damage: one roll is one round of exposure"},
		"steps per instance":     {"damage", `"exposure":{"damage":{"steps":{"per":"instance","list":["1","2"]}}}`, `exposure.damage.steps: the steps of an exposure are its rounds`},
		"dice not added":         {"damage", `"exposure":{"damage":{"dice":"2*1d4"}}`, "exposure.damage: expand 2*1d4"},
		"no dice":                {"damage", `"exposure":{"damage":{"type":"fire"}}`, "exposure.damage: set exactly one of dice, by_level and parts"},
		"one mode":               {"damage", `"exposure":{"modes":[` + near + `]}`, "exposure.modes: at least two are needed, got 1"},
		"empty modes":            {"damage", `"exposure":{"modes":[]}`, "exposure.modes: at least two are needed, got 0"},
		"duplicate key":          {"damage", `"exposure":{"modes":[` + near + `,` + near + `]}`, `exposure.modes[1]: duplicate key "near"`},
		"key not snake_case":     {"damage", `"exposure":{"modes":[{"key":"Near","damage":{"dice":"1d4"}},` + near + `]}`, `exposure.modes[0]: key "Near" is not snake_case`},
		"mode without damage":    {"damage", `"exposure":{"modes":[{"key":"far"},` + near + `]}`, "exposure.modes[0]: an exposure mode needs damage"},
		"mode with instances":    {"damage", `"exposure":{"modes":[{"key":"far","damage":{"dice":"1d4","instances":"3"}},` + near + `]}`, "exposure.modes[0].damage: one roll is one round of exposure"},
		"no damage, no exposure": {"damage", `"note":"x"`, "a damage spell needs damage, modes or an exposure"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := spelleffectsadpt.New(writeSeed(t, modesSeed(tt.kind, "", tt.rest)), testGroups)
			require.Error(t, err)
			assert.ErrorContains(t, err, `spell "bolt"`)
			assert.ErrorContains(t, err, tt.want)
		})
	}
}

func TestCheckAgainst_ListsEveryIDTheCatalogueDoesNotHold(t *testing.T) {
	c, err := spelleffectsadpt.Parse([]byte(`[
	  {"id":"light","name":"Light","level":1,"kind":"utility","confidence":"high","target":"none","duration":{"kind":"instant"}},
	  {"id":"sleep","name":"Sleep","level":1,"kind":"utility","confidence":"high","target":"none","duration":{"kind":"instant"}},
	  {"id":"lihgt","name":"Lihgt","level":1,"kind":"utility","confidence":"high","target":"none","duration":{"kind":"instant"}}]`), testGroups)
	require.NoError(t, err)

	held := catalogue.Catalogue{Spells: []catalogue.Spell{{ID: "light"}, {ID: "sleep"}, {ID: "lihgt"}}}
	require.NoError(t, c.CheckAgainst(held))

	err = c.CheckAgainst(catalogue.Catalogue{Spells: []catalogue.Spell{{ID: "light"}}})
	require.Error(t, err)
	assert.EqualError(t, err, "spell effects: ids not in the spell catalogue: sleep, lihgt")
}

// ADR 0004 step 21: the loader maps a spell's end effects, each with its
// kind, its dice and its causes, in the data's order.
func TestBuffs_Step21_ParseEndEffects(t *testing.T) {
	c, err := spelleffectsadpt.New(writeSeed(t, entry(`,"end_effects":[{"kind":"heal","dice":"1d12","on":["voluntary","expired"]}]`)), testGroups)
	require.NoError(t, err)
	e, ok := c.Get("light")
	require.True(t, ok)
	require.Len(t, e.EndEffects, 1)
	assert.Equal(t, spelleffectsvc.EndHeal, e.EndEffects[0].Kind)
	assert.Equal(t, "1d12", e.EndEffects[0].Dice.String())
	assert.Equal(t, []spelleffectsvc.EndCause{spelleffectsvc.EndVoluntary, spelleffectsvc.EndExpired}, e.EndEffects[0].On)
	assert.True(t, e.AsksEndCause())
}

// ADR 0004 step 21: an unknown kind or cause, an empty or repeated cause
// list, bad dice and dice that use the level are refused at startup.
func TestBuffs_Step21_ParseEndEffectsRefused(t *testing.T) {
	tests := map[string]struct{ effects, want string }{
		"unknown kind":    {`[{"kind":"damage","dice":"1d6","on":["voluntary"]}]`, "end_effects[0].kind"},
		"unknown cause":   {`[{"kind":"heal","dice":"1d6","on":["slain"]}]`, "end_effects[0].on"},
		"empty causes":    {`[{"kind":"heal","dice":"1d6","on":[]}]`, "at least one cause"},
		"no causes":       {`[{"kind":"heal","dice":"1d6"}]`, "at least one cause"},
		"duplicate cause": {`[{"kind":"heal","dice":"1d6","on":["expired","expired"]}]`, "expired twice"},
		"malformed dice":  {`[{"kind":"heal","dice":"1d","on":["voluntary"]}]`, "end_effects[0].dice"},
		"missing dice":    {`[{"kind":"heal","on":["voluntary"]}]`, "end_effects[0].dice"},
		"dice not a sum":  {`[{"kind":"heal","dice":"2*1d6","on":["voluntary"]}]`, "not a sum of dice"},
		"dice use level":  {`[{"kind":"heal","dice":"leveld4","on":["voluntary"]}]`, "may not use level"},
		"empty list":      {`[]`, "end_effects: empty list"},
		"unknown field":   {`[{"kind":"heal","dice":"1d6","on":["voluntary"],"when":"x"}]`, "unknown field"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := spelleffectsadpt.New(writeSeed(t, entry(`,"end_effects":`+tt.effects)), testGroups)
			require.Error(t, err)
			assert.ErrorContains(t, err, tt.want)
		})
	}
}
