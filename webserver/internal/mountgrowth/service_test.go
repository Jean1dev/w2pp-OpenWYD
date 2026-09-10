package mountgrowth

import (
	"context"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/internal/domain"
)

type fakeStore struct {
	rows   []domain.MountGrowthRate
	absorb []domain.MountAbsorb
	bonus  []domain.MountBonus
	salvo  []domain.MountAbsorb
	versao int64
	limpo  []int16
}

func (f *fakeStore) ListMountGrowthRates(context.Context) ([]domain.MountGrowthRate, error) {
	return f.rows, nil
}
func (f *fakeStore) SetMountGrowthCurve(context.Context, int16, []int16, int64, string) error {
	return nil
}
func (f *fakeStore) ClearMountGrowthCurve(context.Context, int16, int64) error { return nil }

func (f *fakeStore) ListMountAbsorb(context.Context) ([]domain.MountAbsorb, error) {
	return f.absorb, nil
}
func (f *fakeStore) SetMountAbsorb(_ context.Context, mountIndex, pvp, pve int16, _ int64, _ string) error {
	f.salvo = append(f.salvo, domain.MountAbsorb{MountIndex: mountIndex, PvP: pvp, PvE: pve})
	return nil
}
func (f *fakeStore) MountConfigVersion(context.Context) (int64, error) { return f.versao, nil }
func (f *fakeStore) ClearMountAbsorb(_ context.Context, mountIndex int16, _ int64) error {
	f.limpo = append(f.limpo, mountIndex)
	return nil
}

// A absorção também lista a linhagem inteira, e uma linhagem sem linha volta
// como não configurada — nunca como zero, que é uma escolha legítima e oposta.
func TestListAbsorbMarksTheUnconfigured(t *testing.T) {
	s := New(&fakeStore{absorb: []domain.MountAbsorb{{MountIndex: 2370, PvP: 60, PvE: 10}}})
	rows, err := s.ListAbsorb(context.Background())
	if err != nil {
		t.Fatalf("ListAbsorb: %v", err)
	}
	if len(rows) != domain.MountAdultHi-domain.MountAdultLo+1 {
		t.Fatalf("veio %d linhagens, want %d", len(rows), domain.MountAdultHi-domain.MountAdultLo+1)
	}
	for _, r := range rows {
		switch r.MountIndex {
		case 2370:
			if !r.Configured || r.PvP != 60 || r.PvE != 10 {
				t.Errorf("2370 = %+v, want configurada 60/10", r)
			}
		default:
			if r.Configured {
				t.Errorf("%d veio configurada sem ter linha: %+v", r.MountIndex, r)
			}
		}
	}
}

// The roster is always whole: an operator balancing a set needs to see which
// lineages are still on the default, and a list of only the saved rows cannot
// answer that.
func TestListReturnsEveryLineage(t *testing.T) {
	s := New(&fakeStore{rows: []domain.MountGrowthRate{{MountIndex: 2370, Band: 0, Rate: 60}}})
	curves, err := s.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(curves) != domain.MountAdultHi-domain.MountAdultLo+1 {
		t.Fatalf("got %d lineages, want the whole roster of 30", len(curves))
	}
	var andaluz Curve
	for _, c := range curves {
		if c.MountIndex == 2370 {
			andaluz = c
		}
	}
	if !andaluz.Configured {
		t.Error("2370 has a saved band and must read as configured")
	}
	if andaluz.Rates[0] != 60 {
		t.Errorf("band 0 = %d, want 60", andaluz.Rates[0])
	}
	// The bands nobody saved stay Unset rather than reading as zero.
	if andaluz.Rates[1] != Unset {
		t.Errorf("band 1 = %d, want Unset", andaluz.Rates[1])
	}
	if curves[0].Configured {
		t.Error("a lineage with no rows must not read as configured")
	}
}

