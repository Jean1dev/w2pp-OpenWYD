package handler

import (
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

func TestExpBonusAffect39(t *testing.T) {
	e := &world.Entity{}
	e.Affect[0] = world.Affect{Type: world.AffectExpChest}
	applyAffectScore(e)
	if e.AffExpBonus != 100 {
		t.Errorf("AffExpBonus = %d, want 100", e.AffExpBonus)
	}
}
