package combat

import (
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/rng"
)

// seqRand returns predetermined Intn values in order, so each formula's rand()
// calls are controlled and the math can be checked by hand.
type seqRand struct {
	vals []int
	i    int
}

func (s *seqRand) Intn(int) int {
	v := s.vals[s.i]
	s.i++
	return v
}

func TestDamage(t *testing.T) {
	tests := []struct {
		name            string
		dam, ac, combat int
		roll, want      int
	}{
		{"normal", 100, 20, 10, 3, 96}, // tdam 90; rnd 107; 96
		{"low floor", 10, 30, 0, 0, 6}, // tdam -5; rnd 99; -4 → (-4+50)/7=6
		{"min one", 0, 100, 14, 0, 1},  // tdam -50; rnd 106; -53 → 0 → 1
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Damage(&seqRand{vals: []int{tt.roll}}, tt.dam, tt.ac, tt.combat)
			if got != tt.want {
				t.Errorf("Damage = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestSkillDamage(t *testing.T) {
	tests := []struct {
		name            string
		dam, ac, combat int
		roll, want      int
	}{
		{"normal", 100, 20, 10, 0, 90}, // tdam 90; rnd 100; 90
		{"ladder", 60, 40, 5, 2, 52},   // tdam 40; rnd 97; 38 → 5*38/4+5=52
		{"min one", 0, 120, 20, 0, 1},  // cap 15; tdam -63 → 0 → 1
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SkillDamage(&seqRand{vals: []int{tt.roll}}, tt.dam, tt.ac, tt.combat)
			if got != tt.want {
				t.Errorf("SkillDamage = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestResolveHit(t *testing.T) {
	tests := []struct {
		name string
		in   HitInput
		seq  []int
		want int
	}{
		{
			name: "melee hit vs mob",
			in:   HitInput{AttackerDamage: 200, TargetAC: 40, Master: 10},
			seq:  []int{3, 500}, // damage roll, parry roll
			want: 192,
		},
		{
			name: "partial crit vs player",
			in:   HitInput{AttackerDamage: 100, TargetAC: 10, TargetIsPlayer: true, DoubleCritical: 2},
			seq:  []int{1, 0, 500}, // crit roll, damage roll, parry roll
			want: 123,
		},
		{
			name: "parry block -3",
			in:   HitInput{AttackerDamage: 100, ParryRate: 600},
			seq:  []int{0, 100}, // damage roll, parry roll (rd=101 < 600)
			want: -3,
		},
		{
			name: "parry block -4",
			in:   HitInput{AttackerDamage: 100, ParryRate: 600, TargetRsvBlock: true},
			seq:  []int{0, 50}, // rd=51 < 100 and < 600 → -4
			want: -4,
		},
		{
			name: "reflect vs player",
			in:   HitInput{AttackerDamage: 100, TargetIsPlayer: true, ReflectDamage: 10, ReflectPvP: 20},
			seq:  []int{0, 500}, // damage roll, parry roll
			want: 89,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveHit(&seqRand{vals: tt.seq}, tt.in)
			if got != tt.want {
				t.Errorf("ResolveHit = %d, want %d", got, tt.want)
			}
		})
	}
}

// TestDamageWithMSVCLCG ties the formula to the real MSVC stream: with the
// default seed the first rand() is 41, so 41%7=6 ⇒ rnd=110 ⇒ 90*110/100 = 99.
func TestDamageWithMSVCLCG(t *testing.T) {
	if got := Damage(rng.New(), 100, 20, 10); got != 99 {
		t.Errorf("Damage(LCG) = %d, want 99", got)
	}
}

// TestResolveHitDeterministic confirms the same seed yields the same result
// (the basis for exact golden cases from a clean boot).
func TestResolveHitDeterministic(t *testing.T) {
	in := HitInput{AttackerDamage: 250, TargetAC: 30, Master: 8, DoubleCritical: 2}
	a := ResolveHit(rng.NewSeeded(1), in)
	b := ResolveHit(rng.NewSeeded(1), in)
	if a != b {
		t.Errorf("non-deterministic: %d != %d", a, b)
	}
}

func TestParryRate(t *testing.T) {
	tests := []struct {
		name                        string
		targetDex, add, attackerDex int
		attackerRsv, want           int
	}{
		{"floor", 10, -5, 500, 0, 1},
		{"dex tiers", 3500, 10, 100, 0, 650},
		{"rsv bonuses", 1000, 0, 500, 0x20 | 0x80, 150},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParryRate(tt.targetDex, tt.add, tt.attackerDex, tt.attackerRsv)
			if got != tt.want {
				t.Errorf("ParryRate = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestDoubleCritical(t *testing.T) {
	server, client := uint16(0), uint16(0)
	flags, ok := DoubleCritical(rng.NewSeeded(1), 0xB2, 255, &server, &client)
	if !ok {
		t.Fatal("DoubleCritical returned !ok")
	}
	if flags != 0x3 {
		t.Fatalf("flags = %#x, want total+partial crit", flags)
	}
	if server != 1 || client != 1 {
		t.Fatalf("progress = server %d client %d, want 1/1", server, client)
	}
}
