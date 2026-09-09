package handler

import (
	"io"
	"log/slog"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/rng"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

func TestSameKingdom(t *testing.T) {
	tests := []struct {
		name string
		a, b uint8
		want bool
	}{
		{"Hekalotia allies", clanHekalotia, clanHekalotia, true},
		{"Akelonia allies", clanAkelonia, clanAkelonia, true},
		{"opposing kingdoms", clanHekalotia, clanAkelonia, false},
		{"neutral equality", 0, 0, false},
		{"other equality", 5, 5, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sameKingdom(tt.a, tt.b); got != tt.want {
				t.Errorf("sameKingdom(%d,%d) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestSameKingdomBattleDoesNotEngageGroup(t *testing.T) {
	for _, kingdom := range []uint8{clanHekalotia, clanAkelonia} {
		t.Run(map[uint8]string{clanHekalotia: "Hekalotia", clanAkelonia: "Akelonia"}[kingdom], func(t *testing.T) {
			_, w, ids := groupWorld(t)
			for _, id := range ids {
				w.Entity(id).Clan = kingdom
			}
			attacker := &world.Entity{ID: 3, X: 22, Y: 20, Clan: kingdom}
			setGroupBattle(w, ids[1], w.Entity(ids[1]), attacker)

			for _, id := range ids {
				e := w.Entity(id)
				if e.Mode == world.MobCombat || e.Target != 0 || enemyListContains(e, attacker.ID) {
					t.Fatalf("allied group member %d entered battle: Mode=%v Target=%d EnemyList=%v", id, e.Mode, e.Target, e.EnemyList)
				}
			}
		})
	}
}

func TestKingdomTargetSelectionDropsAlliedPlayer(t *testing.T) {
	addr, stop, d, w := startServerWeather(t, newDB(), nil)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	runInLoop(t, w, func() {
		player := w.Entity(1)
		player.Clan = clanHekalotia
		w.SetEntityPos(1, 11, 10)
		tmpl := aggressiveMob()
		tmpl[16] = clanHekalotia
		mobID := w.SpawnMobAt(world.MobSpawn{Template: tmpl, X: 10, Y: 10, GenIndex: -1})
		mob := w.Entity(mobID)
		mob.Mode = world.MobCombat
		mob.Target = player.ID
		mob.EnemyList[0] = player.ID

		d.mobBattle(w, mobID, mob)

		if mob.Mode != world.MobIdle || mob.Target != 0 || enemyListContains(mob, player.ID) {
			t.Fatalf("allied target retained: Mode=%v Target=%d EnemyList=%v", mob.Mode, mob.Target, mob.EnemyList)
		}
	})
}

func TestKingdomMobAttackFinalGuardConsumesNoRNG(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := New(Config{Log: log})
	w := world.New(world.Config{GridDim: 16, Now: func() uint32 { return serverTime }}, log, nil, d.Handle)
	mob := &world.Entity{ID: world.MaxUser, Clan: clanAkelonia, HP: 1000, Damage: 500, Mode: world.MobCombat, Target: 1}
	mob.EnemyList[0] = 1
	player := &world.Entity{ID: 1, Clan: clanAkelonia, HP: 1000, MaxHP: 1000}

	d.mobAttack(w, mob.ID, mob, player)

	if player.HP != 1000 || mob.AtkTick != 0 || mob.Target != 0 || enemyListContains(mob, player.ID) {
		t.Fatalf("allied final guard state: playerHP=%d AtkTick=%d Target=%d EnemyList=%v", player.HP, mob.AtkTick, mob.Target, mob.EnemyList)
	}
	want := rng.New().Intn(1000)
	if got := w.Rand().Intn(1000); got != want {
		t.Fatalf("RNG advanced on allied strike: got next %d, want %d", got, want)
	}
}

func TestKingdomGuardTowersIgnoreAlliesAndAttackOpponents(t *testing.T) {
	for _, towerClan := range []uint8{clanHekalotia, clanAkelonia} {
		name := map[uint8]string{clanHekalotia: "Torre_Guardia", clanAkelonia: "Torre_Guardia_"}[towerClan]
		t.Run(name, func(t *testing.T) {
			addr, stop, d, w := startServerWeather(t, newDB(), nil)
			defer stop()
			c := enterWorld(t, addr)
			defer c.Close()

			runInLoop(t, w, func() {
				player := w.Entity(1)
				player.Clan = towerClan
				player.HP, player.MaxHP = 1000, 1000
				w.SetEntityPos(1, 11, 10)
				tmpl := targetMobWithClan(name, towerClan, 0, 5000)
				towerID := w.SpawnMobAt(world.MobSpawn{Template: tmpl, X: 10, Y: 10, GenIndex: -1})
				tower := w.Entity(towerID)
				tower.Damage = 500

				if validTarget(w, tower, player) {
					t.Fatal("guard tower accepted its allied kingdom as a target")
				}
				d.mobAttack(w, towerID, tower, player)
				if player.HP != 1000 {
					t.Fatalf("guard tower damaged ally: HP=%d", player.HP)
				}

				if towerClan == clanHekalotia {
					player.Clan = clanAkelonia
				} else {
					player.Clan = clanHekalotia
				}
				if !validTarget(w, tower, player) {
					t.Fatal("guard tower rejected an opposing kingdom target")
				}
				d.mobAttack(w, towerID, tower, player)
				if player.HP >= 1000 {
					t.Fatalf("guard tower did not damage opponent: HP=%d", player.HP)
				}
			})
		})
	}
}

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
