package castsvc

import (
	"testing"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/actionlog/journalfixture"
)

// The web journal's fixtures of the kinds this service logs are what it logs.
func TestJournalFixturesAreWhatIsLogged(t *testing.T) {
	// The action log merges the HP breakdown into the payload of a cast that heals the Bard (ADR 0008 step 17).
	journalfixture.Matches[struct {
		CastBreakdown
		*actionlog.HPBreakdown
	}](t, actionlog.KindCast)
	// The action log merges the HP breakdown into the payload when the Bard is the one exposed.
	journalfixture.Matches[struct {
		ExposureBreakdown
		*actionlog.HPBreakdown
	}](t, actionlog.KindExposure)
}
