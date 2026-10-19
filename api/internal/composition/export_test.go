package composition

import (
	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/actionlogsvc"
	"github.com/ceffo/critical/api/internal/storage/memory"
)

// MemoryStorage is memoryStorage for the external tests, which build the root
// over a store they also drive (adding a second character).
func MemoryStorage(m *memory.Store) Storage { return memoryStorage(m) }

// Services is the root's one list of services, in build order: the modules
// Build registered on the log and what Close walks.
func Services(b *BackingServices) []domain.Service { return b.services }

// RegisterModules is the loop Build ends with.
func RegisterModules(log actionlogsvc.SVC, services []domain.Service) error {
	return registerModules(log, services)
}
