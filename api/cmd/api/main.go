// Package main is the entry point for the critical api binary.
package main

import (
	"log"

	api "github.com/ceffo/critical/api/internal/cmd/api"
)

func main() {
	if err := api.Start(); err != nil {
		log.Fatalf("api: %v", err)
	}
}
