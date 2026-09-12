package world

import (
	"encoding/binary"
	"io"
	"log/slog"
	"testing"
)

// shopCloneTestTemplate stands in for Release/TMsrv/run/npc/Merc_Carbunkle. It
// ships Merchant=1 exactly like the real file, so the test proves SpawnShopClone
// zeroes it instead of inheriting it.
func shopCloneTestTemplate() []byte {
	b := make([]byte, 816)
	copy(b[0:16], "Merc_Carbunkle")
	b[16] = 2                                      // Clan
	const cs = 92                                  // CurrentScore
	b[cs+12] = 1                                   // Merchant = service NPC
	binary.LittleEndian.PutUint32(b[cs+0:], 2)     // Level
	binary.LittleEndian.PutUint32(b[cs+8:], 300)   // Damage
	binary.LittleEndian.PutUint32(b[cs+16:], 5000) // MaxHp
	binary.LittleEndian.PutUint32(b[cs+24:], 5000) // Hp
	return b
}

func shopCloneWorld(t *testing.T, template []byte) *World {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	w := New(Config{GridDim: 16, ShopCloneTemplate: template}, log, NopPersistence{}, nil)
	w.entities[1] = &Entity{ID: 1, Mode: MobUser, HP: 100, X: 5, Y: 5, Name: "Dono"}
	w.grid.SetMob(5, 5, 1)
	return w
}

func TestSpawnShopCloneEhCenarioIntocavel(t *testing.T) {
	w := shopCloneWorld(t, shopCloneTestTemplate())

	id := w.SpawnShopClone(1, "Dono")
	if id < MaxUser {
		t.Fatalf("clone recebeu id %d, quer um id de mob (>= %d)", id, MaxUser)
	}
	ce := w.Entity(id)
	if ce == nil {
		t.Fatal("clone não está no mundo")
	}
	if ce.ShopOwner != 1 {
		t.Errorf("ShopOwner = %d, quer 1", ce.ShopOwner)
	}
	if ce.Name != "Dono" {
		t.Errorf("nome do clone = %q, quer o nome do dono", ce.Name)
	}
	// Merchant zeroed even though the template ships 1: otherwise clicking the
	// stall runs the quest-NPC path in _MSG_Quest instead of opening the shop.
	if ce.Merchant != 0 {
		t.Errorf("Merchant = %d, quer 0 (senão o clique abre o menu de NPC)", ce.Merchant)
	}
	// And NonCombatNPC set by hand, because zeroing Merchant alone would have
	// turned the stall into a killable monster carrying a Template — which is
	// exactly what the respawn queue looks for.
	if !ce.NonCombatNPC {
		t.Error("o clone precisa ser NonCombatNPC")
	}
	// Beside the owner, never on top of him: the owner's own cell is the one cell
	// guaranteed to be occupied, and taking it would evict him from the grid.
	if ce.X == 5 && ce.Y == 5 {
		t.Error("clone nasceu na célula do dono")
	}
}

func TestDespawnShopCloneSoAtendeAoDono(t *testing.T) {
	w := shopCloneWorld(t, shopCloneTestTemplate())
	id := w.SpawnShopClone(1, "Dono")
	if id == 0 {
		t.Fatal("clone não subiu")
	}

	// A stale CloneID pointing at somebody else's entity must not take it down.
	w.DespawnShopClone(id, 999)
	if w.Entity(id) == nil {
		t.Fatal("um dono errado conseguiu derrubar o clone")
	}
	// A player id is never a clone.
	w.DespawnShopClone(1, 1)
	if w.Entity(1) == nil {
		t.Fatal("DespawnShopClone removeu um JOGADOR")
	}

	w.DespawnShopClone(id, 1)
	if w.Entity(id) != nil {
		t.Fatal("o clone continuou no mundo depois do DespawnShopClone")
	}
}

func TestSpawnShopCloneSemTemplateCaiNoFallback(t *testing.T) {
	w := shopCloneWorld(t, nil)
	// Zero, not -1: the caller reads 0 as "use the legacy pose", which still
	// opens the shop — it just pins the seller, the way the original did.
	if id := w.SpawnShopClone(1, "Dono"); id != 0 {
		t.Fatalf("SpawnShopClone = %d sem template, quer 0", id)
	}
}

func TestSpawnShopCloneSemDonoNoMundo(t *testing.T) {
	w := shopCloneWorld(t, shopCloneTestTemplate())
	if id := w.SpawnShopClone(42, "Fantasma"); id != 0 {
		t.Fatalf("SpawnShopClone = %d para um dono que não está no mundo, quer 0", id)
	}
}
