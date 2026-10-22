package spelleffectsadpt_test

import (
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/spelleffectsadpt"
	"github.com/ceffo/critical/api/internal/domain/service/spelleffectsvc"
	"github.com/ceffo/critical/api/internal/domain/spellstats"
	"github.com/ceffo/critical/api/pkg/formula"
)

func repoEffects(t *testing.T) *spelleffectsadpt.Catalog {
	t.Helper()
	c, err := spelleffectsadpt.New(filepath.Join("..", "..", "..", "..", "seeds"), testGroups)
	require.NoError(t, err)
	return c
}

// ADR 0004 step 20.1, on the campaign's real seeds: a spell whose book gives
// a time has that time in its effect entry, so it gets a timeline row. Each
// row is the spell's stat-line duration as the catalogue parses it; the
// entry's rounds formula must be the one the catalogue's conversion gives.
func TestRepoSeeds_ADR0004Step20_TimedBookDurationsGiveARow(t *testing.T) {
	book := map[string]string{
		"alarm":                    "4 hrs. + 1/2 hr./level",
		"locate_remains":           "1 turn + 5 rounds/level",
		"spectral_voice":           "1 round/level",
		"choke":                    "1 round/level",
		"fireflow":                 "1 round/level",
		"fly":                      "1 turn/level + 1d6 turns",
		"water_breathing":          "1 hr./level + 1d4 hrs.",
		"detect_scrying":           "1d6 turns + 1 turn/level",
		"enervation":               "1d4 hrs. + 1 hr./level",
		"leomunds_secure_shelter":  "1d4+1 hrs. + 1 hr./level",
		"locate_creature":          "1 turn/level",
		"there_not_there":          "1-6 turns",
		"leomunds_hidden_lodge":    "1d4 hrs. + 1 hr./level",
		"control_weather":          "4d6 hrs.",
		"eyebite":                  "1 round/3 levels",
		"repulsion":                "1 round/2 levels",
		"superior_magnetism":       "1 rd./5 levels",
		"forcecage":                "6 turns + 1/level",
		"sequester":                "1 week + 1 day/level",
		"spell_shape":              "1d4+1 rounds",
		"spell_turning":            "Up to 3 rds./level",
		"analyze_dweomer":          "1 rd./2 levels",
		"ottos_irresistible_dance": "1d4+1 rounds",
		"wildzone":                 "2d6 turns",
		"stabilize":                "1d4+1 turns",
		"wildwind":                 "1d3 turns",
		// Priest spells, stat-line durations of the catalogue.
		"bless":                                 "6 rds.",
		"protection_from_evil_priest":           "3 rds./level",
		"protection_from_chaos":                 "3 rds./level",
		"protection_from_evil_10_radius_priest": "1 turn/level",
		"faerie_fire":                           "4 rds./level",
		"barkskin":                              "4 rds. + 1 rd./level",
		"aid":                                   "1 rd. + 1 rd./level",
		"prayer":                                "1 rd./level",
		"recitation":                            "1 rd./level",
		"righteous_wrath_of_the_faithful":       "1 rd./level",
		"magical_vestment":                      "5 rds./level",
		"unfailing_premonition":                 "1 turn + 2 rds./level",
		"hesitation":                            "1 round/level",
		"ebony_hand":                            "3 rounds + 1/level",
		"heat_metal":                            "7 rds.",
		"produce_fire":                          "1 rd.",
		"fire_storm":                            "1 rd.",
		"blade_barrier":                         "3 rds./level",
		"insect_plague":                         "2 rds./level",
	}
	c := repoEffects(t)
	for id, raw := range book {
		src, ok := spellstats.ParseDuration(raw).RoundsFormula()
		require.True(t, ok, "%s: %q converts to rounds", id, raw)
		want, err := formula.Parse(src)
		require.NoError(t, err, id)

		e, ok := c.Get(id)
		require.True(t, ok, id)
		require.Equal(t, spelleffectsvc.DurationRounds, e.Duration.Kind, "%s: duration kind", id)
		require.NotNil(t, e.Duration.Rounds, id)
		assert.Equal(t, want.String(), e.Duration.Rounds.String(), "%s: rounds (%q)", id, raw)
		assert.NotNil(t, spelleffectsvc.RowOf(&e, spellstats.Duration{}).Rounds, "%s: has a timeline row", id)
	}
}

// Every entry left "special" (it puts no row on the timeline, ADR 0004
// step 20.2) is listed here: a new one is a conscious choice, and an entry
// that gets a duration leaves the list.
func TestRepoSeeds_ADR0004Step20_SpecialEntriesAreListed(t *testing.T) {
	want := []string{
		"charm_person", "find_familiar", "nahals_reckless_dweomer", "phantasmal_force", "spook",
		"deafness", "death_recall", "glitterdust", "hypnotic_pattern", "improved_phantasmal_force",
		"invisibility", "irritation", "knock", "magic_mouth", "past_life", "pyrotechnics",
		"summon_swarm", "whispering_wind", "alamirs_fundamental_breakdown", "augmentation_i",
		"far_reaching_i", "invisibility_10_radius", "invisible_mail", "lesser_sign_of_sealing",
		"sepia_snake_sigil", "skulltrap", "spectral_force", "wall_of_water", "watery_double",
		"charm_monster", "dilation_i", "emotion", "extension_i", "far_reaching_ii", "fear",
		"lesser_geas", "massmorph", "rainbow_pattern", "summon_lycanthrope", "wall_of_fire",
		"contact_other_plane", "domination", "dream", "extension_ii", "far_reaching_iii",
		"force_shapechange", "graft_flesh", "leomunds_lamentable_belaborment", "magic_jar",
		"magic_staff", "major_creation", "mordenkainens_faithful_hound", "sending", "telekinesis",
		"transmute_rock_to_mud", "arrow_of_bone", "demishadow_magic", "dilation_ii", "enchant_an_item",
		"ensnarement", "extension_iii", "geas", "ghoul_gauntlet", "invisible_stalker", "legend_lore",
		"mirage_arcana", "otilukes_freezing_sphere", "programmed_illusion", "wildshield",
		"hatch_the_stone_from_the_egg", "hornungs_surge_selector", "limited_wish", "mass_invisibility",
		"phase_door", "power_word_stun", "vanish", "vision", "binding", "death_shroud", "defoliate",
		"demand", "heart_of_stone", "mass_charm", "maze", "polymorph_any_object", "power_word_blind",
		"sink", "symbol", "trap_the_soul", "astral_spell", "death_ward", "gate",
		"glorious_transmutation", "programmed_amnesia", "succor", "time_stop", "weird", "wildfire",
		"wish",
		// Priest spells with no time in the book: lasts while chanting or concentrating, or
		// as long as the seeds keep their power.
		"chant", "fire_seeds", "wall_of_fire_priest",
	}
	var got []string
	for _, e := range repoEffects(t).All() {
		if e.Duration.Kind == spelleffectsvc.DurationSpecial {
			got = append(got, e.ID)
		}
	}
	sort.Strings(got)
	sort.Strings(want)
	assert.Equal(t, want, got)
}
