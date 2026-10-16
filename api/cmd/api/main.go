// Package main is the entry point for the critical api binary.
package main

import (
	"log/slog"
	"os"

	api "github.com/ceffo/critical/api/internal/cmd/api"
)

func main() {
	os.Exit(run(api.Start))
}

// run executes start and, when it fails, logs the error at ERROR on the
// default logger (installed by start, or the stock one if start failed before
// that) and returns the non-zero exit code.
func run(start func() error) int {
	if err := start(); err != nil {
		slog.Error("api: fatal", "error", err)
		return 1
	}
	return 0
}
