package world

import (
	"encoding/binary"
	"testing"
)

func TestNewbieEventSpawnHandicap(t *testing.T) {
	tests := []struct {
		name    string
		enabled bool
		level   int32
		wantHP  int32
	}{
		{name: "disabled", level: 119, wantHP: 100},
		{name: "below cap", enabled: true, level: 119, wantHP: 75},
		{name: "at cap", enabled: true, level: 120, wantHP: 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := New(Config{GridDim: 16}, slogDiscard(), nil, nil)
			w.SetNewbieEvent(tt.enabled)
			id := w.SpawnMobAt(MobSpawn{Template: newbieMobTemplate(tt.level), X: 1, Y: 1, GenIndex: -1})
			e := w.Entity(id)
			if e.HP != tt.wantHP {
				t.Errorf("HP = %d, want %d", e.HP, tt.wantHP)
			}
			if e.MaxHP != 100 {
				t.Errorf("MaxHP = %d, want 100 (legacy handicaps current HP only)", e.MaxHP)
			}
		})
	}
}

func newbieMobTemplate(level int32) []byte {
	b := make([]byte, 816)
	copy(b, "NewbieMob")
	const currentScore = 92
	binary.LittleEndian.PutUint32(b[currentScore:], uint32(level))
	binary.LittleEndian.PutUint32(b[currentScore+16:], 100)
	binary.LittleEndian.PutUint32(b[currentScore+24:], 100)
	return b
}
