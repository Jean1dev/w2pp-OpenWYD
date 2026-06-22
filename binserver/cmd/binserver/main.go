// Command binserver is the billing service (BISrv), designed from scratch as a
// gRPC service with its own API (api/bin/v1) and policy
// (binserver/internal/billing). The legacy _AUTH_GAME 196-byte link is UNVERIFIED
// and intentionally not reproduced (migration-plan.md §4, Fase 6).
//
// Usage:
//
//	binserver [-addr :3000] [-allow-unknown] [-tls-cert … -tls-key … -tls-ca …]
//
// It serves the BillingService over gRPC (+mTLS when cert paths are set) until
// SIGINT/SIGTERM.
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

	"google.golang.org/grpc"

	binv1 "github.com/jeanluca/w2pp-openwyd/api/bin/v1"
	"github.com/jeanluca/w2pp-openwyd/binserver/internal/billing"
	"github.com/jeanluca/w2pp-openwyd/binserver/internal/grpcsrv"
	"github.com/jeanluca/w2pp-openwyd/internal/secure"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("binserver stopped with error", "err", err)
		os.Exit(1)
	}
	logger.Info("binserver stopped")
}

func run(logger *slog.Logger) error {
	addr := flag.String("addr", ":3000", "gRPC listen address")
	allowUnknown := flag.Bool("allow-unknown", true, "free-to-play: allow accounts with no billing record")
	tlsCert := flag.String("tls-cert", os.Getenv("W2PP_TLS_CERT"), "server certificate (PEM)")
	tlsKey := flag.String("tls-key", os.Getenv("W2PP_TLS_KEY"), "server private key (PEM)")
	tlsCA := flag.String("tls-ca", os.Getenv("W2PP_TLS_CA"), "client CA (PEM) for mTLS")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	creds, err := secure.ServerCreds(secure.Config{CertFile: *tlsCert, KeyFile: *tlsKey, CAFile: *tlsCA})
	if err != nil {
		return err
	}
	srv := grpc.NewServer(grpc.Creds(creds))
	binv1.RegisterBillingServiceServer(srv, grpcsrv.New(billing.New(*allowUnknown), nil))

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		return err
	}
	logger.Info("binserver serving", "addr", *addr, "allow_unknown", *allowUnknown, "mtls", *tlsCert != "")

	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(ln) }()

	select {
	case <-ctx.Done():
		logger.Info("shutting down")
		srv.GracefulStop()
		return nil
	case err := <-errCh:
		if err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			return err
		}
		return nil
	}
}
