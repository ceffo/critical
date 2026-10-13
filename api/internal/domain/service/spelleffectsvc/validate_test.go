package spelleffectsvc_test

import (
	"testing"

	"github.com/ceffo/critical/api/internal/domain/service/spelleffectsvc"
	"github.com/ceffo/critical/api/pkg/formula"
	"github.com/ceffo/critical/api/pkg/must"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func utility() spelleffectsvc.Entry {
	return spelleffectsvc.Entry{
		ID: "light", Name: "Light", Level: 1, Kind: spelleffectsvc.KindUtility,
		Confidence: spelleffectsvc.ConfidenceHigh, Target: spelleffectsvc.TargetNone,
		Duration: spelleffectsvc.Duration{Kind: spelleffectsvc.DurationInstant},
	}
}

func TestSlug(t *testing.T) {
	tests := map[string]string{
		"Magic Missile":                      "magic_missile",
		"Melf's Acid Arrow":                  "melfs_acid_arrow",
		"Cat’s Grace":                        "cats_grace",
		"Hornung's Guess*":                   "hornungs_guess",
		"Darkness, 15' Radius":               "darkness_15_radius",
		"Protection From Evil, 10' Radius":   "protection_from_evil_10_radius",
		"Detect Secret Passages and Portals": "detect_secret_passages_and_portals",
		"There/Not There*":                   "there_not_there",
	}
	for name, want := range tests {
		assert.Equal(t, want, spelleffectsvc.Slug(name), name)
	}
}

func TestValidate(t *testing.T) {
	f := must.Do(formula.Parse("1d6"))
	tests := map[string]struct {
		mut  func(*spelleffectsvc.Entry)
		want string // empty = valid
	}{
		"valid utility":      {func(*spelleffectsvc.Entry) {}, ""},
		"bad kind":           {func(e *spelleffectsvc.Entry) { e.Kind = "heal" }, "kind"},
		"bad level":          {func(e *spelleffectsvc.Entry) { e.Level = 10 }, "level"},
		"id not slug":        {func(e *spelleffectsvc.Entry) { e.ID = "Light" }, "slug"},
		"low without note":   {func(e *spelleffectsvc.Entry) { e.Confidence = spelleffectsvc.ConfidenceLow }, "note"},
		"bad confidence":     {func(e *spelleffectsvc.Entry) { e.Confidence = "maybe" }, "confidence"},
		"bad target":         {func(e *spelleffectsvc.Entry) { e.Target = "moon" }, "target"},
		"rounds no formula":  {func(e *spelleffectsvc.Entry) { e.Duration.Kind = spelleffectsvc.DurationRounds }, "duration.rounds"},
		"formula on instant": {func(e *spelleffectsvc.Entry) { e.Duration.Rounds = f }, "duration.rounds"},
		"utility with dmg":   {func(e *spelleffectsvc.Entry) { e.Damage = &spelleffectsvc.Damage{Dice: f} }, "utility"},
		"buff empty":         {func(e *spelleffectsvc.Entry) { e.Kind = spelleffectsvc.KindBuff }, "buff needs"},
		"unknown modifier": {func(e *spelleffectsvc.Entry) {
			e.Kind = spelleffectsvc.KindBuff
			e.Buff = &spelleffectsvc.Buff{Modifiers: map[string]*formula.Formula{"luck": f}}
		}, "modifiers key"},
		"damage missing": {func(e *spelleffectsvc.Entry) { e.Kind = spelleffectsvc.KindDamage }, "needs damage"},
		"damage both forms": {func(e *spelleffectsvc.Entry) {
			e.Kind = spelleffectsvc.KindDamage
			e.Damage = &spelleffectsvc.Damage{Dice: f, ByLevel: []spelleffectsvc.LevelDice{{MinLevel: 1, Dice: f}}}
		}, "exactly one"},
		"by_level start": {func(e *spelleffectsvc.Entry) {
			e.Kind = spelleffectsvc.KindDamage
			e.Damage = &spelleffectsvc.Damage{ByLevel: []spelleffectsvc.LevelDice{{MinLevel: 3, Dice: f}}}
		}, "min_level 1"},
		"bad save": {func(e *spelleffectsvc.Entry) { e.Save = &spelleffectsvc.Save{Type: "luck", Effect: "half"} }, "save.type"},
		"charges no unit": {func(e *spelleffectsvc.Entry) {
			e.Kind = spelleffectsvc.KindBuff
			e.Charges = &spelleffectsvc.Charges{Count: f}
		}, "charges.unit"},
		"bad ability": {func(e *spelleffectsvc.Entry) {
			e.Kind = spelleffectsvc.KindBuff
			e.Buff = &spelleffectsvc.Buff{AbilityGain: map[string]spelleffectsvc.AbilityGain{"luck": {Gain: f}}}
		}, "ability_gain key"},
		"bad group": {func(e *spelleffectsvc.Entry) {
			e.Kind = spelleffectsvc.KindBuff
			e.Buff = &spelleffectsvc.Buff{AbilityGain: map[string]spelleffectsvc.AbilityGain{"str": {ByGroup: map[string]*formula.Formula{"bard": f}}}}
		}, "by_group key"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			e := utility()
			tt.mut(&e)
			err := e.Validate()
			if tt.want == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tt.want)
		})
	}
}

func TestLevelSlug(t *testing.T) {
	assert.Equal(t, "fortify_l4", spelleffectsvc.LevelSlug("Fortify", 4))
	assert.Equal(t, "cats_grace_l2", spelleffectsvc.LevelSlug("Cat’s Grace", 2))
	assert.Equal(t, "fortify", spelleffectsvc.Slug("Fortify"), "the plain slug is unchanged")
}
