package world

import "testing"

func TestKefraGeneratorsNeverQueueRespawns(t *testing.T) {
	for _, period := range []int{-1, 0, 1} {
		w := New(Config{GridDim: 64}, slogDiscard(), nil, nil)
		gens := make([]*Generator, 401)
		for idx := 396; idx <= 400; idx++ {
			gens[idx] = &Generator{LeaderTmpl: genMobTemplate(5), MaxNumMob: 1, MinuteGenerate: period, SegX: [5]int16{int16(10 + (idx-396)*8)}, SegY: [5]int16{10}}
		}
		w.RegisterGenerators(gens)
		for idx := 396; idx <= 400; idx++ {
			ids := w.GenerateMob(idx)
			if len(ids) != 1 {
				t.Fatal("spawn failed")
			}
			w.DespawnMob(ids[0], 1)
		}
		if len(w.respawnQueue) != 0 {
			t.Fatalf("period=%d queued %d event mobs", period, len(w.respawnQueue))
		}
	}
}
