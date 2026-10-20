package skillsvc

import (
	"testing"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/actionlog/journalfixture"
)

// The web journal's fixtures of the kinds this service logs are what it logs.
func TestJournalFixturesAreWhatIsLogged(t *testing.T) {
	journalfixture.Matches[pointsRecord](t, actionlog.KindSkillPoints)
	journalfixture.Matches[picksRecord](t, actionlog.KindProficiencies)
	journalfixture.Matches[checkRecord](t, actionlog.KindSkillCheck)
	journalfixture.Matches[abilityRecord](t, actionlog.KindAbilityCheck)
}
