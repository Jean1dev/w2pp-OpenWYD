// Package grpcsrv implements the binServer's gRPC BillingService (api/bin/v1)
// over the billing policy. This is the new, clean billing boundary tmServer
// calls at character-login; the legacy _AUTH_GAME link is not reproduced
// (migration-plan.md §4, Fase 6).
package grpcsrv

import (
	"context"
	"time"

	binv1 "github.com/jeanluca/w2pp-openwyd/api/bin/v1"
	"github.com/jeanluca/w2pp-openwyd/binserver/internal/billing"
)

// Clock returns the current time; injectable for deterministic tests.
type Clock func() time.Time

// Server implements binv1.BillingServiceServer.
type Server struct {
	binv1.UnimplementedBillingServiceServer
	svc *billing.Service
	now Clock
}

// New builds a BillingService over the given policy. A nil clock uses time.Now.
func New(svc *billing.Service, clock Clock) *Server {
	if clock == nil {
		clock = time.Now
	}
	return &Server{svc: svc, now: clock}
}

// CheckBilling decides whether an account may enter the world.
func (s *Server) CheckBilling(_ context.Context, req *binv1.CheckBillingRequest) (*binv1.CheckBillingResponse, error) {
	d := s.svc.Check(req.GetAccountName(), s.now())
	return &binv1.CheckBillingResponse{
		Allowed: d.Allowed,
		Status:  statusToProto(d.Status),
	}, nil
}

func statusToProto(s billing.Status) binv1.Status {
	switch s {
	case billing.StatusActive:
		return binv1.Status_STATUS_ACTIVE
	case billing.StatusBlocked:
		return binv1.Status_STATUS_BLOCKED
	case billing.StatusExpired:
		return binv1.Status_STATUS_EXPIRED
	default:
		return binv1.Status_STATUS_UNSPECIFIED
	}
}
