package handler

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// itemSvadilfari (2387, adulta) vem de baus.go.
const (
	precoSvadilfari  = 850000
	itemCriaCavaloSN = 2336 // Cria_de_Cavalo_s/Sela_N
)

// mestreFixture is racaoFixture with a dead mount carrying `vidas` lives, the
// gold to pay for it, and a Mestre de Montaria to talk to.
func mestreFixture(t *testing.T, mount int16, vidas uint8, coin int32) (*Dispatcher, *world.World, *world.Session, *world.Entity, *world.Entity) {
	t.Helper()
	d, w, s, e := racaoFixture(t, mount, 0, 0)
	d.itemPrices = map[int]int32{int(mount): precoSvadilfari}
	e.Equip[mountEquipSlot].Effects[1].Value = vidas
	e.Coin = coin
	npc := &world.Entity{ID: world.MaxUser, Merchant: merchantMountMaster}
	return d, w, s, e, npc
}

func TestMestreCuraMontariaMorta(t *testing.T) {
	d, w, s, e, npc := mestreFixture(t, itemSvadilfari, 10, precoSvadilfari+7)
	d.mountMaster(w, s, e, npc, 1)

	m := e.Equip[mountEquipSlot]
	if m.Index != itemSvadilfari {
		t.Fatalf("montaria = %d, want %d — com 10 vidas a cura não pode destruí-la", m.Index, itemSvadilfari)
	}
	if hp := mountHP(m); hp != mountCureHP {
		t.Errorf("HP = %d, want %d (:215)", hp, mountCureHP)
	}
	if m.Effects[2].Effect != mountCureFeed {
		t.Errorf("ração = %d, want %d (:216)", m.Effects[2].Effect, mountCureFeed)
	}
	if e.Coin != 7 {
		t.Errorf("gold = %d, want 7 — a cura custa o preço da montaria no catálogo", e.Coin)
	}
}

func TestMestreDescontaZeroUmaOuDuasVidas(t *testing.T) {
	// `vit -= rand() % 3` (:209): the three losses are all possible and nothing
	// else is. Enough rounds to see every one of them.
	d, w, s, e, npc := mestreFixture(t, itemSvadilfari, 10, 0)
	perdas := map[int]int{}
	for i := 0; i < 600; i++ {
		m := world.Item{Index: itemSvadilfari}
		m.Effects[1].Value = 10
		e.Equip[mountEquipSlot] = m
		e.Coin = precoSvadilfari
		d.mountMaster(w, s, e, npc, 1)
		perdas[10-int(e.Equip[mountEquipSlot].Effects[1].Value)]++
	}
	for perda := range perdas {
		if perda < 0 || perda > 2 {
			t.Fatalf("a cura tirou %d vidas, want 0, 1 ou 2", perda)
		}
	}
	for _, perda := range []int{0, 1, 2} {
		if perdas[perda] == 0 {
			t.Errorf("em 600 curas nenhuma tirou %d vida(s): %v", perda, perdas)
		}
	}
}

func TestMestreDestroiMontariaSemVidas(t *testing.T) {
	// With one life left, a roll of 0 keeps it alive at 1 and anything else takes
	// the last one: the mount is gone and the gold stays spent (:218-222).
	d, w, s, e, npc := mestreFixture(t, itemSvadilfari, 1, 0)
	var curadas, perdidas int
	for i := 0; i < 300; i++ {
		m := world.Item{Index: itemSvadilfari}
		m.Effects[1].Value = 1
		e.Equip[mountEquipSlot] = m
		e.Coin = precoSvadilfari
		d.mountMaster(w, s, e, npc, 1)
		if e.Coin != 0 {
			t.Fatalf("gold = %d, want 0 — a cura cobra mesmo quando falha", e.Coin)
		}
		switch got := e.Equip[mountEquipSlot]; {
		case got.Empty():
			perdidas++
		case got.Effects[1].Value == 1 && mountHP(got) == mountCureHP:
			curadas++
		default:
			t.Fatalf("montaria depois da cura = %+v, want curada com 1 vida ou destruída", got)
		}
	}
	if curadas == 0 || perdidas == 0 {
		t.Errorf("curadas %d, perdidas %d: com 1 vida os dois desfechos têm de acontecer", curadas, perdidas)
	}
}

func TestMestreSoCobraComConfirmacao(t *testing.T) {
	// confirm 0 is the NPC quoting the price (:189-193): nothing is charged and
	// the mount stays as it was.
	d, w, s, e, npc := mestreFixture(t, itemSvadilfari, 10, precoSvadilfari)
	d.mountMaster(w, s, e, npc, 0)
	if e.Coin != precoSvadilfari {
		t.Errorf("gold = %d, want %d intocado", e.Coin, precoSvadilfari)
	}
	if mountHP(e.Equip[mountEquipSlot]) != 0 {
		t.Error("a montaria foi curada só com a pergunta")
	}
}

