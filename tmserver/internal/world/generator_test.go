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

func genMerchantTemplate(merchant uint8) []byte {
	b := genMobTemplate(0)
	const cs = 92
	b[cs+12] = merchant
	return b
}

func TestClearGeneratorRemovesWholeGroup(t *testing.T) {
	w := New(Config{GridDim: 64}, slogDiscard(), nil, nil)
	g := &Generator{
		DBManaged: true, MinGroup: 2, MaxGroup: 2, MaxNumMob: 3,
		SegX: [5]int16{20}, SegY: [5]int16{20},
		LeaderTmpl: genMerchantTemplate(12), FollowerTmpl: genMobTemplate(0),
	}
	w.RegisterGenerators([]*Generator{g})
	ids := w.GenerateMob(0)
	if len(ids) != 3 {
		t.Fatalf("GenerateMob spawned %d entities, want 3", len(ids))
	}

	w.ClearGenerator(0)

	if g.CurrentNumMob != 0 {
		t.Fatalf("CurrentNumMob = %d, want 0", g.CurrentNumMob)
	}
	for _, id := range ids {
		if w.Entity(id) != nil {
			t.Errorf("entity %d survived generator clear", id)
		}
	}
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
	wantOffset := []struct{ x, y int16 }{{1, 1}, {-1, 1}}
	for i, fid := range ids[1:] {
		fe := w.Entity(fid)
		if fe.Leader != ids[0] {
			t.Fatalf("follower %d Leader = %d, want %d", fid, fe.Leader, ids[0])
		}
		if fe.SegListX[0] != leader.SegListX[0]+wantOffset[i].x ||
			fe.SegListY[0] != leader.SegListY[0]+wantOffset[i].y {
			t.Fatalf("follower %d start = %d,%d; leader = %d,%d; want offset %+v",
				i, fe.SegListX[0], fe.SegListY[0], leader.SegListX[0], leader.SegListY[0], wantOffset[i])
		}
		// Waypoints without SegmentRange keep the generator's exact coordinate,
		// matching GenerateMob's follower branch.
		if fe.SegListX[4] != 30 || fe.SegListY[4] != 20 {
			t.Fatalf("follower %d dest = %d,%d, want 30,20", i, fe.SegListX[4], fe.SegListY[4])
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

func TestSpawnGeneratorLeaderUsesExactWaypointWithoutGroup(t *testing.T) {
	w := New(Config{GridDim: 64}, slogDiscard(), nil, nil)
	g := &Generator{
		MinGroup: 4, MaxGroup: 9, MaxNumMob: 10,
		SegX: [5]int16{20}, SegY: [5]int16{21}, SegRange: [5]int16{10},
		LeaderTmpl: genMobTemplate(1), FollowerTmpl: genMobTemplate(1),
	}
	w.RegisterGenerators([]*Generator{g})

	id := w.SpawnGeneratorLeader(0)
	if id < 0 {
		t.Fatal("SpawnGeneratorLeader failed")
	}
	e := w.Entity(id)
	if e.X != 20 || e.Y != 21 || e.SegListX[0] != 20 || e.SegListY[0] != 21 {
		t.Fatalf("leader position/waypoint = %d,%d / %d,%d, want exact 20,21", e.X, e.Y, e.SegListX[0], e.SegListY[0])
	}
	if g.CurrentNumMob != 1 {
		t.Fatalf("CurrentNumMob = %d, want leader only", g.CurrentNumMob)
	}
}

func TestGenerateMobFormationFourOffsetsFollowers(t *testing.T) {
	w := New(Config{GridDim: 64}, slogDiscard(), nil, nil)
	g := &Generator{
		MinuteGenerate: 1,
		MinGroup:       2, MaxGroup: 2,
		MaxNumMob:    5,
		RouteType:    6,
		Formation:    4,
		SegX:         [5]int16{24, 0, 0, 0, 28},
		SegY:         [5]int16{24, 0, 0, 0, 24},
		SegRange:     [5]int16{1, 0, 0, 0, 1},
		LeaderTmpl:   genMobTemplate(5),
		FollowerTmpl: genMobTemplate(5),
	}
	w.RegisterGenerators([]*Generator{g})

	ids := w.GenerateMob(0)
	if len(ids) != 3 {
		t.Fatalf("GenerateMob spawned %d mobs, want 3", len(ids))
	}
	leader := w.Entity(ids[0])
	tests := []struct {
		id int
		x  int16
		y  int16
	}{
		{ids[1], 2, 0},
		{ids[2], 0, 2},
	}
	for _, tt := range tests {
		follower := w.Entity(tt.id)
		if follower.SegListX[0] != leader.SegListX[0]+tt.x ||
			follower.SegListY[0] != leader.SegListY[0]+tt.y {
			t.Fatalf("follower %d start = %d,%d; leader = %d,%d; want offset %d,%d",
				tt.id, follower.SegListX[0], follower.SegListY[0], leader.SegListX[0], leader.SegListY[0], tt.x, tt.y)
		}
		if follower.SegListX[4] != leader.SegListX[4]+tt.x ||
			follower.SegListY[4] != leader.SegListY[4]+tt.y {
			t.Fatalf("follower %d dest = %d,%d; leader = %d,%d; want offset %d,%d",
				tt.id, follower.SegListX[4], follower.SegListY[4], leader.SegListX[4], leader.SegListY[4], tt.x, tt.y)
		}
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

func TestGenerateMobNegativeCapSpawnsStaticMerchant(t *testing.T) {
	w := New(Config{GridDim: 4096}, slogDiscard(), nil, nil)
	g := &Generator{
		MinuteGenerate: 1, MinGroup: 0, MaxGroup: 0, MaxNumMob: -1,
		SegX: [5]int16{2116}, SegY: [5]int16{2080},
		LeaderTmpl: genMerchantTemplate(23),
	}
	w.RegisterGenerators([]*Generator{g})

	ids := w.GenerateMob(0)
	if len(ids) != 1 {
		t.Fatalf("GenerateMob negative cap spawned %d mobs, want 1", len(ids))
	}
	if g.CurrentNumMob != 1 {
		t.Fatalf("CurrentNumMob = %d, want 1", g.CurrentNumMob)
	}
	if got := len(w.GenerateMob(0)); got != 0 {
		t.Fatalf("second GenerateMob spawned %d, want cap reached", got)
	}
}
