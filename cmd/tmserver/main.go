// Command tmserver is the WYD game server (TMSrv): it speaks the legacy CPSock
// wire protocol to the unmodified WYD.exe 7662 client and owns the in-memory
// world state.
//
// This entrypoint only does wiring (guidelines §3): flags, logging and graceful
// shutdown. The world game-loop and the CPSock listener are wired in Phase 3
// (migration-plan.md §4); Phase 1 delivers the protocol codec in
// internal/protocol.
package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	addr := flag.String("addr", ":8281", "CPSock listen address for the client edge")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	logger.Info("tmserver starting", "addr", *addr, "phase", "1 (protocol codec only)")

	// TODO(Phase 3): bind the CPSock listener (internal/protocol) and start the
	// world game-loop (1 owner goroutine + channels). For now we only honour the
	// shutdown signal so the binary is a valid, testable wiring shell.
	<-ctx.Done()

	logger.Info("tmserver shutting down")
}
