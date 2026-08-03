package handler

import (
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/worldevents"
)

func TestTowerCaptureRespawnsUnderKillerGuild(t *testing.T) {
	d, w, _ := mobKilledWorld(t)
	g := &world.Generator{
		MinGroup: 0, MaxGroup: 0, MaxNumMob: 1,
		SegX: [5]int16{10}, SegY: [5]int16{10},
		LeaderTmpl: expMobTemplate(1, 0, 0),
	}
	gens := make([]*world.Generator, towerGenerator+1)
	gens[towerGenerator] = g
	w.RegisterGenerators(gens)
	originalID := w.SpawnGeneratorLeader(towerGenerator)
	original := w.Entity(originalID)

	monday := time.Date(2026, time.August, 3, 20, 0, 0, 0, time.Local)
	d.events.tower = worldevents.NewTower(20)
	d.events.tower.Step(monday, true)
	d.events.tower.Step(monday.Add(6*time.Minute), true)
	reward := &world.Entity{ID: 1, Guild: 77}
	if !d.towerKilled(w, reward, original) {
		t.Fatal("tower kill was not consumed by the event")
	}
	if d.events.towerOwner != 77 {
		t.Fatalf("tower owner = %d, want 77", d.events.towerOwner)
	}
	if w.Entity(originalID) == original {
		t.Fatal("captured tower instance was not replaced")
	}
	var towers []*world.Entity
	w.ForEachMob(func(_ int, e *world.Entity) {
		if e.GenIndex == towerGenerator {
			towers = append(towers, e)
		}
	})
	if len(towers) != 1 || towers[0].Guild != 77 || towers[0].HP != 10000 {
		t.Fatalf("respawned towers = %+v, want one owner=77 HP=10000", towers)
	}
}

func TestTowerAttackRules(t *testing.T) {
	d := New(Config{})
	target := &world.Entity{GenIndex: towerGenerator, Guild: 10}
	if !d.towerAttackAllowed(&world.Entity{}, target) {
		t.Fatal("idle tower should use ordinary combat rules")
	}
	monday := time.Date(2026, time.August, 3, 20, 0, 0, 0, time.Local)
	d.events.tower.Step(monday, true)
	if d.towerAttackAllowed(&world.Entity{Guild: 0}, target) {
		t.Fatal("guildless attacker was allowed during tower war")
	}
	if d.towerAttackAllowed(&world.Entity{Guild: 10}, target) {
		t.Fatal("owner guild was allowed to attack its own tower")
	}
	if !d.towerAttackAllowed(&world.Entity{Guild: 11}, target) {
		t.Fatal("enemy guild was denied tower attack")
	}
}
