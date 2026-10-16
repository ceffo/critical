// Package postgres is the PostgreSQL storage (ADR 0025): the schema migrations the server applies
// at boot. The baseline, 001_baseline.sql, is the schema the SQLite migrations built.
package postgres

import "embed"

// migrationFiles are the schema migrations, applied in lexical order. Schema only: no migration
// holds an INSERT, UPDATE or DELETE (ADR 0025 step 2.1).
//
//go:embed migrations/*.sql
var migrationFiles embed.FS
