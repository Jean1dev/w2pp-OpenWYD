// Command dbserver is the persistence service (DBSrv): a gRPC service backed by
// PostgreSQL that owns accounts, characters, ranking and the one-shot account
// file converter.
//
// Placeholder entrypoint — implemented in Phase 2 (migration-plan.md §4,
// data-formats.md). It compiles so `go build ./...` is green from Phase 1.
package main

import (
	"log/slog"
	"os"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	logger.Info("dbserver placeholder — implemented in Phase 2")
}
