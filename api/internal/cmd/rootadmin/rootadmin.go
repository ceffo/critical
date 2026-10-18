// Package rootadmin sets the one root admin (ADR 0029 step 5.1): a committed
// script run locally against a DSN, never hosting configuration. It creates
// the user as an admin with no Google subject (the first Google login of the
// email binds it), is idempotent, and refuses another email once the root has
// logged in.
package rootadmin

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/signal"
	"syscall"

	"github.com/ceffo/critical/api/internal/composition"
	"github.com/ceffo/critical/api/internal/domain/service/authsvc"
)

// Opener opens the users' service of a database (composition.OpenAuth).
type Opener func(ctx context.Context, dsn string) (authsvc.SVC, func() error, error)

// Run is the script: args are the one email. dsn is required: the script
// never guesses which database to change.
func Run(args []string, dsn string, out io.Writer) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	return run(ctx, args, dsn, out, composition.OpenAuth)
}

func run(ctx context.Context, args []string, dsn string, out io.Writer, open Opener) error {
	if len(args) != 1 || args[0] == "" {
		return errors.New("usage: root-admin <email> (with CRITICAL_DSN set)")
	}
	if dsn == "" {
		return errors.New("CRITICAL_DSN is not set: name the database to change")
	}
	svc, closeDB, err := open(ctx, dsn)
	if err != nil {
		return err
	}
	defer func() { _ = closeDB() }()
	u, err := svc.SetRootAdmin(ctx, args[0])
	if errors.Is(err, authsvc.ErrRootTaken) {
		return fmt.Errorf("%w; the root admin has logged in and cannot be moved by this script", err)
	}
	if err != nil {
		return fmt.Errorf("%w (has the server run once on this database, so its migrations are applied?)", err)
	}
	state := "not logged in yet: the first Google login of this email binds it"
	if u.GoogleSubject != "" {
		state = "bound to its Google account"
	}
	_, err = fmt.Fprintf(out, "root admin: %s (%s)\n", u.Email, state)
	return err
}
