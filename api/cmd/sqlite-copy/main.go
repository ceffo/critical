// Command sqlite-copy copies a SQLite database of the app into an empty PostgreSQL database
// (ADR 0025 step 9): `just db-import-sqlite <file> <dsn>`. The SQLite file is only read. It prints
// the number of rows of each table, never their content. Retired once production is seeded.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"text/tabwriter"

	"github.com/ceffo/critical/api/internal/sqlitecopy"
	"github.com/ceffo/critical/api/internal/sqlitecopy/pgtarget"
	"github.com/ceffo/critical/api/internal/sqlitecopy/sqlitesource"
	"github.com/ceffo/critical/api/internal/storage/postgres"
)

const usage = `usage: sqlite-copy <sqlite file> <postgres dsn>

Copies every table of the app's SQLite file into the PostgreSQL database, which must be new (no
table: it gets the server's migrations) or on the server's baseline without any row. The SQLite
file is read, never written.`

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "sqlite-copy:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, out io.Writer) (err error) {
	if len(args) != 2 {
		return fmt.Errorf("two arguments expected\n%s", usage)
	}
	sqliteMigrations, err := sqlitesource.Migrations()
	if err != nil {
		return err
	}
	baseline, err := postgres.MigrationNames()
	if err != nil {
		return err
	}
	src, err := sqlitesource.Open(ctx, args[0])
	if err != nil {
		return err
	}
	defer func() {
		if cerr := src.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()
	db, err := postgres.Open(args[1])
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	counts, err := sqlitecopy.Run(ctx, src, pgtarget.New(db), sqliteMigrations, baseline)
	if err != nil {
		return err
	}
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "table\trows")
	var total int64
	for _, c := range counts {
		_, _ = fmt.Fprintf(w, "%s\t%d\n", c.Table, c.Rows)
		total += c.Rows
	}
	_, _ = fmt.Fprintf(w, "total (%d tables, counts equal)\t%d\n", len(counts), total)
	return w.Flush()
}
