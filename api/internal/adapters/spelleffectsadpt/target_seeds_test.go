package spelleffectsadpt_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/spelleffectsadpt"
	"github.com/ceffo/critical/api/internal/domain/service/spelleffectsvc"
)

// ADR 0027 step 10.1, on the campaign's real seeds/spell_effects.json: where
// a cast of every buff lands when the Bard names no target, and the target
// choice its cast offers (nil: a plain Cast button). Only a defensive buff
// that is not self-only offers one; a hostile buff (Blindness, Pain Touch,
// Slow, the Malisons) lands on somebody else and offers none. A buff added to
// the seeds fails the test until its row is written here.
func TestRepoSeeds_EveryBuffDefaultTargetAndChoice(t *testing.T) {
	c, err := spelleffectsadpt.New(filepath.Join("..", "..", "..", "..", "seeds"))
	require.NoError(t, err)

	onMe := &spelleffectsvc.TargetChoice{Default: spelleffectsvc.CastOnSelf, Alternate: spelleffectsvc.CastOnOther}
	onAlly := &spelleffectsvc.TargetChoice{Default: spelleffectsvc.CastOnOther, Alternate: spelleffectsvc.CastOnSelf}
	type want struct {
		onSelf bool
		choice *spelleffectsvc.TargetChoice
	}
	wants := map[string]want{
		// Self or other, defensive: on the Bard, the arrow offers an ally.
		"armor":                 {true, onMe},
		"enlarge":               {true, onMe},
		"protection_from_evil":  {true, onMe},
		"cats_grace":            {true, onMe},
		"strength":              {true, onMe},
		"improved_invisibility": {true, onMe},
		"improved_strength":     {true, onMe},
		"stoneskin":             {true, onMe},
		"dragon_scales":         {true, onMe},
		// Touch, defensive, on the party's priests: on the cleric, the arrow offers an ally.
		"protection_from_evil_priest": {true, onMe},
		"protection_from_chaos":       {true, onMe},
		"barkskin":                    {true, onMe},
		"aid":                         {true, onMe},
		// Friendly areas: on the Bard, the arrow offers not on him.
		"haste":                                 {true, onMe},
		"protection_from_evil_10_radius":        {true, onMe},
		"protection_from_evil_10_radius_priest": {true, onMe},
		"bless":                                 {true, onMe},
		"prayer":                                {true, onMe},
		"chant":                                 {true, onMe},
		"recitation":                            {true, onMe},
		"righteous_wrath_of_the_faithful":       {true, onMe},
		// Other, defensive: on an ally, the arrow offers the Bard.
		"enchanted_weapon": {false, onAlly},
		"remove_fear":      {false, onAlly},
		// Hostile: not on the Bard, no arrow.
		"blindness":       {false, nil},
		"pain_touch":      {false, nil},
		"slow":            {false, nil},
		"minor_malison":   {false, nil},
		"greater_malison": {false, nil},
		"faerie_fire":     {false, nil},
		"hesitation":      {false, nil},
		// Self-only: always on the Bard, no arrow.
		"expeditious_retreat":    {true, nil},
		"fist_of_stone":          {true, nil},
		"friends":                {true, nil},
		"shield":                 {true, nil},
		"blur":                   {true, nil},
		"mirror_image":           {true, nil},
		"spirit_armor":           {true, nil},
		"psychic_protection":     {true, nil},
		"tensers_transformation": {true, nil},
		"trollish_fortitude":     {true, nil},
		"magical_vestment":       {true, nil},
		"unfailing_premonition":  {true, nil},
		"ebony_hand":             {true, nil},
	}

	const level = 15
	seen := map[string]bool{}
	for _, e := range c.All() {
		if e.Kind != spelleffectsvc.KindBuff {
			continue
		}
		seen[e.ID] = true
		w, ok := wants[e.ID]
		if !assert.True(t, ok, "buff %s has no expected row", e.ID) {
			continue
		}
		onSelf, err := e.DefaultOnSelf(level)
		require.NoError(t, err, e.ID)
		assert.Equal(t, w.onSelf, onSelf, "%s: default on the Bard", e.ID)
		choice, err := e.TargetChoiceAt(level)
		require.NoError(t, err, e.ID)
		assert.Equal(t, w.choice, choice, "%s: target choice", e.ID)
	}
	for id := range wants {
		assert.True(t, seen[id], "%s is no longer a buff in the seeds", id)
	}
}

