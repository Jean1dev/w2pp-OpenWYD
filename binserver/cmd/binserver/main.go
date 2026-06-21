// Command binserver is the billing service (BISrv), designed from scratch as a
// gRPC service with its own API and policy.
//
// Placeholder entrypoint — implemented in Phase 6 (migration-plan.md §4), and
// only after the _AUTH_GAME 196-byte layout is confirmed by capture
// (protocol-spec.md §4.3, UNVERIFIED). It compiles so `go build ./...` is green.
package main

import (
	"log/slog"
	"os"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	logger.Info("binserver placeholder — implemented in Phase 6")
}
