package gamedata

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	webv1 "github.com/jeanluca/w2pp-openwyd/api/web/v1"
)

// Drop is one catalog item and every mob slot it falls from.
type Drop struct {
	ItemIndex int32
	ItemName  string
	Mobs      []DropMob
}

// DropMob is one mob and slot an item drops from, with the odds.
type DropMob struct {
	TemplateName string
	MobName      string
	MobLevel     int32
	Slot         int32

	// Divisor is the number the game rolls against: one kill in Divisor drops
	// the item. It is the EFFECTIVE value, already scaled by the mob level and
	// with the four hard slot overrides applied — not the raw table entry, which
	// would report the guaranteed slot 11 as a one-in-four rarity.
	Divisor int32

	// Origens is where NPCGener spawns this template, busiest first.
	// OrigensLidas is false when the webServer had no spawn index: an empty
	// Origens then means "not known", and only with it true does it mean that no
	// generator spawns the mob — a drop no player meets in the open world.
	Origens      []MobOrigem
	OrigensLidas bool
}

// NasceEmGerador reports whether some generator spawns the mob. Only
// meaningful with OrigensLidas.
func (m DropMob) NasceEmGerador() bool { return len(m.Origens) > 0 }

// OutrosLugares is how many places beyond the first (the busiest) spawn it.
func (m DropMob) OutrosLugares() int { return max(len(m.Origens)-1, 0) }

// Lugares lists every place, for the tooltip behind "e mais N".
func (m DropMob) Lugares() string {
	var b strings.Builder
	for i, o := range m.Origens {
		if i > 0 {
			b.WriteString("; ")
		}
		fmt.Fprintf(&b, "%s (%d, %d)", o.Local, o.X, o.Y)
	}
	return b.String()
}

// Mortes is how many of this mob you kill, on average, for one drop.
//
// It is the divisor itself: for a roll that succeeds with probability 1/d, the
// expected number of attempts is exactly d. Reported as-is rather than rounded,
// because the number is already whole.
func (m DropMob) Mortes() int32 { return m.Divisor }

// Chance is the per-kill chance as a percentage.
func (m DropMob) Chance() float64 {
	if m.Divisor <= 0 {
		return 100 // a non-positive rate always drops (see loot.Drops)
	}
	return 100 / float64(m.Divisor)
}

// Porcentagem formats Chance for a table, keeping small odds readable.
//
// A fixed number of decimals does not work across this range: the rarest slots
// sit near 0.005%, and two decimals would render every one of them as "0.01%" or
// "0.00%". The decimals therefore follow the size of the number, which is what a
// person comparing two rows actually needs.
func (m DropMob) Porcentagem() string {
	p := m.Chance()
	switch {
	case p >= 100:
		return "100%"
	case p >= 10:
		return strconv.FormatFloat(p, 'f', 1, 64) + "%"
	case p >= 1:
		return strconv.FormatFloat(p, 'f', 2, 64) + "%"
	case p >= 0.1:
		return strconv.FormatFloat(p, 'f', 3, 64) + "%"
	default:
		return strconv.FormatFloat(p, 'f', 4, 64) + "%"
	}
}

// Garantido reports whether the item always drops from this slot. Worth marking
// on its own, because it is the one row where the raw table is most misleading.
func (m DropMob) Garantido() bool { return m.Divisor <= 1 }

// Drops returns the item-centric drop report, filtered by item and mob name.
//
// Both filters are substring matches the webServer applies; an empty pair asks
// for everything, which is thousands of rows, so the caller is expected to cap
// what it renders.
func (c *Client) Drops(ctx context.Context, moderatorID int64, item, mob string) ([]Drop, error) {
	resp, err := c.npc.ListDropItems(ctx, &webv1.ListDropItemsRequest{
		ModeratorId: moderatorID, ItemQuery: item, MobQuery: mob,
	})
	if err != nil {
		return nil, fmt.Errorf("gamedata: list drops: %w", err)
	}
	if err := resultErr(resp.GetResult()); err != nil {
		return nil, err
	}
	out := make([]Drop, 0, len(resp.GetItems()))
	for _, it := range resp.GetItems() {
		d := Drop{ItemIndex: it.GetItemIndex(), ItemName: it.GetItemName()}
		for _, m := range it.GetMobs() {
			dm := DropMob{
				TemplateName: m.GetTemplateName(),
				MobName:      m.GetMobName(),
				MobLevel:     m.GetMobLevel(),
				Slot:         m.GetSlot(),
				Divisor:      m.GetEffectiveDivisor(),
				OrigensLidas: resp.GetOriginsKnown(),
			}
			for _, o := range m.GetOrigins() {
				dm.Origens = append(dm.Origens, MobOrigem{
					Local: o.GetPlace(), Pontos: o.GetPoints(), Quantidade: o.GetAmount(),
					RespawnMin: o.GetRespawnMinutes(), X: o.GetX(), Y: o.GetY(),
				})
			}
			d.Mobs = append(d.Mobs, dm)
		}
		out = append(out, d)
	}
	return out, nil
}
