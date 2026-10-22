package ruletablesvc

import "fmt"

// IntelligenceFor is PHB Table 4's row of an Intelligence score (ADR 0030
// step 20): the highest spell level, the chance to learn a spell and the
// maximum number of spells per level. A score outside the table is an
// error.
func (t *Tables) IntelligenceFor(score int) (IntelligenceRow, error) {
	c, err := t.classTables()
	if err != nil {
		return IntelligenceRow{}, err
	}
	for _, r := range c.Intelligence {
		if score >= r.IntMin && score <= r.IntMax {
			return r, nil
		}
	}
	return IntelligenceRow{}, fmt.Errorf("intelligence: score %d outside the table", score)
}