// ADR 0004 step 20.5 on the real seeds: a spell that adds no buff but lasts a
// number of rounds offers the target choice when its target can be someone
// else, except a damage spell: "not on you" by default for a spell cast at a
// creature or on an area, "on you" by default for one the caster may take
// himself. The lists are every such spell: a seed that adds one fails the test
// until it is written here.
func TestRepoSeeds_EveryRowOnlySpellTargetChoice(t *testing.T) {
	c, err := spelleffectsadpt.New(filepath.Join("..", "..", "..", "..", "seeds"))
	require.NoError(t, err)

	onMe := &spelleffectsvc.TargetChoice{Default: spelleffectsvc.CastOnSelf, Alternate: spelleffectsvc.CastOnOther}
	onAlly := &spelleffectsvc.TargetChoice{Default: spelleffectsvc.CastOnOther, Alternate: spelleffectsvc.CastOnSelf}
	// Creature or area at range: the row reads "not on you" by default.
	wantOnAlly := []string{
		"acid_storm", "advanced_illusion", "affect_normal_fires", "alarm", "animal_growth",
		"bands_of_sirellyn", "bind_undead", "blackmantle", "chaos", "choke", "confusion", "control_undead",
		"darkness_15_radius", "dig", "distance_distortion", "enervation", "evards_black_tentacles",
		"eyebite", "fire_charm", "fog_cloud", "forcecage", "fumble", "grease", "gust_of_wind",
		"hallucinatory_terrain", "hold_monster", "hold_person", "hold_undead", "hypnotism",
		"insatiable_thirst", "leomunds_hidden_lodge", "leomunds_secure_shelter", "light",
		"lower_resistance", "mass_suggestion", "maximilians_earthen_grasp", "maximilians_stony_grasp",
		"mind_fog", "misdirection", "mordenkainens_private_sanctum", "otilukes_dispelling_screen",
		"otilukes_resilient_sphere", "otilukes_telekinetic_sphere", "ottos_irresistible_dance", "passwall",
		"phantasmal_killer", "ray_of_enfeeblement", "ray_of_fatigue", "repulsion", "reverse_gravity",
		"scare", "sleep", "snapping_teeth", "solid_fog", "solvent_of_corrosion", "spectral_hand",
		"sphere_of_ultimate_destruction", "stabilize", "stinking_cloud", "suffocate", "suggestion",
		"tashas_uncontrollable_hideous_laughter", "taunt", "there_not_there", "throbbing_bones", "unluck",
		"vacancy", "veil", "von_gasiks_refusal", "vortex", "wall_of_fog", "wall_of_force", "wall_of_gloom",
		"waveform", "web", "wildstrike", "wildwind", "wildzone", "wind_wall",
	}
	// Self or other (a touch, a creature the caster may be): on the Bard by default.
	wantOnMe := []string{
		"claws_of_the_umber_hulk", "comprehend_languages", "corpse_visage", "delay_death", "etherealness",
		"feather_fall", "feign_death", "filter", "fly", "fools_speech", "hornungs_baneful_deflector",
		"infravision", "iron_mind", "jump", "lasting_breath", "levitate",
		"lorloveims_shadowy_transformation", "mind_blank", "nahals_nonsensical_nullifier", "nondetection",
		"protection_from_cantrips", "protection_from_hunger_and_thirst", "protection_from_normal_missiles",
		"protection_from_paralysis", "protection_from_poison", "protection_from_vermin",
		"rarys_telepathic_bond", "ride_the_wind", "seeming", "sertens_spell_immunity", "spider_climb",
		"statue", "ultravision", "vocalize", "water_breathing",
	}

	var gotOnAlly, gotOnMe []string
	for _, e := range c.All() {
		if e.AddsBuff() {
			continue
		}
		choice, err := e.TargetChoiceAt(15)
		require.NoError(t, err, e.ID)
		switch choice {
		case nil:
		default:
			if *choice == *onAlly {
				gotOnAlly = append(gotOnAlly, e.ID)
			} else {
				require.Equal(t, onMe, choice, e.ID)
				gotOnMe = append(gotOnMe, e.ID)
			}
		}
	}
	assert.ElementsMatch(t, wantOnAlly, gotOnAlly, "rows that default to somebody else")
	assert.ElementsMatch(t, wantOnMe, gotOnMe, "rows that default to the Bard")
}

