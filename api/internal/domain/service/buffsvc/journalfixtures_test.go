package buffsvc

import (
	"testing"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/actionlog/journalfixture"
)

// The web journal's fixtures of the kinds this service logs are what it logs.
func TestJournalFixturesAreWhatIsLogged(t *testing.T) {
	journalfixture.Matches[initiativePayload](t, actionlog.KindInitiative)
	journalfixture.Matches[logPayload](t, actionlog.KindBuffChange)
	journalfixture.Matches[logPayload](t, actionlog.KindRoundAdvance)
	journalfixture.Matches[logPayload](t, actionlog.KindRoundReset)
	journalfixture.Matches[logPayload](t, actionlog.KindRoundSet)
	journalfixture.Matches[logPayload](t, actionlog.KindTimePassed)
}
