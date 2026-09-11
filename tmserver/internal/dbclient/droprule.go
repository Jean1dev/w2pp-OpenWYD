package dbclient

import (
	"context"
	"fmt"

	"google.golang.org/grpc"

	dbv1 "github.com/jeanluca/w2pp-openwyd/api/db/v1"
	"github.com/jeanluca/w2pp-openwyd/internal/droprule"
)

// DropRuleSource reads the Mesa de Drops (internal/droprule) from dbServer.
//
// Polled, like the combat rule: the table is built before launch by watching
// the game, and restart-to-apply would cost a disconnect per line. The version
// call is the cheap half; the rules are fetched only when it moved.
type DropRuleSource struct {
	api dbv1.DropRuleServiceClient
}

// NewDropRuleSource wraps a gRPC connection.
func NewDropRuleSource(conn grpc.ClientConnInterface) *DropRuleSource {
	return &DropRuleSource{api: dbv1.NewDropRuleServiceClient(conn)}
}

// Version is the poll.
func (c *DropRuleSource) Version(ctx context.Context) (int64, error) {
	resp, err := c.api.DropRuleVersion(ctx, &dbv1.DropRuleVersionRequest{})
	if err != nil {
		return 0, fmt.Errorf("dbclient: drop rule version: %w", err)
	}
	return resp.GetVersion(), nil
}

// Fetch returns every rule and the version they belong to.
func (c *DropRuleSource) Fetch(ctx context.Context) (droprule.Config, error) {
	resp, err := c.api.ListDropRules(ctx, &dbv1.ListDropRulesRequest{})
	if err != nil {
		return droprule.Config{}, fmt.Errorf("dbclient: list drop rules: %w", err)
	}
	cfg := droprule.Config{Version: resp.GetVersion(), Rules: make([]droprule.Rule, 0, len(resp.GetRules()))}
	for _, r := range resp.GetRules() {
		cfg.Rules = append(cfg.Rules, droprule.Rule{Mob: r.GetMob(), Item: int16(r.GetItem()), Chance: r.GetChance()})
	}
	return cfg, nil
}
