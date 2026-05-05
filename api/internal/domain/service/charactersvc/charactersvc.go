// Package charactersvc owns read access to character core attributes (stats,
// HP max, THAC0, saving throws). It is read-only today; writes happen via
// seeding in the storage layer.
package charactersvc

import (
	"context"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain"
)

// SVC is the driver port.
type SVC interface {
	domain.Service
	GetCharacter(ctx context.Context, id int64) (*Character, error)
}

// Reader is the read-side driven port. Per clean_architecture.md §5.4, reads
// don't need the full DAL detour; this port returns domain entities directly.
type Reader interface {
	GetCharacter(ctx context.Context, id int64) (*Character, error)
}

type service struct {
	reader Reader
}

// New constructs the service.
func New(reader Reader) (SVC, error) {
	if reader == nil {
		return nil, fmt.Errorf("charactersvc: nil reader")
	}
	return &service{reader: reader}, nil
}

// NewWithConfig is here for symmetry with the other services. There is no
// configurable surface today.
func NewWithConfig(_ *Config, reader Reader) (SVC, error) { return New(reader) }

func (s *service) GetCharacter(ctx context.Context, id int64) (*Character, error) {
	return s.reader.GetCharacter(ctx, id)
}

func (s *service) Health(_ context.Context) error { return nil }
func (s *service) Close(_ context.Context) error  { return nil }
