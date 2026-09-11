// Package mountgrowth is the staff-panel side of the mount growth curves
// (0030_mount_growth_rate): the chance an âmago raises an ADULT mount one level,
// per lineage and per band of twenty levels.
//
// It lists the WHOLE roster of thirty adult lineages, configured or not, rather
// than only the rows someone has already saved. A screen that shows only what
// was touched cannot answer the question the operator actually has — "which
// mounts are still on the default?" — and that is the question that matters when
// balancing a set.
package mountgrowth

import (
	"context"
	"fmt"

	"github.com/jeanluca/w2pp-openwyd/internal/domain"
	"github.com/jeanluca/w2pp-openwyd/internal/mountbonus"
	"github.com/jeanluca/w2pp-openwyd/webserver/internal/itemcatalog"
)

// Store is the persistence this service needs (satisfied by *store.Store).
type Store interface {
	ListMountGrowthRates(ctx context.Context) ([]domain.MountGrowthRate, error)
	SetMountGrowthCurve(ctx context.Context, mountIndex int16, rates []int16, moderatorID int64, moderator string) error
	ClearMountGrowthCurve(ctx context.Context, mountIndex int16, moderatorID int64) error
	ListMountAbsorb(ctx context.Context) ([]domain.MountAbsorb, error)
	SetMountAbsorb(ctx context.Context, mountIndex, pvp, pve int16, moderatorID int64, moderator string) error
	ClearMountAbsorb(ctx context.Context, mountIndex int16, moderatorID int64) error
	ListMountBonus(ctx context.Context) ([]domain.MountBonus, error)
	SetMountBonus(ctx context.Context, b domain.MountBonus, moderatorID int64, moderator string) error
	ClearMountBonus(ctx context.Context, mountIndex int16, moderatorID int64) error
	MountConfigVersion(ctx context.Context) (int64, error)
}

// CatalogReader supplies a catalog entry, for the lineage name. Same shape the
// item-stat editor uses, so both are wired from one place in main.
type CatalogReader func(itemIndex int32) (itemcatalog.Entry, bool)

// Service is the panel-facing logic.
type Service struct {
	store   Store
	catalog CatalogReader
}

// New builds the service over the given store.
func New(s Store) *Service { return &Service{store: s} }

// SetCatalog installs the reader used for lineage names.
func (s *Service) SetCatalog(r CatalogReader) { s.catalog = r }

// Curve is one lineage as the panel shows it.
type Curve struct {
	MountIndex  int16
	DisplayName string
	CriaIndex   int16
	AmagoIndex  int16
	Configured  bool
	// Rates carries one entry per band; a band nobody set is Unset (-1), which
	// is NOT zero — zero is an operator deliberately making that band impossible.
	Rates [domain.MountGrowthBands]int16
}

// Unset marks a band nobody configured.
const Unset int16 = -1

// mountRowSize is the stride between the cria, adult and âmago rows.
const mountRowSize = 30

// amagoBase is where the âmago row starts.
const amagoBase = 2390

// List returns every adult lineage, in index order, with whatever is configured.
func (s *Service) List(ctx context.Context) ([]Curve, error) {
	rows, err := s.store.ListMountGrowthRates(ctx)
	if err != nil {
		return nil, fmt.Errorf("mountgrowth: list: %w", err)
	}
	byMount := make(map[int16][domain.MountGrowthBands]int16, len(rows)/domain.MountGrowthBands+1)
	for _, r := range rows {
		if r.Band < 0 || int(r.Band) >= domain.MountGrowthBands {
			continue
		}
		cur, ok := byMount[r.MountIndex]
		if !ok {
			cur = unsetRates()
		}
		cur[r.Band] = r.Rate
		byMount[r.MountIndex] = cur
	}

	out := make([]Curve, 0, domain.MountAdultHi-domain.MountAdultLo+1)
	for idx := int16(domain.MountAdultLo); idx <= domain.MountAdultHi; idx++ {
		rates, configured := byMount[idx]
		if !configured {
			rates = unsetRates()
		}
		out = append(out, Curve{
			MountIndex:  idx,
			DisplayName: s.name(idx),
			CriaIndex:   idx - mountRowSize,
			AmagoIndex:  amagoFor(idx),
			Configured:  configured,
			Rates:       rates,
		})
	}
	return out, nil
}

