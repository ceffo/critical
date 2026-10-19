package charactersvc

import (
	"testing"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/actionlog/journalfixture"
)

// armorRecord is the payload of a skill_armor entry, logged before ADR 0033
// step 14 retired the entered armour: no move logs it any more, but the
// entries already in the log still show in the journal.
type armorRecord struct {
	Before string `json:"before"`
	After  string `json:"after"`
}

// The web journal's fixtures of the kinds this service logged are what it
// logged.
func TestJournalFixturesAreWhatIsLogged(t *testing.T) {
	journalfixture.Matches[armorRecord](t, actionlog.KindSkillArmor)
}
