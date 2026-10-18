// Package main is the root-admin script (ADR 0029 step 5.1), run locally by
// `just root-admin <email>` against a DSN: the local compose database, or
// staging and production through `fly proxy` (docs/deploy.md).
package main

import (
	"fmt"
	"os"

	"github.com/ceffo/critical/api/internal/cmd/rootadmin"
)

func main() {
	if err := rootadmin.Run(os.Args[1:], os.Getenv("CRITICAL_DSN"), os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "root-admin:", err)
		os.Exit(1)
	}
}
