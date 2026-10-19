package castsvc

import (
	"testing"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/actionlog/journalfixture"
)

// The web journal's fixtures of the kinds this service logs are what it logs.
func TestJournalFixturesAreWhatIsLogged(t *testing.T) {
	journalfixture.Matches[CastBreakdown](t, actionlog.KindCast)
	// The action log merges the HP breakdown into the payload when the Bard is the one exposed.
	journalfixture.Matches[struct {
		ExposureBreakdown
		*actionlog.HPBreakdown
	}](t, actionlog.KindExposure)
}