func TestMestreRecusa(t *testing.T) {
	cases := []struct {
		nome  string
		mount int16
		hp    uint16
		coin  int32
	}{
		{"montaria viva", itemSvadilfari, 100, precoSvadilfari},
		{"sem montaria", 0, 0, precoSvadilfari},
		{"gold insuficiente", itemSvadilfari, 0, precoSvadilfari - 1},
	}
	for _, c := range cases {
		t.Run(c.nome, func(t *testing.T) {
			d, w, s, e, npc := mestreFixture(t, itemSvadilfari, 10, c.coin)
			m := world.Item{Index: c.mount}
			if c.mount != 0 {
				putShort(&m.Effects[0], c.hp)
				m.Effects[1].Value = 10
			}
			e.Equip[mountEquipSlot] = m
			d.mountMaster(w, s, e, npc, 1)
			if e.Coin != c.coin {
				t.Errorf("gold = %d, want %d — recusa não cobra", e.Coin, c.coin)
			}
			if e.Equip[mountEquipSlot] != m {
				t.Errorf("montaria = %+v, want %+v intocada", e.Equip[mountEquipSlot], m)
			}
		})
	}
}

func TestMestreCuraCria(t *testing.T) {
	// A cria (2330-2359) is cured by the same NPC (:175 takes 2330..2389).
	d, w, s, e, npc := mestreFixture(t, itemCriaCavaloSN, 10, precoSvadilfari)
	d.mountMaster(w, s, e, npc, 1)
	if hp := mountHP(e.Equip[mountEquipSlot]); hp != mountCureHP {
		t.Errorf("HP da cria = %d, want %d", hp, mountCureHP)
	}
}

func TestMontariaCuradaDevolveOBonus(t *testing.T) {
	// A dead adult lends nothing (mountBonusFor gates on HP); the cure has to put
	// the bonus back in the score on the spot, not on the next refresh.
	d, w, s, e, npc := mestreFixture(t, itemSvadilfari, 10, precoSvadilfari)
	e.Equip[mountEquipSlot].Effects[1].Effect = 120 // nível: dá ataque de verdade
	d.refreshScore(e)
	morta := e.Damage
	d.mountMaster(w, s, e, npc, 1)
	if e.Damage <= morta {
		t.Errorf("dano %d → %d: a montaria curada tem de voltar a somar", morta, e.Damage)
	}
}

// --- pelo fio: o Merchant 58 chega ao handler ---

func mestreMontariaTemplate() []byte {
	if b, err := os.ReadFile(filepath.Join("..", "..", "..", "Release", "TMsrv", "run", "npc", "M._de_Montaria")); err == nil && len(b) == 816 {
		return b
	}
	b := make([]byte, 816)
	copy(b[0:16], "M._de_Montaria")
	const cs = 92
	b[cs+12] = merchantMountMaster
	binary.LittleEndian.PutUint32(b[cs+24:], 1) // Hp (alive)
	return b
}

func startServerMestreMontaria(t *testing.T, st world.CharacterState) (string, func(), int) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := New(Config{Log: log, ItemPrices: map[int]int32{itemSvadilfari: precoSvadilfari}})
	db := newDB()
	db.loadResult = st
	w := world.New(world.Config{GridDim: world.DefaultGridDim}, log, db, d.Handle)
	npcID := w.SpawnMob(mestreMontariaTemplate(), st.X+1, st.Y+1)
	if npcID < 0 {
		t.Fatal("failed to spawn M._de_Montaria")
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { _ = w.Serve(ctx, ln); close(done) }()
	return ln.Addr().String(), func() {
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("server did not stop")
		}
	}, npcID
}

// TestMestreDeMontariaPeloFio is the bug as the player saw it: clicking the NPC
// did nothing, because Merchant 58 had no route in _MSG_Quest.
func TestMestreDeMontariaPeloFio(t *testing.T) {
	st := world.CharacterState{Slot: 0, Name: "Hero", X: 2116, Y: 2080, HP: 1000, MaxHP: 1000, Level: 100, Coin: precoSvadilfari}
	m := world.Item{Index: itemSvadilfari}
	m.Effects[1].Value = 10 // vidas; HP 0 = morta
	st.Equip[mountEquipSlot] = m
	addr, stop, npcID := startServerMestreMontaria(t, st)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	// O clique: o NPC diz o preço, em nome dele.
	send(t, c, protocol.MsgQuest, protocol.EncodeStandardParm2(int32(npcID), 0))
	h, fala, ok := expectHeader(t, c, protocol.MsgMessageChat)
	if !ok {
		t.Fatal("o Mestre de Montaria não respondeu ao clique")
	}
	if int(h.ID) != npcID {
		t.Errorf("fala veio do id %d, want o NPC %d", h.ID, npcID)
	}
	if !bytes.Contains(fala, []byte("850000")) {
		t.Errorf("fala = %q, want o preço 850000", cstr(fala))
	}

	// A confirmação: a montaria volta com 20 de HP.
	send(t, c, protocol.MsgQuest, protocol.EncodeStandardParm2(int32(npcID), 1))
	got := equipItem(t, c)
	if got.Index != itemSvadilfari || mountHP(got) != mountCureHP {
		t.Errorf("montaria depois da cura = %d HP %d, want %d HP %d", got.Index, mountHP(got), itemSvadilfari, mountCureHP)
	}
}