// Set writes one lineage's whole curve.
func (s *Service) Set(ctx context.Context, moderatorID int64, moderator string, mountIndex int16, rates []int16) error {
	if err := s.store.SetMountGrowthCurve(ctx, mountIndex, rates, moderatorID, moderator); err != nil {
		return fmt.Errorf("mountgrowth: set %d: %w", mountIndex, err)
	}
	return nil
}

// Clear drops the lineage's rows so the compiled default applies again.
func (s *Service) Clear(ctx context.Context, moderatorID int64, mountIndex int16) error {
	if err := s.store.ClearMountGrowthCurve(ctx, mountIndex, moderatorID); err != nil {
		return fmt.Errorf("mountgrowth: clear %d: %w", mountIndex, err)
	}
	return nil
}

func (s *Service) name(index int16) string {
	if s.catalog == nil {
		return ""
	}
	if e, ok := s.catalog(int32(index)); ok {
		return e.DisplayName
	}
	return ""
}

// amagoFor is the âmago that feeds a mount: its own row. The legacy fed the
// Svadilfari and the Sleipnir with another lineage's âmago; this server does
// not (tmserver/internal/handler/amago.go, mountAmagoSlot), and the screen
// names the item the game actually accepts.
func amagoFor(mountIndex int16) int16 {
	return int16(amagoBase + (int(mountIndex)-domain.MountAdultLo)%mountRowSize)
}

func unsetRates() [domain.MountGrowthBands]int16 {
	var r [domain.MountGrowthBands]int16
	for i := range r {
		r[i] = Unset
	}
	return r
}

// BandLabel names a band the way the screen shows it: "1 – 20", "101 – 120".
func BandLabel(band int) string {
	lo := band*domain.MountGrowthBandSize + 1
	hi := lo + domain.MountGrowthBandSize - 1
	return fmt.Sprintf("%d – %d", lo, hi)
}

// AmagosToCap estimates how many âmagos a curve costs to reach the cap, which is
// the number that makes a percentage mean something: 45% and 70% do not read as
// "somewhat harder" until they read as 353 against 188.
//
// The expected gain per feed is rate - 0.2*(1-rate), because one failure in five
// costs a level. Below roughly 17% that gain turns negative and the mount never
// arrives at all — reported as false so the screen can say so instead of showing
// an enormous number that looks merely expensive.
func AmagosToCap(rates [domain.MountGrowthBands]int16, defaultRate int16) (int, bool) {
	total := 0.0
	for _, r := range rates {
		if r == Unset {
			r = defaultRate
		}
		p := float64(r) / 100
		gain := p - 0.2*(1-p)
		if gain <= 0 {
			return 0, false
		}
		total += float64(domain.MountGrowthBandSize) / gain
	}
	return int(total + 0.5), true
}

// Absorb is one lineage's absorption pair as the panel shows it: how much of a
// hit the mount eats instead of its rider, against a player and against a
// monster.
//
// Configured is a flag and not a sentinel inside the numbers because 0 is a
// legitimate setting — a lineage deliberately made to absorb nothing on one axis
// — and collapsing the two would turn "still on the default" into "defenceless".
type Absorb struct {
	MountIndex  int16
	DisplayName string
	Configured  bool
	PvP         int16
	PvE         int16
}

// ListAbsorb returns every adult lineage, configured or not, in index order —
// the same whole-roster answer ListCurves gives, for the same reason: the
// question is "which mounts are still on the default?".
func (s *Service) ListAbsorb(ctx context.Context) ([]Absorb, error) {
	rows, err := s.store.ListMountAbsorb(ctx)
	if err != nil {
		return nil, fmt.Errorf("mountgrowth: list absorb: %w", err)
	}
	byMount := make(map[int16]domain.MountAbsorb, len(rows))
	for _, r := range rows {
		byMount[r.MountIndex] = r
	}

	out := make([]Absorb, 0, domain.MountAdultHi-domain.MountAdultLo+1)
	for idx := int16(domain.MountAdultLo); idx <= domain.MountAdultHi; idx++ {
		a := Absorb{MountIndex: idx, DisplayName: s.name(idx)}
		if row, ok := byMount[idx]; ok {
			a.Configured, a.PvP, a.PvE = true, row.PvP, row.PvE
		}
		out = append(out, a)
	}
	return out, nil
}

