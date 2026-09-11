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
	padrao := combatrule.Default()
	return combatrule.Config{
		Version:    resp.GetVersion(),
		Configured: true,
		Rules: combatrule.Rules{
			WeaponIntMagicPct:    resp.GetWeaponIntMagicPct(),
			SpellDamageMulti:     resp.GetSpellDamageMulti(),
			MobResistBase:        resp.GetMobResistBase(),
			PvPSkillPct:          pvpPct(resp.GetPvpSkillPct()),
			PvPMeleePct:          pvpPct(resp.GetPvpMeleePct()),
			SpellIntAccuracyPct:  presentOr(resp.SpellIntAccuracyPct, padrao.SpellIntAccuracyPct),
			MaxMissStreak:        presentOr(resp.MaxMissStreak, padrao.MaxMissStreak),
			WeaponDamageGrants:   presentOr(resp.WeaponDamageGrants, padrao.WeaponDamageGrants),
			DoubleCriticalMaxPct: presentOr(resp.DoubleCriticalMaxPct, padrao.DoubleCriticalMaxPct),
			PhysicalDamagePct:    presentOr(resp.PhysicalDamagePct, padrao.PhysicalDamagePct),
		},
	}, nil
}

// presentOr reads one of the `optional` fields. Unlike the PvP pair, 0 is a real
// value there — the legacy — so only absence can mean a dbServer that predates
// the field. An absent field takes the decided default rather than the legacy:
// migration 0046 gives a row saved before the columns existed that same
// default, so a rolling deploy runs exactly what the database will say once the
// dbServer catches up.
func presentOr(v *int32, def int32) int32 {
	if v == nil {
		return def
	}
	return *v
}

// legacyPvPPct is the PvP share that leaves the legacy quarter untouched — the
// same 100 migration 0045 gives a row saved before the columns existed.
const legacyPvPPct = 100

// pvpPct reads one PvP field. Zero is outside its range (1..200), so it can only
// mean a dbServer that predates the field and never sent it. Taking that as the
// legacy keeps a rolling deploy — this tmServer ahead of its dbServer — running
// the saved rule; passing the zero through would make the whole rule invalid,
// and the game would drop back to the default while the panel showed the rule.
func pvpPct(v int32) int32 {
	if v == 0 {
		return legacyPvPPct
	}
	return v
}
