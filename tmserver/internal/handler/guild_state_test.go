package handler

import (
	"errors"
	"log/slog"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// guildStateFixture builds the dispatcher/world pair applyGuildStateSnapshot
// writes into. The snapshot is applied directly (no fakeDB): loadGuildStateSnapshot
// only fills the struct, and everything worth asserting happens on the apply side.
func guildStateFixture() (*Dispatcher, *world.World) {
	log := slog.New(slog.DiscardHandler)
	return New(Config{Log: log}), world.New(world.Config{GridDim: 16}, log, nil, nil)
}

func TestApplyGuildStateSnapshotRegistersGuilds(t *testing.T) {
	d, w := guildStateFixture()
	d.guildStateBusy = true
	d.applyGuildStateSnapshot(w, guildStateSnapshot{guilds: []world.GuildRecord{
		{ID: 5, Name: "Nova", Fame: 42},
		{ID: 9, Name: "Vento", Fame: -7},
	}})

	for _, want := range []world.GuildRecord{{ID: 5, Name: "Nova", Fame: 42}, {ID: 9, Name: "Vento", Fame: -7}} {
		gi, ok := w.GuildInfo(want.ID)
		if !ok {
			t.Fatalf("guild %d not registered", want.ID)
		}
		if gi.Name != want.Name || gi.Fame != want.Fame {
			t.Errorf("guild %d = %q/%d, want %q/%d", want.ID, gi.Name, gi.Fame, want.Name, want.Fame)
		}
	}
	if !d.guildStateLoad {
		t.Error("guildStateLoad = false after a clean snapshot")
	}
	if d.guildStateBusy {
		t.Error("guildStateBusy still set after apply")
	}
}

// A failed sub-load leaves that slice of state untouched and keeps guildStateLoad
// false, so ensureGuildStateLoaded retries on a later tick.
func TestApplyGuildStateSnapshotLoadErrors(t *testing.T) {
	boom := errors.New("db down")
	cases := []struct {
		name string
		fail func(*guildStateSnapshot)
	}{
		{"guilds", func(s *guildStateSnapshot) { s.guildsErr = boom }},
		{"zones", func(s *guildStateSnapshot) { s.zonesErr = boom }},
		{"relations", func(s *guildStateSnapshot) { s.relationsErr = boom }},
		{"tower", func(s *guildStateSnapshot) { s.towerErr = boom }},
		{"castle", func(s *guildStateSnapshot) { s.castleErr = boom }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d, w := guildStateFixture()
			snap := guildStateSnapshot{
				guilds: []world.GuildRecord{{ID: 5, Name: "Nova", Fame: 42}},
				tower:  world.GuildTowerState{OwnerGuild: 3},
			}
			tc.fail(&snap)
			d.applyGuildStateSnapshot(w, snap)

			if d.guildStateLoad {
				t.Errorf("guildStateLoad = true despite a %s load error", tc.name)
			}
			// Only the failing sub-load is skipped; the others still apply.
			wantGuilds := tc.name != "guilds"
			if _, ok := w.GuildInfo(5); ok != wantGuilds {
				t.Errorf("guild registered = %t on a %s error, want %t", ok, tc.name, wantGuilds)
			}
			wantTower := tc.name != "tower"
			if ok := d.events.towerOwner == 3; ok != wantTower {
				t.Errorf("towerOwner applied = %t on a %s error, want %t", ok, tc.name, wantTower)
			}
		})
	}
}

// The tower owner and an in-progress castle quest are runtime event state, not
// just the persisted record: both have to be rehydrated for the tick handlers.
func TestApplyGuildStateSnapshotRestoresEventState(t *testing.T) {
	d, w := guildStateFixture()
	d.applyGuildStateSnapshot(w, guildStateSnapshot{
		tower:  world.GuildTowerState{OwnerGuild: 77},
		castle: world.CastleQuestState{Level: 2, TimeLeft: 30, LeaderName: "lider"},
	})

	if d.events.towerOwner != 77 {
		t.Errorf("towerOwner = %d, want 77", d.events.towerOwner)
	}
	if level, left, _ := d.events.castle.State(); level != 2 || left != 30 {
		t.Errorf("castle state = level %d left %d, want 2/30", level, left)
	}
}

// An empty castle record means "no quest was running"; it must not resurrect one.
func TestApplyGuildStateSnapshotLeavesCastleIdle(t *testing.T) {
	d, w := guildStateFixture()
	d.applyGuildStateSnapshot(w, guildStateSnapshot{})

	if level, _, _ := d.events.castle.State(); level != -1 {
		t.Errorf("castle level = %d after an empty snapshot, want -1 (idle)", level)
	}
}
