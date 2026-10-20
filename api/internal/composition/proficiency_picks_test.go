package composition_test

// The proficiency picks, non-weapon and weapon, end to end over the in-memory storage (ADR 0037 steps 3, 7, 10 to
// 14): the router, the real services and the fixture's catalogue.

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type picksAnswer struct {
	ProficiencySlots struct {
		NonWeapon struct {
			Available int `json:"available"`
			Spent     int `json:"spent"`
			Left      int `json:"left"`
		} `json:"nonWeapon"`
		Picks []struct {
			ID         string `json:"id"`
			Slots      int    `json:"slots"`
			Cost       int    `json:"cost"`
			ExtraSlots int    `json:"extraSlots"`
			Target     *int   `json:"target"`
		} `json:"picks"`
	} `json:"proficiencySlots"`
	Warnings []warningView `json:"warnings"`
}

type picksLog struct {
	Entries []struct {
		ID     int64  `json:"id"`
		Kind   string `json:"kind"`
		Status string `json:"status"`
	} `json:"entries"`
	TopID int64 `json:"topId"`
	Redo  *struct {
		ID int64 `json:"id"`
	} `json:"redo"`
}

// ADR 0037 steps 12 and 13 through the API: Kelsen (10 slots, 8 spent by his seeded picks) saves
// two extra slots on Tumbling; it is one proficiencies entry, his skills tab reads Tumbling's
// target 17 + 2; undo writes his picks back (target 17), redo the save again. A save over the
// budget is a 409 that logs nothing.
func TestProficiencies_Step13_PickUndoRedo(t *testing.T) {
	tb, _ := newTable(t)
	root, _ := tb.login("")
	var me struct {
		CurrentID *int64 `json:"currentId"`
	}
	require.Equal(t, http.StatusOK, tb.call(root, "GET", "/api/characters", "", &me))
	kelsen := *me.CurrentID

	tumbling := func() int {
		var skills struct {
			Proficiencies []struct {
				ID         string `json:"id"`
				ExtraSlots int    `json:"extraSlots"`
				Target     int    `json:"target"`
			} `json:"proficiencies"`
		}
		require.Equal(t, http.StatusOK, tb.showing(root, kelsen, "GET", "/api/skills", "", &skills))
		for _, p := range skills.Proficiencies {
			if p.ID == "tumbling" {
				return p.Target
			}
		}
		t.Fatal("no tumbling")
		return 0
	}
	readLog := func() picksLog {
		var l picksLog
		require.Equal(t, http.StatusOK, tb.showing(root, kelsen, "GET", "/api/log", "", &l))
		return l
	}
	require.Equal(t, 17, tumbling(), "DEX 17, no extra slot")
	before := readLog()

	save := `{"nonweapon":[{"id":"musical_instrument","slots":1},{"id":"tumbling","slots":3},{"id":"ventriloquism","slots":1},` +
		`{"id":"riding_land","slots":1},{"id":"appraising","slots":1},{"id":"singing","slots":1},{"id":"reading_writing","slots":1},` +
		`{"id":"local_history","slots":1}]}`
	var preview, saved picksAnswer
	require.Equal(t, http.StatusOK, tb.showing(root, kelsen, "POST", "/api/proficiencies/preview", save, &preview))
	assert.Equal(t, before.TopID, readLog().TopID, "the preview logs nothing")
	require.Equal(t, http.StatusOK, tb.showing(root, kelsen, "PUT", "/api/proficiencies", save, &saved))
	assert.Equal(t, preview, saved, "the preview answers what the save does")
	assert.Equal(t, 10, saved.ProficiencySlots.NonWeapon.Available)
	assert.Equal(t, 10, saved.ProficiencySlots.NonWeapon.Spent)
	assert.Empty(t, saved.Warnings, "spent exactly: no warning")
	assert.Equal(t, 2, saved.ProficiencySlots.Picks[1].ExtraSlots)
	assert.Equal(t, 19, *saved.ProficiencySlots.Picks[1].Target)
	assert.Equal(t, 19, tumbling())

	after := readLog()
	require.NotEmpty(t, after.Entries)
	assert.Equal(t, "proficiencies", after.Entries[0].Kind, "one entry, the newest")
	assert.Len(t, after.Entries, len(before.Entries)+1)

	// Over budget and growing: refused, nothing logged.
	over := strings.Replace(save, `"tumbling","slots":3`, `"tumbling","slots":4`, 1)
	var refused map[string]any
	require.Equal(t, http.StatusConflict, tb.showing(root, kelsen, "PUT", "/api/proficiencies", over, &refused))
	assert.Equal(t, "over_budget", refused["code"])
	assert.Equal(t, "nonweapon", refused["field"])
	assert.Equal(t, after.TopID, readLog().TopID, "a refusal logs nothing")

	require.Equal(t, http.StatusOK, tb.showing(root, kelsen, "POST", "/api/log/undo", `{"expectedTopId":`+itoa(after.TopID)+`}`, nil))
	assert.Equal(t, 17, tumbling(), "undo gives the picks back")
	undone := readLog()
	require.NotNil(t, undone.Redo)
	require.Equal(t, http.StatusOK, tb.showing(root, kelsen, "POST", "/api/log/redo", `{"expectedRedoId":`+itoa(undone.Redo.ID)+`}`, nil))
	assert.Equal(t, 19, tumbling(), "redo saves them again")
}

