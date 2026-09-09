package dbclient

import (
	"context"
	"fmt"

	"google.golang.org/grpc"

	dbv1 "github.com/jeanluca/w2pp-openwyd/api/db/v1"
	"github.com/jeanluca/w2pp-openwyd/internal/level"
)

// XPConfigSource fetches the Mesa de XP — the panel-managed reward tables — from
// dbServer's XPConfigService.
//
// It is read at boot and then polled, like the spawn pacing and the world-event
// config next to it. That is a deliberate reversal of the earlier decision here,
// which was restart-to-apply on the grounds that swapping the tables mid-flight
// pays two players different experience for the same mob depending on when each
// kill landed. That objection is true and it is not worth what it costs:
//
//   - The same unfairness already exists and is already shipped. Double-exp and
//     the newbie event reload live (worldevent.go), and they move the very same
//     number by the very same kind of step. Nobody has ever proposed restarting
//     the server to start an event.
//   - The Mesa is the main dial for the server's pace, so it is the one that
//     gets turned repeatedly while somebody watches whether the pace came out
//     right. Restart-to-apply makes every one of those turns cost a disconnect
//     for everybody online, which is a far larger unfairness than two kills
//     paying differently, and it also makes backing out of a bad value slow at
//     the exact moment being slow hurts.
//
// So: the tables move live, and a mistake can be undone in fifteen seconds.
type XPConfigSource struct {
	api dbv1.XPConfigServiceClient
}

// NewXPConfigSource wraps a gRPC connection as an XPConfigSource.
func NewXPConfigSource(conn grpc.ClientConnInterface) *XPConfigSource {
	return &XPConfigSource{api: dbv1.NewXPConfigServiceClient(conn)}
}

// Version returns the monotonic version of the saved tables, which is what the
// poll compares against so a reload only costs a full read when something moved.
func (c *XPConfigSource) Version(ctx context.Context) (int64, error) {
	resp, err := c.api.XPConfigVersion(ctx, &dbv1.XPConfigVersionRequest{})
	if err != nil {
		return 0, fmt.Errorf("dbclient: xp config version: %w", err)
	}
	return resp.GetVersion(), nil
}

// Fetch returns the configuration ready for level.ExpReward. A reply with no
// rules yields a zero Config, which is the pure legacy behaviour.
func (c *XPConfigSource) Fetch(ctx context.Context) (level.Config, error) {
	resp, err := c.api.GetXPConfig(ctx, &dbv1.GetXPConfigRequest{})
	if err != nil {
		return level.Config{}, fmt.Errorf("dbclient: get xp config: %w", err)
	}
	cfg := level.Config{Version: resp.GetVersion()}
	for _, r := range resp.GetRules() {
		ov := level.Override{RatePercent: r.GetRatePercent()}
		if r.GetHasCuts() {
			// Non-nil even when empty: an edited branch with no cuts divides by
			// nothing, and a nil slice would silently mean "use the legacy's".
			ov.Cuts = make([]level.Cut, 0, len(r.GetCuts()))
			for _, c := range r.GetCuts() {
				ov.Cuts = append(ov.Cuts, level.Cut{UpTo: c.GetUpTo(), Divisor: c.GetDivisor()})
			}
		}
		if cfg.Overrides == nil {
			cfg.Overrides = make(map[level.ConfigKey]level.Override, len(resp.GetRules()))
		}
		cfg.Overrides[level.ConfigKey{Zone: level.Zone(r.GetZone()), Tier: uint8(r.GetTier())}] = ov
	}
	return cfg, nil
}
