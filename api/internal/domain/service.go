// Package domain holds cross-cutting primitives shared by every business
// service: a generic Service interface (Health/Close), sentinel errors used at
// boundaries (DAL → service → handler), and other generic helpers.
//
// Concrete business types and ports live in internal/domain/service/<svc>.
package domain

import "context"

// Service is the uniform shape every business service exposes so the
// composition root can iterate a []Service to shut everything down.
type Service interface {
	Health(ctx context.Context) error
	Close(ctx context.Context) error
}
