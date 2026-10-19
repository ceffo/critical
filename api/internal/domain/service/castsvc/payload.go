package castsvc

import (
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
	"github.com/ceffo/critical/api/internal/domain/service/spelleffectsvc"
)

// CastBreakdown is the one wire shape of a cast (a cast, a received spell or
// an added effect): the payload of its action-log entry and the answer of
// the cast endpoints (POST /api/spells/{id}/cast, /receive and /effect) are
// this struct, so the UI reads both with one type. The answer adds the entry
// id and what is not part of the cast (the catalogue spell, the slots left;
// see casthdl). Entries already in the log must stay decodable: add fields,
// never rename them (ADR 0010, ADR 0018); where the payload and the previous
// answer named a field differently, the payload's name won.
type CastBreakdown struct {
	// Action is "cast", "received" or "effect" (added from the lookup, ADR
	// 0004 step 19).
	Action  string `json:"action"`
	SpellID string `json:"spellId"`
	// Spell is the spell's name as recorded at write time (ADR 0023
	// consequences); the UI names the spell from its id.
	Spell       string `json:"spell"`
	CasterLevel int    `json:"casterLevel"`
	// Mode is the key of the cast-time mode the spell was cast in; absent for
	// a spell without modes and on casts logged before modes existed.
	Mode   string      `json:"mode,omitempty"`
	Buff   *CastBuff   `json:"buff,omitempty"`
	Damage *CastDamage `json:"damage,omitempty"`
	// ExtraSlots are the levels of the extra slots the cast granted.
	ExtraSlots []int `json:"extraSlots,omitempty"`
	// Item is the inventory item the cast spent.
	Item *CastItem `json:"item,omitempty"`
	// Attack is the spell's to-hit roll (one d20, no crit). Entries logged
	// before the cast mechanics were general carry it as "orb" (with the gem
	// name), which the UI no longer reads.
	Attack *CastAttack `json:"attack,omitempty"`
	// Save is the target's saving throw; absent on casts logged before it
	// was recorded.
	Save *CastSave `json:"save,omitempty"`
	// Failure is the low-Wisdom spell failure roll (ADR 0030 step 14);
	// absent when no die was rolled. Failed: the spell failed (d100 at or
	// under the chance), its slot spent and nothing applied.
	Failure *CastFailure `json:"failure,omitempty"`
	// Exposure is the exposure damage served at the caster level, so the
	// journal's cast card rolls an exposure from it (ADR 0008 step 16);
	// absent without one.
	Exposure *spelleffectsvc.ExposureAtWire `json:"exposure,omitempty"`

	// Level to Untracked are absent on casts logged before the payload was
	// the answer (ADR 0018: add fields). Level is the spell's level; SlotIndex the slot spent (a
	// cast only).
	Level     int  `json:"level"`
	SlotIndex *int `json:"slotIndex,omitempty"`
	// Kind, Confidence and Target describe the spell-effects entry (absent:
	// not in it). Its English note is neither logged nor served: the UI
	// never showed it, and the backend serves no presentation text.
	Kind       string `json:"kind,omitempty"`
	Confidence string `json:"confidence,omitempty"`
	Target     string `json:"target,omitempty"`
	// TextOnly is why the cast shows only its text (absent: it does more).
	TextOnly string `json:"textOnly,omitempty"`
	// Untracked are the effects to apply by hand ("[]": none).
	Untracked []CastUntracked `json:"untracked"`

	// LogEntryID is the entry recording the cast: set on the endpoints'
	// answer, never in the payload (the entry carries its own id).
	LogEntryID int64 `json:"logEntryId,omitempty"`
}

// CastSave is the target's saving throw (the DM's side).
type CastSave struct {
	Type   string `json:"type"`
	Effect string `json:"effect"`
}

// FormulaRoll is one evaluated formula of a cast: the formula, every die it
// threw (combatsvc.BreakdownDie, the one die shape) and the result. Parts
// are the rolls of the instance's damage parts, in the order of
// CastDamage.Parts (absent: no parts, or logged before parts existed).
type FormulaRoll struct {
	Formula string                   `json:"formula"`
	Dice    []combatsvc.BreakdownDie `json:"dice"`
	Total   int                      `json:"total"`
	Parts   []FormulaRoll            `json:"parts,omitempty"`
	// Step is the dice step (round or instance number) of the roll; absent
	// without steps and on casts logged before steps existed.
	Step int `json:"step,omitempty"`
}

// CastItem is the item a cast spent and how many are left.
type CastItem struct {
	ItemID    string `json:"itemId"`
	Name      string `json:"name"`
	Remaining int    `json:"remaining"`
}

// CastFailure is a cast's spell failure d100: failed at or under the
// chance.
type CastFailure struct {
	D100   int  `json:"d100"`
	Chance int  `json:"chance"`
	Failed bool `json:"failed"`
}

