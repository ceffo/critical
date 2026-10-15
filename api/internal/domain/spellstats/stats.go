package spellstats

// Raw is the six PHB stat texts of a spell, as in the spell catalogue artifact.
type Raw struct {
	CastingTime, Range, Duration, AreaOfEffect, Components, SavingThrow string
}

// Stats is the typed form of a spell's stat block. It is parsed once, at
// seed time, and stored as JSON.
type Stats struct {
	CastingTime CastingTime  `json:"casting_time"`
	Range       Range        `json:"range"`
	Duration    Duration     `json:"duration"`
	Area        AreaOfEffect `json:"area"`
	Components  Components   `json:"components"`
	Save        Save         `json:"save"`
}

// Parse types every field of a stat block. It never fails: text it cannot
// read becomes a special kind that keeps the raw text.
func Parse(r Raw) Stats {
	return Stats{
		CastingTime: ParseCastingTime(r.CastingTime),
		Range:       ParseRange(r.Range),
		Duration:    ParseDuration(r.Duration),
		Area:        ParseArea(r.AreaOfEffect),
		Components:  ParseComponents(r.Components),
		Save:        ParseSave(r.SavingThrow),
	}
}

// Special lists the fields of s that fell back to special, by field name.
func (s Stats) Special() []string {
	var out []string
	if s.CastingTime.Kind == KindSpecial {
		out = append(out, "casting_time")
	}
	if s.Range.Kind == KindSpecial {
		out = append(out, "range")
	}
	if s.Duration.Kind == KindSpecial {
		out = append(out, "duration")
	}
	if s.Area.Kind == KindSpecial {
		out = append(out, "area_of_effect")
	}
	if s.Components.Special {
		out = append(out, "components")
	}
	for _, k := range s.Save.Kinds {
		if k == SaveSpecial {
			out = append(out, "saving_throw")
			break
		}
	}
	return out
}

// Values are the computed fields at one caster level.
type Values struct {
	CasterLevel int       `json:"caster_level"`
	CastingTime Value     `json:"casting_time"`
	Range       Value     `json:"range"`
	Duration    Value     `json:"duration"`
	Area        AreaValue `json:"area"`
}

// At computes the stat block for casterLevel.
func (s Stats) At(casterLevel int) Values {
	return Values{
		CasterLevel: casterLevel,
		CastingTime: s.CastingTime.At(casterLevel),
		Range:       s.Range.At(casterLevel),
		Duration:    s.Duration.At(casterLevel),
		Area:        s.Area.At(casterLevel),
	}
}
