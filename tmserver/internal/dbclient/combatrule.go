package dbclient

import (
	"context"
	"fmt"

	"google.golang.org/grpc"

	dbv1 "github.com/jeanluca/w2pp-openwyd/api/db/v1"
	"github.com/jeanluca/w2pp-openwyd/internal/combatrule"
)

// CombatRuleSource reads the combat rule (internal/combatrule) from dbServer.
//
// Polled, like the spawn pacing: the staff tune these knobs by watching a
// fight, and restart-to-apply would cost everyone online a disconnect per turn.
// The version call is the cheap half; the rule is only fetched when it moved.
type CombatRuleSource struct {
	api dbv1.CombatRuleServiceClient
}

// NewCombatRuleSource wraps a gRPC connection.
func NewCombatRuleSource(conn grpc.ClientConnInterface) *CombatRuleSource {
	return &CombatRuleSource{api: dbv1.NewCombatRuleServiceClient(conn)}
}

// Version is the poll.
func (c *CombatRuleSource) Version(ctx context.Context) (int64, error) {
	resp, err := c.api.CombatRuleVersion(ctx, &dbv1.CombatRuleVersionRequest{})
	if err != nil {
		return 0, fmt.Errorf("dbclient: combat rule version: %w", err)
	}
	return resp.GetVersion(), nil
}

// Fetch returns the rule in force. A reply that is not configured yields
// combatrule.Default() whatever its fields carry: nobody saved a rule, and the
// zero values of those fields are not one.
func (c *CombatRuleSource) Fetch(ctx context.Context) (combatrule.Config, error) {
	resp, err := c.api.GetCombatRule(ctx, &dbv1.GetCombatRuleRequest{})
	if err != nil {
		return combatrule.Config{}, fmt.Errorf("dbclient: get combat rule: %w", err)
	}
	if !resp.GetConfigured() {
		return combatrule.Unconfigured(resp.GetVersion()), nil
	}
	return combatrule.Config{
		Version:    resp.GetVersion(),
		Configured: true,
		Rules: combatrule.Rules{
			WeaponIntMagicPct: resp.GetWeaponIntMagicPct(),
			SpellDamageMulti:  resp.GetSpellDamageMulti(),
			MobResistBase:     resp.GetMobResistBase(),
		},
	}, nil
}
