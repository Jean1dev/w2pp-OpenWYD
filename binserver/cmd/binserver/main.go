// Command binserver is the billing service (BISrv), designed from scratch as a
// gRPC service with its own API (api/bin/v1) and policy
// (binserver/internal/billing). The legacy _AUTH_GAME 196-byte link is UNVERIFIED
// and intentionally not reproduced (migration-plan.md §4, Fase 6).
//
// Phase 6 ships the billing policy and contract. The gRPC server is wired once
// the proto is generated (`make proto`); until then this entrypoint constructs
// the policy so the wiring is exercised.
package main

import (
	"log/slog"
	"os"

	"github.com/jeanluca/w2pp-openwyd/binserver/internal/billing"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Free-to-play default: accounts without a record are allowed. Set to false
	// (and load records) to require an active subscription.
	svc := billing.New(true)
	_ = svc // TODO(Phase 6): serve api/bin/v1.BillingService over gRPC + mTLS.

	logger.Info("binserver billing policy ready",
		"policy", "free-to-play (allow unknown)",
		"note", "gRPC server pending `make proto` codegen")
}
