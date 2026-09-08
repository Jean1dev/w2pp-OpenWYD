package handler

import (
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// A Jóia da Precisão is the one PvP jewel the original leaves inert: it writes
// Accuracy += 50 into CMob::Accuracy (Basedef.cpp:4539) and no line of the
// legacy server ever reads that field back. This port spends the 50 in the parry
// formula, which is where the legacy's own naming points — GetParryRate already
// subtracts an attacker "dex" made of Dex/5, +100 for skill bit 24 and +500 for
// the Revelação (GetFunc.cpp:686). The Precisão is therefore a smaller sibling of
// the Revelação, and the deliberate divergence is documented on AffAccuracy.
func TestPrecisaoLowersTheTargetParry(t *testing.T) {
	d := &Dispatcher{}
	target := &world.Entity{Dex: 400, Parry: 40}

	plain := &world.Entity{Dex: 400}
	base := d.parryRate(plain, target)

	jewelled := &world.Entity{Dex: 400}
	jewelled.Affect[0] = world.Affect{Type: affectPvP, Level: 1 << 6, Time: 100}
	applyAffectScore(jewelled)
	if jewelled.AffAccuracy != 50 {
		t.Fatalf("AffAccuracy = %d, want 50", jewelled.AffAccuracy)
	}

	if got := d.parryRate(jewelled, target); got != base-50 {
		t.Errorf("parry rate with the Precisão = %d, want %d (base %d − 50)", got, base-50, base)
	}
}

// The jewel must not become a second Revelação: it moves the same term, by a
// tenth of the amount, and stays inside the legacy's [1,650] clamp.
func TestPrecisaoIsSmallerThanRevelacao(t *testing.T) {
	d := &Dispatcher{}
	target := &world.Entity{Dex: 4000, Parry: 100} // a heavily evasive target

	prec := &world.Entity{Dex: 400}
	prec.Affect[0] = world.Affect{Type: affectPvP, Level: 1 << 6, Time: 100}
	applyAffectScore(prec)

	rev := &world.Entity{Dex: 400}
	rev.Affect[0] = world.Affect{Type: affectPvP, Level: 1 << 2, Time: 100}
	applyAffectScore(rev)

	precRate, revRate := d.parryRate(prec, target), d.parryRate(rev, target)
	if precRate <= revRate {
		t.Errorf("parry with the Precisão = %d, with the Revelação = %d: the Precisão must be the weaker of the two", precRate, revRate)
	}
	if precRate < 1 || precRate > 650 {
		t.Errorf("parry rate %d escaped the legacy [1,650] clamp", precRate)
	}
}

// Without the jewel nothing moves — the field is zeroed on every affect pass, so
// an expired jewel cannot leave its bonus behind.
func TestPrecisaoClearsWhenTheJewelExpires(t *testing.T) {
	e := &world.Entity{Dex: 400}
	e.Affect[0] = world.Affect{Type: affectPvP, Level: 1 << 6, Time: 100}
	applyAffectScore(e)

	e.Affect[0] = world.Affect{}
	applyAffectScore(e)
	if e.AffAccuracy != 0 {
		t.Errorf("AffAccuracy = %d after the jewel expired, want 0", e.AffAccuracy)
	}
}
