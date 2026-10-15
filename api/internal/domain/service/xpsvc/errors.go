package xpsvc

import (
	"fmt"

	"github.com/ceffo/critical/api/internal/domain"
)

// ErrEntryNotFound is an unknown entry id (404).
var ErrEntryNotFound = fmt.Errorf("xp entry %w", domain.ErrNotFound)
