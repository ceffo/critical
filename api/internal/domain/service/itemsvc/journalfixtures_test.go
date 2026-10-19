package itemsvc

import (
	"testing"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/actionlog/journalfixture"
)

// The web journal's fixtures of the kinds this service logs are what it logs.
func TestJournalFixturesAreWhatIsLogged(t *testing.T) {
	journalfixture.Matches[stateRecord](t, actionlog.KindEquipmentState)
	journalfixture.Matches[changeRecord](t, actionlog.KindItemAdd)
	journalfixture.Matches[changeRecord](t, actionlog.KindItemEdit)
	journalfixture.Matches[changeRecord](t, actionlog.KindItemRemove)
	journalfixture.Matches[useRecord](t, actionlog.KindItemUse)
	journalfixture.Matches[restockRecord](t, actionlog.KindItemRestock)
}
