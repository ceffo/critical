package psionicsvc

import (
	"testing"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/actionlog/journalfixture"
)

// The web journal's fixtures of the kinds this service logs are what it logs.
func TestJournalFixturesAreWhatIsLogged(t *testing.T) {
	journalfixture.Matches[recoveryRecord](t, actionlog.KindPSPRecovery)
	journalfixture.Matches[knownRecord](t, actionlog.KindPsionicPick)
	journalfixture.Matches[CheckRecord](t, actionlog.KindPowerCheck)
	journalfixture.Matches[RaiseRecord](t, actionlog.KindDefenceRaised)
	journalfixture.Matches[DefenceRecord](t, actionlog.KindPsionicDefence)
	journalfixture.Matches[AttackRecord](t, actionlog.KindPsionicAttack)
}
