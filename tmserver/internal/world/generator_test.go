package world

import (
	"encoding/binary"
	"testing"
)

// genMobTemplate is an 816-byte STRUCT_MOB with enough score to spawn alive.
func genMobTemplate(clan uint8) []byte {
	b := make([]byte, structMobTemplateSize)
	copy(b[0:16], "Grunt")
	b[16] = clan
	const cs = 92
	binary.LittleEndian.PutUint32(b[cs+16:], 100) // MaxHp
	binary.LittleEndian.PutUint32(b[cs+24:], 100) // Hp
	return b
}

// TestGenerateMobGroup covers the GenerateMob port (Server.cpp:3442-3810):
// group size, leader/follower linking, per-block population accounting
// (including the leader-not-counted-in-the-clamp quirk) and the cap.
func TestGenerateMobGroup(t *testing.T) {
	w := New(Config{GridDim: 64}, slogDiscard(), nil, nil)
	g := &Generator{
		MinuteGenerate: 1,
		MinGroup:       2, MaxGroup: 2, // qmob=1 → rand%1=0 → always 2 followers
		MaxNumMob:    5,
		RouteType:    2,
		SegX:         [5]int16{20, 0, 0, 0, 30},
		SegY:         [5]int16{20, 0, 0, 0, 20},
		SegRange:     [5]int16{2, 0, 0, 0, 0},
		LeaderTmpl:   genMobTemplate(5),
		FollowerTmpl: genMobTemplate(5),
	}
	w.RegisterGenerators([]*Generator{g})

	ids := w.GenerateMob(0)
	if len(ids) != 3 {
		t.Fatalf("GenerateMob spawned %d mobs, want 3 (leader + 2 followers)", len(ids))
	}
	if g.CurrentNumMob != 3 {
		t.Fatalf("CurrentNumMob = %d, want 3", g.CurrentNumMob)
	}
	leader := w.Entity(ids[0])
	if leader.Leader != 0 || leader.PartyList[0] != ids[1] || leader.PartyList[1] != ids[2] {
		t.Fatalf("leader links = Leader %d PartyList %v, want 0 / followers %v", leader.Leader, leader.PartyList[:2], ids[1:])
	}
	for _, fid := range ids[1:] {
		fe := w.Entity(fid)
		if fe.Leader != ids[0] {
			t.Fatalf("follower %d Leader = %d, want %d", fid, fe.Leader, ids[0])
		}
		// Followers share the leader's randomized waypoints.
		if fe.SegListX != leader.SegListX || fe.SegListY != leader.SegListY {
			t.Fatalf("follower waypoints %v differ from leader's %v", fe.SegListX, leader.SegListX)
		}
	}
	// Waypoint 0 randomized within [seg-Range, seg] (the legacy negative bias);
	// waypoint 4 has no range → exact.
	if leader.SegListX[0] < 18 || leader.SegListX[0] > 20 || leader.SegListX[4] != 30 {
		t.Fatalf("leader waypoints = %v, want [0] in [18,20] and [4]==30", leader.SegListX)
	}

	// Second group: clamp says current(3)+n(2) ≤ 5 → allowed; the leader itself
	// is NOT counted by the clamp (kept quirk), so CurrentNumMob overshoots to 6.
	if got := len(w.GenerateMob(0)); got != 3 {
		t.Fatalf("second GenerateMob spawned %d, want 3", got)
	}
	if g.CurrentNumMob != 6 {
		t.Fatalf("CurrentNumMob after overshoot = %d, want 6", g.CurrentNumMob)
	}
	// Now saturated: no more spawns until deaths bring the count down.
	if got := len(w.GenerateMob(0)); got != 0 {
		t.Fatalf("saturated GenerateMob spawned %d, want 0", got)
	}
}

// TestGenerateMobDeathAccounting: a death decrements the block's population
// (DeleteMob, Server.cpp:7825-7831), releases the group links, and — for a
// timer-regenerated block (MinuteGenerate>0) — does NOT enter the 15s respawn
// queue (the minute timer refills whole groups instead).
func TestGenerateMobDeathAccounting(t *testing.T) {
	w := New(Config{GridDim: 64}, slogDiscard(), nil, nil)
	g := &Generator{
		MinuteGenerate: 1, MinGroup: 1, MaxGroup: 1, MaxNumMob: 9,
		SegX: [5]int16{20}, SegY: [5]int16{20},
		LeaderTmpl: genMobTemplate(5), FollowerTmpl: genMobTemplate(5),
	}
	w.RegisterGenerators([]*Generator{g})
	ids := w.GenerateMob(0) // leader + 1 follower
	if len(ids) != 2 || g.CurrentNumMob != 2 {
		t.Fatalf("setup: ids=%v CurrentNumMob=%d, want 2/2", ids, g.CurrentNumMob)
	}
	leader, follower := ids[0], ids[1]

	w.DespawnMob(follower, 1)
	if g.CurrentNumMob != 1 {
		t.Fatalf("CurrentNumMob after follower death = %d, want 1", g.CurrentNumMob)
	}
	if w.Entity(leader).PartyList[0] != 0 {
		t.Fatal("dead follower still in the leader's PartyList")
	}
	if got := len(w.SpawnDueRespawns(^uint32(0))); got != 0 {
		t.Fatalf("timer-block death queued %d respawns, want 0 (the minute timer refills)", got)
	}

	w.DespawnMob(leader, 1)
	if g.CurrentNumMob != 0 {
		t.Fatalf("CurrentNumMob after leader death = %d, want 0", g.CurrentNumMob)
	}
}

// TestGenerateMobQueueFallback: a block the timer never touches
// (MinuteGenerate<=0) falls back to the 15s respawn queue — our deliberate
// divergence (the original never regenerates those outside events) — and the
// queue respawn restores the block's population count.
func TestGenerateMobQueueFallback(t *testing.T) {
	w := New(Config{GridDim: 64}, slogDiscard(), nil, nil)
	g := &Generator{
		MinuteGenerate: -1, MinGroup: 0, MaxGroup: 0, MaxNumMob: 9,
		SegX: [5]int16{20}, SegY: [5]int16{20},
		LeaderTmpl: genMobTemplate(5),
	}
	w.RegisterGenerators([]*Generator{g})
	ids := w.GenerateMob(0)
	if len(ids) != 1 || g.CurrentNumMob != 1 {
		t.Fatalf("setup: ids=%v CurrentNumMob=%d, want 1/1", ids, g.CurrentNumMob)
	}

	w.DespawnMob(ids[0], 1)
	if g.CurrentNumMob != 0 {
		t.Fatalf("CurrentNumMob after death = %d, want 0", g.CurrentNumMob)
	}
	respawned := w.SpawnDueRespawns(^uint32(0)) // far future: delay elapsed
	if len(respawned) != 1 {
		t.Fatalf("queue respawned %d mobs, want 1", len(respawned))
	}
	if g.CurrentNumMob != 1 {
		t.Fatalf("CurrentNumMob after queue respawn = %d, want 1 (SpawnMobAt re-counts)", g.CurrentNumMob)
	}
}