// CastAttack is a spell's to-hit roll; what a hit does is the DM's call.
// BaseThac0 to HitMargin are absent on casts logged before the payload was
// the answer.
type CastAttack struct {
	D20      int  `json:"d20"`
	TargetAC int  `json:"targetAc"`
	Hit      bool `json:"hit"`

	BaseThac0      int                           `json:"baseThac0"`
	Modifiers      []combatsvc.BreakdownModifier `json:"modifiers"`
	EffectiveThac0 int                           `json:"effectiveThac0"`
	TargetACNeeded int                           `json:"targetACNeeded"`
	HitMargin      int                           `json:"hitMargin"`
	// D20Roll is D20 under the name the previous release's web reads from
	// the answer, kept for one release so a page of that release still shows
	// the die against this server during the rolling deploy (ADR 0024 step
	// 5.2). Drop it in the next release.
	D20Roll int `json:"d20Roll,omitempty"`
}

// CastBuff is the buff a cast puts on (or tells an ally about) and the rolls
// behind it. ID to Effects are absent on casts logged before the payload was
// the answer.
type CastBuff struct {
	Name    string `json:"name"`
	Rounds  int    `json:"rounds,omitempty"`
	Charges int    `json:"charges,omitempty"`
	StrGain int    `json:"strGain,omitempty"`
	Applied bool   `json:"applied"`

	ID           string       `json:"id,omitempty"`
	DurationRoll *FormulaRoll `json:"durationRoll,omitempty"`
	ChargesRoll  *FormulaRoll `json:"chargesRoll,omitempty"`
	ChargeUnit   string       `json:"chargeUnit,omitempty"`
	StrGainRoll  *FormulaRoll `json:"strGainRoll,omitempty"`
	// Effects are the engine-tracked effects, keyed like the buff API
	// (to_hit, ac, str_bonus, ...; "{}": none).
	Effects map[string]float64 `json:"effects"`
}

// CastDamage is a damage roll: Dice, Instances, Rounds and Rolls (every
// instance roll, round by round, with its dice) are absent on casts logged
// before they were recorded. Parts and TotalOnSave are set only for a damage
// made of parts.
type CastDamage struct {
	Total       int              `json:"total"`
	Type        string           `json:"type"`
	Dice        string           `json:"dice,omitempty"`
	Instances   int              `json:"instances"`
	Rounds      int              `json:"rounds"`
	Rolls       []FormulaRoll    `json:"rolls"`
	Parts       []CastDamagePart `json:"parts,omitempty"`
	TotalOnSave *int             `json:"totalOnSave,omitempty"`
	// StepsPer is "round" or "instance" for a damage made of dice steps (each
	// roll carries its step); absent otherwise.
	StepsPer string `json:"stepsPer,omitempty"`
}

// CastDamagePart is one damage part: its formula and what a save does to it
// (none, negates).
type CastDamagePart struct {
	Formula string `json:"formula"`
	Save    string `json:"save"`
}

// CastUntracked is an effect to apply by hand.
type CastUntracked struct {
	Key       string `json:"key"`
	Value     int    `json:"value"`
	Condition string `json:"condition,omitempty"`
}

// NewCastBreakdown is the breakdown of res as the cast endpoints answer it:
// the payload logged for res plus the id of its entry.
func NewCastBreakdown(res CastResult) CastBreakdown {
	out := toBreakdown(res)
	out.LogEntryID = res.EntryID
	return out
}

// toBreakdown is the payload logged for res: its breakdown without the
// entry id.
func toBreakdown(res CastResult) CastBreakdown {
	r := CastBreakdown{
		Action: string(res.Source), SpellID: res.SpellID, Spell: res.Name, CasterLevel: res.CasterLevel, Mode: res.Mode,
		Level: res.Level, SlotIndex: res.SlotIndex, Kind: string(res.Kind), Confidence: string(res.Confidence),
		Target: string(res.Target), TextOnly: string(res.TextOnly),
		Untracked: []CastUntracked{},
	}
	if b := res.Buff; b != nil {
		r.Buff = &CastBuff{
			Name: b.Buff.Name, Rounds: b.Buff.Duration, Charges: b.Buff.Charges, Applied: b.Applied,
			ID: b.Buff.ID, DurationRoll: toFormulaRoll(b.Duration), ChargesRoll: toFormulaRoll(b.Charges),
			ChargeUnit: b.ChargeUnit, StrGainRoll: toFormulaRoll(b.StrGain), Effects: buffEffects(b),
		}
		if b.StrGain != nil {
			r.Buff.StrGain = b.StrGain.Total
		}
	}
	r.Damage = toCastDamage(res.Damage)
	r.Exposure = res.Exposure.Wire()
	if s := res.Save; s != nil {
		r.Save = &CastSave{Type: string(s.Type), Effect: string(s.Effect)}
	}
	r.ExtraSlots = res.ExtraSlots
	if c := res.Consumed; c != nil {
		r.Item = &CastItem{ItemID: c.ItemID, Name: c.Name, Remaining: c.Remaining}
	}
	if a := res.Attack; a != nil {
		r.Attack = &CastAttack{
			D20: a.D20Roll, TargetAC: a.TargetAC, Hit: a.Hit,
			BaseThac0: a.BaseThac0, Modifiers: combatsvc.BreakdownModifiers(a.Modifiers), EffectiveThac0: a.EffectiveThac0,
			TargetACNeeded: a.TargetACNeeded, HitMargin: a.HitMargin, D20Roll: a.D20Roll,
		}
	}
	if f := res.Failure; f != nil {
		r.Failure = &CastFailure{D100: f.Roll, Chance: f.Chance, Failed: f.Failed}
	}
	for _, u := range res.Untracked {
		r.Untracked = append(r.Untracked, CastUntracked(u))
	}
	return r
}

