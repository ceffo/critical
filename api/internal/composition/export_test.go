package composition

import "github.com/ceffo/critical/api/internal/storage/memory"

// MemoryStorage is memoryStorage for the external tests, which build the root
// over a store they also drive (adding a second character).
func MemoryStorage(m *memory.Store) Storage { return memoryStorage(m) }