// SetAbsorb writes one lineage's pair.
func (s *Service) SetAbsorb(ctx context.Context, moderatorID int64, moderator string, mountIndex, pvp, pve int16) error {
	if err := s.store.SetMountAbsorb(ctx, mountIndex, pvp, pve, moderatorID, moderator); err != nil {
		return fmt.Errorf("mountgrowth: set absorb %d: %w", mountIndex, err)
	}
	return nil
}

// ClearAbsorb drops the lineage's row so the compiled default applies again.
func (s *Service) ClearAbsorb(ctx context.Context, moderatorID int64, mountIndex int16) error {
	if err := s.store.ClearMountAbsorb(ctx, mountIndex, moderatorID); err != nil {
		return fmt.Errorf("mountgrowth: clear absorb %d: %w", mountIndex, err)
	}
	return nil
}

// ConfigVersion is when the mount overlay last changed, as unix seconds. The
// panel holds it against the number the running game reports, which is the only
// way the screen can say whether what it shows is what players are getting.
func (s *Service) ConfigVersion(ctx context.Context) (int64, error) {
	v, err := s.store.MountConfigVersion(ctx)
	if err != nil {
		return 0, fmt.Errorf("mountgrowth: config version: %w", err)
	}
	return v, nil
}

// Bonus is one lineage's attributes as the panel shows it: what is in effect,
// and beside it the compiled default — the client's own numbers — so the screen
// can say what a lineage had before anyone touched it.
type Bonus struct {
	MountIndex  int16
	DisplayName string
	Configured  bool
	Current     mountbonus.Bonus
	Default     mountbonus.Bonus
}

// ListBonus returns every adult lineage, configured or not, in index order.
func (s *Service) ListBonus(ctx context.Context) ([]Bonus, error) {
	rows, err := s.store.ListMountBonus(ctx)
	if err != nil {
		return nil, fmt.Errorf("mountgrowth: list bonus: %w", err)
	}
	byMount := make(map[int16]domain.MountBonus, len(rows))
	for _, r := range rows {
		byMount[r.MountIndex] = r
	}

	out := make([]Bonus, 0, mountbonus.AdultHi-mountbonus.AdultLo+1)
	for idx := int16(mountbonus.AdultLo); idx <= mountbonus.AdultHi; idx++ {
		def, _ := mountbonus.Default(idx)
		b := Bonus{MountIndex: idx, DisplayName: s.name(idx), Current: def, Default: def}
		if row, ok := byMount[idx]; ok {
			b.Configured = true
			b.Current = mountbonus.Bonus{Attack: row.Attack, Magic: row.Magic, Evasion: row.Evasion, Resist: row.Resist}
		}
		out = append(out, b)
	}
	return out, nil
}

// SetBonus writes one lineage's four numbers.
func (s *Service) SetBonus(ctx context.Context, moderatorID int64, moderator string, mountIndex int16, b mountbonus.Bonus) error {
	row := domain.MountBonus{MountIndex: mountIndex, Attack: b.Attack, Magic: b.Magic, Evasion: b.Evasion, Resist: b.Resist}
	if err := s.store.SetMountBonus(ctx, row, moderatorID, moderator); err != nil {
		return fmt.Errorf("mountgrowth: set bonus %d: %w", mountIndex, err)
	}
	return nil
}

// ClearBonus drops the lineage's row so the compiled table applies again.
func (s *Service) ClearBonus(ctx context.Context, moderatorID int64, mountIndex int16) error {
	if err := s.store.ClearMountBonus(ctx, mountIndex, moderatorID); err != nil {
		return fmt.Errorf("mountgrowth: clear bonus %d: %w", mountIndex, err)
	}
	return nil
}