// ADR 0037 step 14 through the API: a level 3 bard with Intelligence 14 (3 + 0 + 4 = 7 slots) is
// created with picks within the budget, the leftover reminded on its read; one over the budget is
// a 409 and creates nothing; the draft answers the pool.
func TestProficiencies_Step14_CreationWithPicks(t *testing.T) {
	tb, _ := newTable(t)
	bob, _ := tb.login("bob@example.com")
	with := func(picks string) string {
		return strings.Replace(newBard, `"xp":2500`, `"xp":2500,"proficiencies":{"nonweapon":`+picks+`}`, 1)
	}
	var list listView
	require.Equal(t, http.StatusConflict, tb.call(bob, "POST", "/api/characters", with(`[{"id":"tumbling","slots":6},{"id":"tracking","slots":2}]`), nil))
	require.Equal(t, http.StatusOK, tb.call(bob, "GET", "/api/characters", "", &list))
	assert.Empty(t, list.Characters, "nothing created over the budget")

	require.Equal(t, http.StatusCreated, tb.call(bob, "POST", "/api/characters", with(`[{"id":"tumbling","slots":2},{"id":"tracking","slots":2}]`), &list))
	var read struct {
		Derived struct {
			Slots struct {
				NonWeapon struct {
					Available int `json:"available"`
					Left      int `json:"left"`
				} `json:"nonWeapon"`
			} `json:"proficiencySlots"`
		} `json:"derived"`
		Warnings []warningView `json:"warnings"`
	}
	require.Equal(t, http.StatusOK, tb.showing(bob, *list.CurrentID, "GET", "/api/character", "", &read))
	assert.Equal(t, 7, read.Derived.Slots.NonWeapon.Available)
	assert.Equal(t, 3, read.Derived.Slots.NonWeapon.Left)
	three := 3
	assert.Contains(t, read.Warnings, warningView{Code: "slots_to_spend", Field: "nonweapon", Pool: "nonweapon", Left: &three})

	d := tb.draft(t, bob, with(`[{"id":"tumbling","slots":2}]`))
	require.NotNil(t, d.Derived)
	pool := d.Derived["proficiencySlots"].(map[string]any)["nonWeapon"].(map[string]any)
	assert.InDelta(t, 7, pool["available"], 0)
	assert.InDelta(t, 2, pool["spent"], 0)
}

