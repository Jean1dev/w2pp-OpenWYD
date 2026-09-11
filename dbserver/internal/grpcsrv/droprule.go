package grpcsrv

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	dbv1 "github.com/jeanluca/w2pp-openwyd/api/db/v1"
	"github.com/jeanluca/w2pp-openwyd/internal/droprule"
)

// DropRuleStore is the read surface tmServer needs (satisfied by *store.Store).
// Staff writes go through the admin panel.
type DropRuleStore interface {
	DropRuleVersion(ctx context.Context) (int64, error)
	DropRules(ctx context.Context) (droprule.Config, error)
}

// DropRuleServer implements dbv1.DropRuleServiceServer.
type DropRuleServer struct {
	dbv1.UnimplementedDropRuleServiceServer
	store DropRuleStore
}

// NewDropRule builds the service over the given store.
func NewDropRule(s DropRuleStore) *DropRuleServer { return &DropRuleServer{store: s} }

// DropRuleVersion is the poll tmServer runs every few seconds.
func (s *DropRuleServer) DropRuleVersion(ctx context.Context, _ *dbv1.DropRuleVersionRequest) (*dbv1.DropRuleVersionResponse, error) {
	v, err := s.store.DropRuleVersion(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "drop rule version: %v", err)
	}
	return &dbv1.DropRuleVersionResponse{Version: v}, nil
}

// ListDropRules returns every rule and the version they belong to.
func (s *DropRuleServer) ListDropRules(ctx context.Context, _ *dbv1.ListDropRulesRequest) (*dbv1.ListDropRulesResponse, error) {
	cfg, err := s.store.DropRules(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "drop rules: %v", err)
	}
	out := &dbv1.ListDropRulesResponse{Version: cfg.Version, Rules: make([]*dbv1.DropRule, 0, len(cfg.Rules))}
	for _, r := range cfg.Rules {
		out.Rules = append(out.Rules, &dbv1.DropRule{Mob: r.Mob, Item: int32(r.Item), Chance: r.Chance})
	}
	return out, nil
}
