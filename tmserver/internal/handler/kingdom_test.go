package handler

import (
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

func TestKingdomDamageExemptsClanAndNeverKills(t *testing.T) {
	addr, stop, d, w := startServerWeather(t, newDB(), nil)
	defer stop()
	enemyConn := enterWorldAs(t, addr, "tester")
	defer enemyConn.Close()
	exemptConn := enterWorldAs(t, addr, "tradeb")
	defer exemptConn.Close()

	box := areaBox{10, 10, 20, 20}
	runInLoop(t, w, func() {
		enemy := w.Entity(1)
		exempt := w.Entity(2)
		enemy.X, enemy.Y, enemy.Clan = 15, 15, clanAkelonia
		enemy.MaxHP, enemy.HP = 1000, 1000
		w.Session(1).ReqHp = 1000
		exempt.X, exempt.Y, exempt.Clan = 15, 15, clanHekalotia
		exempt.MaxHP, exempt.HP = 1000, 1000
		w.Session(2).ReqHp = 1000

		d.sendDamageKingdom(w, box, clanHekalotia)
		if enemy.HP != 900 || w.Session(1).ReqHp != 900 {
			t.Fatalf("enemy HP/ReqHp = %d/%d, want 900/900", enemy.HP, w.Session(1).ReqHp)
		}
		if exempt.HP != 1000 {
			t.Fatalf("exempt clan HP = %d, want 1000", exempt.HP)
		}

		enemy.HP, w.Session(1).ReqHp = 5, 5
		for range 50 {
			d.sendDamageKingdom(w, box, clanHekalotia)
		}
		if enemy.HP != 1 {
			t.Fatalf("kingdom wall killed player: HP = %d, want floor 1", enemy.HP)
		}
	})
}

func TestKingdomKingClearDelay(t *testing.T) {
	d, w, _ := mobKilledWorld(t)
	d.kingdomKingKilled(w, &world.Entity{GenIndex: kingHarabardGen})
	if d.events.kingdom1 != 1 {
		t.Fatalf("Kingdom1Clear = %d after king death, want 1", d.events.kingdom1)
	}
	d.advanceKingdomClear(w, &d.events.kingdom1, kingdom1Room)
	if d.events.kingdom1 != 2 {
		t.Fatalf("Kingdom1Clear = %d after first minute, want 2", d.events.kingdom1)
	}
	d.advanceKingdomClear(w, &d.events.kingdom1, kingdom1Room)
	if d.events.kingdom1 != 0 {
		t.Fatalf("Kingdom1Clear = %d after second minute, want 0", d.events.kingdom1)
	}
}

func TestAreaBoxBoundaryModes(t *testing.T) {
	b := areaBox{10, 20, 30, 40}
	if !b.contains(30, 40) {
		t.Fatal("inclusive area rejected upper corner")
	}
	if b.containsExclusive(30, 40) {
		t.Fatal("exclusive area accepted upper corner")
	}
	if !b.containsExclusive(10, 20) {
		t.Fatal("exclusive-upper area rejected lower corner")
	}
}
