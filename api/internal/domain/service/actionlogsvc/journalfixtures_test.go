package actionlogsvc

import (
	"testing"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/actionlog/journalfixture"
)

// The web journal's fixtures of the kinds this service logs are what it logs.
func TestJournalFixturesAreWhatIsLogged(t *testing.T) {
	journalfixture.Matches[actionlog.CorrectionStep](t, actionlog.KindCorrection)
}
