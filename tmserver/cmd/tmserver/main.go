// Command tmserver is the WYD game server (TMSrv): it speaks the legacy CPSock
// wire protocol to the unmodified WYD.exe 7662 client (tmserver/internal/protocol)
// and owns the in-memory world state through a single game-loop goroutine
// (tmserver/internal/world).
//
// This entrypoint only does wiring (guidelines §3): flags, logging, the
// listener and graceful shutdown. Message handlers (login, movement, combat, …)
// are wired in Phase 4; persistence is a no-op until the dbServer gRPC client
// adapter lands.
package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/handler"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

func main() {
	addr := flag.String("addr", ":8281", "CPSock listen address for the client edge")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		logger.Error("listen failed", "addr", *addr, "err", err)
		os.Exit(1)
	}
	logger.Info("tmserver listening", "addr", *addr)

	// Phase 4 batch 1: login/character dispatch installed. Persistence is still
	// NopPersistence until the dbServer gRPC client adapter lands, so logins
	// report "no account" — enough to bring the edge up end to end.
	persist := world.NopPersistence{}
	dispatch := handler.New(handler.Config{Log: logger})
	w := world.New(world.Config{}, logger, persist, dispatch.Handle)
	if err := w.Serve(ctx, ln); err != nil && !errors.Is(err, context.Canceled) {
		logger.Error("server stopped with error", "err", err)
		os.Exit(1)
	}
	logger.Info("tmserver stopped")
}
