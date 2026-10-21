package mailadpt

import (
	"context"
	"sync"
)

// Recorder keeps the mails it is asked to send, for tests: Err, when set, is
// what Send returns (the mail is still recorded).
type Recorder struct {
	mu   sync.Mutex
	sent []Message
	// Err is returned by every Send.
	Err error
	// Block, when set, makes Send wait for it or for the context.
	Block <-chan struct{}
}

var _ Mailer = (*Recorder)(nil)

// Send implements Mailer.
func (r *Recorder) Send(ctx context.Context, m Message) error {
	if r.Block != nil {
		select {
		case <-r.Block:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sent = append(r.sent, m)
	return r.Err
}

// Sent is the mails recorded so far.
func (r *Recorder) Sent() []Message {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Message(nil), r.sent...)
}