// ADR 0037 steps 7, 10, 12 and 13 through the API with the weapon pool: Kelsen (5 weapon slots
// at level 15, his five seeded weapon types spending them) swaps darts for a sling with a body of
// the weapon picks alone; it is one proficiencies entry and his non-weapon picks stay. A sixth
// type is a 409 on the weapon pool that logs nothing. Undo gives the darts back, redo the sling.
func TestProficiencies_Step13_WeaponPickUndoRedo(t *testing.T) {
	tb, _ := newTable(t)
	root, _ := tb.login("")
	var me struct {
		CurrentID *int64 `json:"currentId"`
	}
	require.Equal(t, http.StatusOK, tb.call(root, "GET", "/api/characters", "", &me))
	kelsen := *me.CurrentID

	type weaponView struct {
		Type    string `json:"type"`
		Slots   int    `json:"slots"`
		Allowed bool   `json:"allowed"`
	}
	type slotsView struct {
		Weapon struct {
			Available int `json:"available"`
			Spent     int `json:"spent"`
			Left      int `json:"left"`
		} `json:"weapon"`
		Picks       []struct{ ID string } `json:"picks"`
		WeaponPicks []weaponView          `json:"weaponPicks"`
	}
	read := func() slotsView {
		var c struct {
			Derived struct {
				Slots slotsView `json:"proficiencySlots"`
			} `json:"derived"`
		}
		require.Equal(t, http.StatusOK, tb.showing(root, kelsen, "GET", "/api/character", "", &c))
		return c.Derived.Slots
	}
	types := func(s slotsView) []string {
		out := []string{}
		for _, w := range s.WeaponPicks {
			out = append(out, w.Type)
		}
		return out
	}
	readLog := func() picksLog {
		var l picksLog
		require.Equal(t, http.StatusOK, tb.showing(root, kelsen, "GET", "/api/log", "", &l))
		return l
	}
	seeded := read()
	assert.Equal(t, []string{"short_bow", "dagger", "dart", "long_sword", "other_melee"}, types(seeded))
	assert.Equal(t, 5, seeded.Weapon.Available)
	assert.Equal(t, 5, seeded.Weapon.Spent)
	before := readLog()

	save := `{"weapon":[{"type":"short_bow","slots":1},{"type":"dagger","slots":1},{"type":"sling","slots":1},` +
		`{"type":"long_sword","slots":1},{"type":"other_melee","slots":1}]}`
	var saved picksAnswer
	require.Equal(t, http.StatusOK, tb.showing(root, kelsen, "PUT", "/api/proficiencies", save, &saved))
	now := read()
	assert.Equal(t, []string{"short_bow", "dagger", "sling", "long_sword", "other_melee"}, types(now))
	assert.Len(t, now.Picks, len(seeded.Picks), "the non-weapon picks stay")
	after := readLog()
	assert.Equal(t, "proficiencies", after.Entries[0].Kind, "one entry, the newest")
	assert.Len(t, after.Entries, len(before.Entries)+1)

	over := strings.Replace(save, `]}`, `,{"type":"dart","slots":1}]}`, 1)
	var refused map[string]any
	require.Equal(t, http.StatusConflict, tb.showing(root, kelsen, "PUT", "/api/proficiencies", over, &refused))
	assert.Equal(t, "over_budget", refused["code"])
	assert.Equal(t, "weapon", refused["field"])
	assert.InDelta(t, 6, refused["spent"], 0)
	assert.InDelta(t, 5, refused["limit"], 0)
	assert.Equal(t, after.TopID, readLog().TopID, "a refusal logs nothing")
	assert.Equal(t, types(now), types(read()), "a refusal writes nothing")

	require.Equal(t, http.StatusOK, tb.showing(root, kelsen, "POST", "/api/log/undo", `{"expectedTopId":`+itoa(after.TopID)+`}`, nil))
	assert.Equal(t, types(seeded), types(read()), "undo gives the picks back")
	undone := readLog()
	require.NotNil(t, undone.Redo)
	require.Equal(t, http.StatusOK, tb.showing(root, kelsen, "POST", "/api/log/redo", `{"expectedRedoId":`+itoa(undone.Redo.ID)+`}`, nil))
	assert.Equal(t, types(now), types(read()), "redo saves them again")
}

// ADR 0037 steps 7, 10 and 14 through the API: a level 3 bard has 2 weapon slots (a rogue's next
// comes at 4); created with three weapon picks it is a 409 on the weapon pool and nothing is
// created; with two it is created and reads them; an unknown type is a 400 naming its field.
func TestProficiencies_Step14_CreationWithWeaponPicks(t *testing.T) {
	tb, _ := newTable(t)
	bob, _ := tb.login("bob@example.com")
	with := func(picks string) string {
		return strings.Replace(newBard, `"xp":2500`, `"xp":2500,"proficiencies":{"weapon":`+picks+`}`, 1)
	}
	var refused map[string]any
	require.Equal(t, http.StatusConflict, tb.call(bob, "POST", "/api/characters",
		with(`[{"type":"dagger","slots":1},{"type":"sling","slots":1},{"type":"long_bow","slots":1}]`), &refused))
	assert.Equal(t, "weapon", refused["field"])
	require.Equal(t, http.StatusBadRequest, tb.call(bob, "POST", "/api/characters", with(`[{"type":"katana","slots":1}]`), &refused))
	assert.Equal(t, "weapon.katana", refused["field"])
	var list listView
	require.Equal(t, http.StatusOK, tb.call(bob, "GET", "/api/characters", "", &list))
	assert.Empty(t, list.Characters, "nothing created")

	require.Equal(t, http.StatusCreated, tb.call(bob, "POST", "/api/characters", with(`[{"type":"dagger","slots":1},{"type":"long_bow","slots":1}]`), &list))
	var c struct {
		Derived struct {
			Slots struct {
				Weapon struct {
					Available int `json:"available"`
					Left      int `json:"left"`
				} `json:"weapon"`
				WeaponPicks []struct {
					Type string `json:"type"`
				} `json:"weaponPicks"`
			} `json:"proficiencySlots"`
		} `json:"derived"`
	}
	require.Equal(t, http.StatusOK, tb.showing(bob, *list.CurrentID, "GET", "/api/character", "", &c))
	assert.Equal(t, 2, c.Derived.Slots.Weapon.Available)
	assert.Equal(t, 0, c.Derived.Slots.Weapon.Left)
	require.Len(t, c.Derived.Slots.WeaponPicks, 2)
	assert.Equal(t, "long_bow", c.Derived.Slots.WeaponPicks[1].Type)
}
