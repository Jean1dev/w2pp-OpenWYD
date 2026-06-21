// Package migrations embeds the versioned SQL migration files so they ship in
// the dbserver binary (no external migration tool needed at runtime).
package migrations

import "embed"

// FS holds all *.up.sql / *.down.sql migration files (data-formats.md §4).
//
//go:embed *.sql
var FS embed.FS