// buffEffects are the engine-tracked effects of b, keyed like the buff API.
func buffEffects(b *AppliedBuff) map[string]float64 {
	fx := b.Buff.Effects
	out := map[string]float64{}
	ints := map[string]*int{
		"min_str": fx.MinStr, "weapon_buff": fx.WeaponBuff, "save_bonus": fx.SaveBonus,
		"str_bonus": fx.StrBonus, "str_max": fx.StrMax,
		"ac": fx.AC, "to_hit": fx.ToHit, "damage": fx.Damage,
	}
	for k, v := range ints {
		if v != nil {
			out[k] = float64(*v)
		}
	}
	if fx.SpellsBuff != nil {
		out["spells_buff"] = *fx.SpellsBuff
	}
	if fx.Multiplier != nil {
		out["multiplier"] = *fx.Multiplier
	}
	return out
}

func toFormulaRoll(roll *Roll) *FormulaRoll {
	if roll == nil {
		return nil
	}
	out := formulaRoll(*roll)
	return &out
}

func formulaRoll(roll Roll) FormulaRoll {
	dice := make([]combatsvc.BreakdownDie, len(roll.Dice))
	for i, die := range roll.Dice {
		dice[i] = combatsvc.BreakdownDie(die)
	}
	out := FormulaRoll{Formula: roll.Formula, Dice: dice, Total: roll.Total, Step: roll.Step}
	for _, p := range roll.Parts {
		out.Parts = append(out.Parts, formulaRoll(p))
	}
	return out
}

// ExposureBreakdown is the one wire shape of one round of exposure (ADR 0008
// step 16): the payload of its action-log entry and the answer of POST
// /api/spells/exposure, which adds the entry id and the HP the ledger
// applied (see casthdl). Add fields, never rename them.
type ExposureBreakdown struct {
	SpellID string `json:"spellId"`
	// Spell is the spell's name as recorded at write time (as on a cast).
	Spell       string `json:"spell"`
	CasterLevel int    `json:"casterLevel"`
	// Mode is the key of the exposure mode rolled; absent without modes.
	Mode string `json:"mode,omitempty"`
	// Round is the round of exposure, from 1.
	Round int `json:"round"`
	// OnSelf: the Bard was exposed and the entry's HP delta is the damage.
	OnSelf bool        `json:"onSelf"`
	Damage *CastDamage `json:"damage"`
	// Exposure is the exposure damage served at the caster level, so the
	// next round of exposure rolls from the entry.
	Exposure *spelleffectsvc.ExposureAtWire `json:"exposure,omitempty"`
	// LogEntryID is the entry recording the round: set on the endpoint's
	// answer, never in the payload.
	LogEntryID int64 `json:"logEntryId,omitempty"`
}

// NewExposureBreakdown is the breakdown of res as the exposure endpoint
// answers it: the payload logged for res plus the id of its entry.
func NewExposureBreakdown(res ExposureResult) ExposureBreakdown {
	out := toExposureBreakdown(res)
	out.LogEntryID = res.EntryID
	return out
}

func toExposureBreakdown(res ExposureResult) ExposureBreakdown {
	return ExposureBreakdown{
		SpellID: res.SpellID, Spell: res.Name, CasterLevel: res.CasterLevel, Mode: res.Mode,
		Round: res.Round, OnSelf: res.OnSelf, Damage: toCastDamage(res.Damage),
		Exposure: res.Exposure.Wire(),
	}
}

func toCastDamage(d *DamageResult) *CastDamage {
	if d == nil {
		return nil
	}
	r := &CastDamage{
		Total: d.Total, Type: d.Type, Dice: d.Dice, Instances: d.Instances, Rounds: d.Rounds,
		Rolls: make([]FormulaRoll, 0, len(d.Rolls)), TotalOnSave: d.TotalOnSave, StepsPer: string(d.StepsPer),
	}
	for _, roll := range d.Rolls {
		r.Rolls = append(r.Rolls, formulaRoll(roll))
	}
	for _, p := range d.Parts {
		r.Parts = append(r.Parts, CastDamagePart{Formula: p.Formula, Save: string(p.Save)})
	}
	return r
}