// Every spell that lasts a number of rounds without adding a buff and still
// has no target, each checked against the book: it is the caster's own senses
// or aura (detections, divinations, wards and globes centred on him), an
// object, a summoned or created servant, or a spell modelled on none of those.
// A seed that adds one fails the test until the entry gets a target or is
// written here as a conscious choice.
func TestRepoSeeds_RoundLastingEntriesLeftWithNoTarget(t *testing.T) {
	c, err := spelleffectsadpt.New(filepath.Join("..", "..", "..", "..", "seeds"))
	require.NoError(t, err)

	want := []string{
		"airboat", "airy_water", "analyze_dweomer", "antipathy_sympathy", "audible_glamer",
		"bigbys_clenched_fist", "bigbys_crushing_hand", "bigbys_forceful_hand", "bigbys_grasping_hand",
		"bigbys_interposing_hand", "bind", "bloodstones_frightful_joining", "bloodstones_spectral_steed",
		"bone_club", "bone_dance", "cantrip", "clairaudience", "clairvoyance", "conjure_animals",
		"conjure_elemental", "conjure_elemental_kin", "conjure_spell_component", "control_weather",
		"corpse_host", "dancing_lights", "dead_mans_eyes", "deeppockets", "demishadow_monsters",
		"detect_disease", "detect_evil", "detect_invisibility", "detect_life", "detect_magic",
		"detect_phase", "detect_scrying", "detect_secret_passages_and_portals", "detect_undead",
		"dictation", "dimensional_blade", "divining_rod", "duplicate", "esp", "false_vision", "fireflow",
		"fools_gold", "foresight", "forests_fiery_constrictor", "gaze_reflection", "glassee",
		"globe_of_invulnerability", "guards_and_wards", "halo_of_eyes", "hold_portal", "hovering_skull",
		"identify", "illusionary_script", "intensify_summoning", "invulnerability_to_magical_weapons",
		"invulnerability_to_normal_weapons", "item", "khazids_procurement", "know_alignment",
		"leomunds_secret_chest", "leomunds_tiny_hut", "living_link", "locate_creature", "locate_object",
		"locate_remains", "lower_water", "magic_mirror", "mask_of_death", "message", "minor_creation",
		"minor_globe_of_invulnerability", "mislead", "monster_summoning_i", "monster_summoning_ii",
		"monster_summoning_iii", "monster_summoning_iv", "monster_summoning_v", "monster_summoning_vi",
		"monster_summoning_vii", "mordenkainens_magnificent_mansion", "mordenkainens_sword", "mount",
		"nystuls_magical_aura", "part_water", "patternweave", "phantom_steed", "prismatic_sphere",
		"prismatic_wall", "project_image", "prying_eyes", "read_magic", "rope_trick", "safeguarding",
		"screen", "sequester", "seven_eyes", "shades", "shadow_door", "shadow_magic", "shadow_monsters",
		"shadow_walk", "shadowcat", "skeletal_hands", "squaring_the_circle", "summon_shadow",
		"summon_spirit", "superior_magnetism", "tensers_floating_disc", "true_seeing", "unseen_servant",
		"ventriloquism", "vile_venom", "wall_of_bones", "wizard_eye", "zombie_double",
	}

	var got []string
	for _, e := range c.All() {
		if e.AddsBuff() || e.Duration.Kind != spelleffectsvc.DurationRounds || e.Target != spelleffectsvc.TargetNone {
			continue
		}
		got = append(got, e.ID)
	}
	assert.ElementsMatch(t, want, got)
}