// Every lineage points at the cria it grows from and the âmago that feeds it —
// and for two of them that âmago belongs to another lineage.
func TestCurveIdentifiesCriaAndAmago(t *testing.T) {
	s := New(&fakeStore{})
	curves, _ := s.List(context.Background())
	byIndex := map[int16]Curve{}
	for _, c := range curves {
		byIndex[c.MountIndex] = c
	}
	if got := byIndex[2370].CriaIndex; got != 2340 {
		t.Errorf("Andaluz cria = %d, want 2340", got)
	}
	if got := byIndex[2370].AmagoIndex; got != 2400 {
		t.Errorf("Andaluz âmago = %d, want 2400", got)
	}
	// Sleipnir (slot 28) is fed by slot 21's âmago, Svadilfari (27) by slot 10's.
	if got := byIndex[2388].AmagoIndex; got != 2390+21 {
		t.Errorf("Sleipnir âmago = %d, want %d", got, 2390+21)
	}
	if got := byIndex[2387].AmagoIndex; got != 2390+10 {
		t.Errorf("Svadilfari âmago = %d, want %d", got, 2390+10)
	}
}

// The cost is what makes a percentage mean something, and the impossible case
// must be reported as impossible rather than as a very large number.
func TestAmagosToCap(t *testing.T) {
	var easy [domain.MountGrowthBands]int16
	for i := range easy {
		easy[i] = 85
	}
	got, ok := AmagosToCap(easy, 50)
	if !ok || got < 130 || got > 160 {
		t.Errorf("a flat 85%% curve = %d,%v, want roughly 146", got, ok)
	}

	var stuck [domain.MountGrowthBands]int16
	for i := range stuck {
		stuck[i] = 85
	}
	// One band below the break-even point is enough: the mount never arrives.
	stuck[5] = 16
	if _, ok := AmagosToCap(stuck, 50); ok {
		t.Error("a band under break-even must report the mount as unreachable")
	}

	// An unset band falls back to the default rather than counting as zero.
	unset := unsetRates()
	if _, ok := AmagosToCap(unset, 50); !ok {
		t.Error("an unconfigured curve must cost what the default costs")
	}
}

func TestBandLabel(t *testing.T) {
	if got := BandLabel(0); got != "1 – 20" {
		t.Errorf("band 0 = %q", got)
	}
	if got := BandLabel(5); got != "101 – 120" {
		t.Errorf("band 5 = %q", got)
	}
}

func (f *fakeStore) ListMountBonus(context.Context) ([]domain.MountBonus, error) {
	return f.bonus, nil
}
func (f *fakeStore) SetMountBonus(_ context.Context, b domain.MountBonus, _ int64, _ string) error {
	f.bonus = append(f.bonus, b)
	return nil
}
func (f *fakeStore) ClearMountBonus(context.Context, int16, int64) error { return nil }

// TestAtributosMostramOPadraoAoLado pins what the screen needs to make a
// decision: every lineage, configured or not, with the compiled default beside
// what is in effect — Andaluz B edited to 40 of immunity still says it was 32.
func TestAtributosMostramOPadraoAoLado(t *testing.T) {
	st := &fakeStore{bonus: []domain.MountBonus{
		{MountIndex: 2375, Attack: 500, Magic: 85, Evasion: 20, Resist: 40},
	}}
	svc := New(st)
	rows, err := svc.ListBonus(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 30 {
		t.Fatalf("%d linhagens, want as 30 adultas", len(rows))
	}
	var andaluz, svad Bonus
	for _, r := range rows {
		switch r.MountIndex {
		case 2375:
			andaluz = r
		case 2387:
			svad = r
		}
	}
	if !andaluz.Configured || andaluz.Current.Resist != 40 || andaluz.Current.Evasion != 20 {
		t.Errorf("Andaluz B = %+v, want configurada com imunidade 40 e evasão 20", andaluz)
	}
	if andaluz.Default.Resist != 32 || andaluz.Default.Evasion != 0 {
		t.Errorf("padrão da Andaluz B = %+v, want 32 de imunidade e 0 de evasão", andaluz.Default)
	}
	if svad.Configured || svad.Current != svad.Default || svad.Current.Attack != 600 {
		t.Errorf("Svadilfari = %+v, want no padrão do cliente {600,40,60,28}", svad)
	}
}
