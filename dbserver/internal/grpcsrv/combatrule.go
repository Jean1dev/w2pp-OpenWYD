package grpcsrv

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	dbv1 "github.com/jeanluca/w2pp-openwyd/api/db/v1"
	"github.com/jeanluca/w2pp-openwyd/internal/combatrule"
)

// CombatRuleStore is the read surface tmServer needs (satisfied by
// *store.Store). Staff writes go through the admin panel.
type CombatRuleStore interface {
	CombatRuleVersion(ctx context.Context) (int64, error)
	CombatRule(ctx context.Context) (combatrule.Config, error)
}

// CombatRuleServer implements dbv1.CombatRuleServiceServer.
type CombatRuleServer struct {
	dbv1.UnimplementedCombatRuleServiceServer
	store CombatRuleStore
}

// NewCombatRule builds the service over the given store.
func NewCombatRule(s CombatRuleStore) *CombatRuleServer { return &CombatRuleServer{store: s} }

// CombatRuleVersion is the poll tmServer runs every few seconds.
func (s *CombatRuleServer) CombatRuleVersion(ctx context.Context, _ *dbv1.CombatRuleVersionRequest) (*dbv1.CombatRuleVersionResponse, error) {
	v, err := s.store.CombatRuleVersion(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "combat rule version: %v", err)
	}
	return &dbv1.CombatRuleVersionResponse{Version: v}, nil
}

// GetCombatRule returns the rule and the version it belongs to.
func (s *CombatRuleServer) GetCombatRule(ctx context.Context, _ *dbv1.GetCombatRuleRequest) (*dbv1.GetCombatRuleResponse, error) {
	cfg, err := s.store.CombatRule(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "combat rule: %v", err)
	}
	return &dbv1.GetCombatRuleResponse{
		Version:           cfg.Version,
		Configured:        cfg.Configured,
		WeaponIntMagicPct: cfg.Rules.WeaponIntMagicPct,
		SpellDamageMulti:  cfg.Rules.SpellDamageMulti,
		MobResistBase:     cfg.Rules.MobResistBase,
		PvpSkillPct:       cfg.Rules.PvPSkillPct,
		PvpMeleePct:       cfg.Rules.PvPMeleePct,
		// Always present, even at 0: presence is how tmServer tells this
		// dbServer from one that predates the fields (see the proto).
		SpellIntAccuracyPct: proto.Int32(cfg.Rules.SpellIntAccuracyPct),
		MaxMissStreak:       proto.Int32(cfg.Rules.MaxMissStreak),
	}, nil
}
