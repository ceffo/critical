package domain

import "github.com/ceffo/critical/api/internal/domain/actionlog"

// Module is what a service declares to the composition root, which loops
// over the services' modules instead of wiring each one by hand: the
// snapshot codecs that write its state-key families back on undo and redo
// (ADR 0018 steps 2 and 8, ADR 0019 step 1) and the payload views of its
// entry kinds. The action log refuses a family or a kind two modules claim,
// so the build fails, never the player's undo.
type Module struct {
	// Name names the service in the build's errors.
	Name string
	// Codecs restore the service's state-key families, one per family
	// ("items" restores "items:arrows_plus3").
	Codecs []actionlog.Codec
	// Views fill, on read, the payload fields an entry kind gained after its
	// entries were logged.
	Views actionlog.PayloadViews
}

// Restorer is the codec of family, nil when the module declares none.
func (m Module) Restorer(family string) actionlog.Codec {
	for _, c := range m.Codecs {
		if c.Family() == family {
			return c
		}
	}
	return nil
}
