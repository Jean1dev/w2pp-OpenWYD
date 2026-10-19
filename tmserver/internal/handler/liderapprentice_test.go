package handler

import (
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// TestLiderAprendizRouting pins the issue #397 route on the shipped templates:
// only the Lider_Aprendiz, not the gate guards that share Merchant 100 +
// EF_GRADE0 5, nor the other Merchant-100 quest NPCs.
func TestLiderAprendizRouting(t *testing.T) {
	tests := []struct {
		tmpl string
		want bool
	}{
		{"Lider_Aprendiz", true},
		{"Guarda_Helgardh", false},
		{"Porteiro_Inferno", false},
		{"Chefe_de_Treino", false},
		{"Sobrevivente", false},
		{"Cap.Explor", false},
	}
	_, _, w, _ := startKefraServer(t, perzenDB(0))
	for _, tt := range tests {
		t.Run(tt.tmpl, func(t *testing.T) {
			runInLoop(t, w, func() {
				id := w.SpawnMob(npcTemplate(t, tt.tmpl), 20, 20)
				if id < world.MaxUser {
					t.Fatal("spawn failed")
				}
				if got := isLiderAprendizNPC(w.Entity(id)); got != tt.want {
					t.Errorf("isLiderAprendizNPC = %v, want %v", got, tt.want)
				}
			})
		})
	}
}

func TestLiderAprendiz(t *testing.T) {
	addr, d, w, _ := startKefraServer(t, perzenDB(0))
	c := enterWorld(t, addr)
	defer c.Close()
	var lider, guard int
	runInLoop(t, w, func() {
		lider = w.SpawnMob(npcTemplate(t, "Lider_Aprendiz"), 6, 6)
		guard = w.SpawnMob(npcTemplate(t, "Guarda_Helgardh"), 6, 6)
	})
	tests := []struct {
		name     string
		npc      *int
		class    uint8
		level    int32
		cape     int16
		wantCape int16
		wantSay  uint32
	}{
		{name: "arch 99 gets the cape", npc: &lider, class: classMasterArch, level: 99, wantCape: greenCapeItem},
		{name: "arch 149 gets the cape", npc: &lider, class: classMasterArch, level: 149, wantCape: greenCapeItem},
		{name: "arch 98", npc: &lider, class: classMasterArch, level: 98, wantSay: 1},
		{name: "arch 150", npc: &lider, class: classMasterArch, level: 150, wantSay: 1},
		{name: "mortal", npc: &lider, class: classMasterMortal, level: 120, wantSay: 1},
		{name: "cape already worn", npc: &lider, class: classMasterArch, level: 120, cape: 4007, wantCape: 4007, wantSay: 1},
		{name: "gate guard gives nothing", npc: &guard, class: classMasterArch, level: 120},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runInLoop(t, w, func() {
				s, e := w.SessionByName("Hero")
				e.ClassMaster, e.Level = tt.class, tt.level
				e.Equip[capeEquipSlot] = world.Item{Index: tt.cape}
				says := w.SentOfType(s, protocol.MsgMessageChat)

				d.quest(w, s, protocol.Header{}, protocol.EncodeStandardParm2(int32(*tt.npc), 0))

				if got := e.Equip[capeEquipSlot].Index; got != tt.wantCape {
					t.Errorf("Equip[15] = %d, want %d", got, tt.wantCape)
				}
				if got := w.SentOfType(s, protocol.MsgMessageChat) - says; got < tt.wantSay {
					t.Errorf("NPC lines = %d, want at least %d", got, tt.wantSay)
				}
			})
		})
	}
}
