package gamedata

import (
	"context"
	"fmt"

	webv1 "github.com/jeanluca/w2pp-openwyd/api/web/v1"
)

// MountGrowthCurve is one adult lineage as the panel shows it: the chance an
// âmago raises it one level, per band of twenty levels.
//
// Rates carries one entry per band, and a band nobody configured carries -1 —
// which is NOT zero. Zero is a legitimate setting: an operator deliberately
// making that band impossible. Collapsing the two would silently turn "still on
// the default" into "blocked", which is the one mistake this screen exists to
// prevent.
type MountGrowthCurve struct {
	MountIndex  int32
	DisplayName string
	CriaIndex   int32
	AmagoIndex  int32
	Configured  bool
	Rates       []int32
}

// MountGrowthCurves lists the whole roster of adult lineages, configured or not.
func (c *Client) MountGrowthCurves(ctx context.Context) ([]MountGrowthCurve, error) {
	resp, err := c.mountGrowth.ListMountGrowthCurves(ctx, &webv1.ListMountGrowthCurvesRequest{})
	if err != nil {
		return nil, fmt.Errorf("gamedata: list mount growth curves: %w", err)
	}
	out := make([]MountGrowthCurve, 0, len(resp.GetCurves()))
	for _, cur := range resp.GetCurves() {
		out = append(out, MountGrowthCurve{
			MountIndex:  cur.GetMountIndex(),
			DisplayName: cur.GetDisplayName(),
			CriaIndex:   cur.GetCriaIndex(),
			AmagoIndex:  cur.GetAmagoIndex(),
			Configured:  cur.GetConfigured(),
			Rates:       cur.GetRates(),
		})
	}
	return out, nil
}

// SetMountGrowthCurve writes one lineage's bands, all of them at once. The
// service refuses a partial list: the bands only mean anything together, and a
// save that landed half of them would leave a curve nobody chose.
func (c *Client) SetMountGrowthCurve(ctx context.Context, moderatorID int64, moderator string, mountIndex int32, rates []int32) error {
	resp, err := c.mountGrowth.SetMountGrowthCurve(ctx, &webv1.SetMountGrowthCurveRequest{
		ModeratorId: moderatorID, Moderator: moderator,
		MountIndex: mountIndex, Rates: rates,
	})
	if err != nil {
		return fmt.Errorf("gamedata: set mount growth curve %d: %w", mountIndex, err)
	}
	return resultErr(resp.GetResult())
}

// ClearMountGrowthCurve drops the lineage's rows so the compiled default applies
// again.
func (c *Client) ClearMountGrowthCurve(ctx context.Context, moderatorID int64, mountIndex int32) error {
	resp, err := c.mountGrowth.ClearMountGrowthCurve(ctx, &webv1.ClearMountGrowthCurveRequest{
		ModeratorId: moderatorID, MountIndex: mountIndex,
	})
	if err != nil {
		return fmt.Errorf("gamedata: clear mount growth curve %d: %w", mountIndex, err)
	}
	return resultErr(resp.GetResult())
}

// MountAbsorb is one lineage's absorption pair as the panel shows it: how much
// of a hit the mount eats instead of its rider, against a player and against a
// monster.
//
// Configured is a flag rather than a sentinel inside the numbers because 0 is a
// legitimate setting — a lineage deliberately made to absorb nothing on one axis
// — and collapsing the two would silently turn "still on the default" into
// "defenceless".
type MountAbsorb struct {
	MountIndex  int32
	DisplayName string
	Configured  bool
	PvP         int32
	PvE         int32
}

// MountAbsorbs lists the whole roster of adult lineages, configured or not.
func (c *Client) MountAbsorbs(ctx context.Context) ([]MountAbsorb, error) {
	resp, err := c.mountGrowth.ListMountAbsorb(ctx, &webv1.ListMountAbsorbRequest{})
	if err != nil {
		return nil, fmt.Errorf("gamedata: list mount absorb: %w", err)
	}
	out := make([]MountAbsorb, 0, len(resp.GetAbsorb()))
	for _, a := range resp.GetAbsorb() {
		out = append(out, MountAbsorb{
			MountIndex:  a.GetMountIndex(),
			DisplayName: a.GetDisplayName(),
			Configured:  a.GetConfigured(),
			PvP:         a.GetAbsorbPvp(),
			PvE:         a.GetAbsorbPve(),
		})
	}
	return out, nil
}

// SetMountAbsorb writes one lineage's two numbers, both at once. They are the
// two halves of a single decision about what the mount is for, and the service
// refuses to take one without the other.
func (c *Client) SetMountAbsorb(ctx context.Context, moderatorID int64, moderator string, mountIndex, pvp, pve int32) error {
	resp, err := c.mountGrowth.SetMountAbsorb(ctx, &webv1.SetMountAbsorbRequest{
		ModeratorId: moderatorID, Moderator: moderator,
		MountIndex: mountIndex, AbsorbPvp: pvp, AbsorbPve: pve,
	})
	if err != nil {
		return fmt.Errorf("gamedata: set mount absorb %d: %w", mountIndex, err)
	}
	return resultErr(resp.GetResult())
}

// ClearMountAbsorb drops the lineage's row so the compiled default applies again.
func (c *Client) ClearMountAbsorb(ctx context.Context, moderatorID int64, mountIndex int32) error {
	resp, err := c.mountGrowth.ClearMountAbsorb(ctx, &webv1.ClearMountAbsorbRequest{
		ModeratorId: moderatorID, MountIndex: mountIndex,
	})
	if err != nil {
		return fmt.Errorf("gamedata: clear mount absorb %d: %w", mountIndex, err)
	}
	return resultErr(resp.GetResult())
}

// MountConfigVersion is when the mount overlay last changed, as unix seconds.
// Compared with what the running game reports, it is what separates "salvo e
// valendo" from "salvo, esperando reinício" — dois estados que a tela não
// distingue de outra forma.
func (c *Client) MountConfigVersion(ctx context.Context) (int64, error) {
	resp, err := c.mountGrowth.MountConfigVersion(ctx, &webv1.MountConfigVersionRequest{})
	if err != nil {
		return 0, fmt.Errorf("gamedata: mount config version: %w", err)
	}
	return resp.GetVersion(), nil
}
