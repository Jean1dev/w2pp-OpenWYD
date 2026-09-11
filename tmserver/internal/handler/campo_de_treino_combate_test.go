package handler

import (
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// O Orc_Sniper do campo de treino tem 16 no byte de loja que o port lê e 0 no
// do legado: era imortal, e todo golpe voltava 0 (combat.go, o portão do
// NonCombatNPC). No campo decide o byte do legado (internal/campotreino), então o
// golpe tem de tirar vida — de ponta a ponta, pelo socket.
func TestOrcSniperDoCampoRecebeDano(t *testing.T) {
	db := skillCombatDB(1 << 2)
	db.loadResult.X, db.loadResult.Y = 2078, 1976 // colado no Orc_Sniper (2079,1976)
	addr, stop, w := startServerSkillsTargetMobAt(t, db, targetMobWithClan("Orc_Sniper", 1, 16, 5000), 4096, 2079, 1976)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	if dmg := attackEchoDamage(t, c, serverTime, world.MaxUser, -1, -2); dmg <= 0 {
		t.Fatalf("golpe no Orc_Sniper do campo = %d, want > 0", dmg)
	}
	if mob := w.Entity(world.MaxUser); mob == nil || mob.NonCombatNPC || mob.HP >= 5000 {
		t.Fatalf("Orc_Sniper = %+v, want monstro com a vida reduzida", mob)
	}
}
