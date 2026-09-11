package handler

import (
	"net"
	"strings"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// Every table is a partition of rand()%100: strictly rising thresholds that end
// exactly at 100, and a real item on every line.
func TestChestTablesAreWellFormed(t *testing.T) {
	for chest, table := range chestTables {
		last := 0
		for i, p := range table {
			if p.upTo <= last {
				t.Errorf("baú %d linha %d: limite %d não sobe (anterior %d)", chest, i, p.upTo, last)
			}
			if p.item.Empty() {
				t.Errorf("baú %d linha %d: prêmio vazio", chest, i)
			}
			last = p.upTo
		}
		if last != 100 {
			t.Errorf("baú %d: a tabela termina em %d, want 100", chest, last)
		}
	}
}

// The Baú do Âmago Especial gives only the Svadilfari's and the Sleipnir's own
// âmago — never 2400/2411, which are the Andaluz N's and the Unicórnio's now.
func TestAmagoEspecialGivesTheMythicAmagos(t *testing.T) {
	for roll := 0; roll < 100; roll++ {
		got := drawChest(chestTables[3218], roll)
		if got.Index != itemAmagoSvadilfari && got.Index != itemAmagoSleipnir {
			t.Fatalf("roll %d: âmago %d, want 2417 ou 2418", roll, got.Index)
		}
		if n := itemAmount(got); n != 10 && n != 20 {
			t.Fatalf("roll %d: pilha de %d, want 10 ou 20", roll, n)
		}
	}
}

func TestDrawChestFollowsTheLegacyThresholds(t *testing.T) {
	for _, c := range []struct {
		roll int
		want int16
	}{
		{0, 5334}, {24, 5334}, {25, 5335}, {74, 5336}, {99, 5337},
	} {
		if got := drawChest(chestTables[itemBauPedraSecreta], c.roll).Index; got != c.want {
			t.Errorf("Pedra Secreta roll %d = %d, want %d", c.roll, got, c.want)
		}
	}
	// Runas: 87 and up is three more baús.
	if got := drawChest(runeTable, 87); got.Index != itemBauRunas || itemAmount(got) != 3 {
		t.Errorf("runas roll 87 = %d×%d, want 3219×3", got.Index, itemAmount(got))
	}
	if got := drawChest(runeTable, 72).Index; got != 5120 { // Isa: 72..77
		t.Errorf("runas roll 72 = %d, want 5120 (Isa)", got)
	}
}

func chestDB(chest int16, amount uint8, fill bool) *fakeDB {
	db := newDB()
	st := world.CharacterState{Slot: 0, Name: "Hero", X: 5, Y: 5, HP: 1000, MaxHP: 1000, Level: 100}
	it := world.Item{Index: chest}
	if amount > 1 {
		it.Effects[0] = world.Effect{Effect: efAmount, Value: amount}
	}
	st.Carry[0] = it
	if fill {
		for i := 1; i < baseCarrySlots; i++ {
			st.Carry[i] = world.Item{Index: 400}
		}
	}
	db.loadResult = st
	return db
}

// carrySlot drains until the SendItem for the given carry slot.
func carrySlot(t *testing.T, c net.Conn, slot int) world.Item {
	t.Helper()
	for i := 0; i < 16; i++ {
		p := expect(t, c, protocol.MsgSendItem)
		if le16(p[0:2]) == uint16(world.ItemPlaceCarry) && le16(p[2:4]) == uint16(slot) {
			return decodeItem(p[4:])
		}
	}
	t.Fatalf("nenhum SendItem para o slot %d da bolsa", slot)
	return world.Item{}
}

// The last baú of a stack is replaced by its prize in the same slot.
func TestOpenPedraSecreta(t *testing.T) {
	addr, stop := startServerClockVol(t, chestDB(itemBauPedraSecreta, 1, false), map[int]int{itemBauPedraSecreta: 210})
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	useCarry(t, c, 0)
	got := carrySlot(t, c, 0)
	if got.Index < 5334 || got.Index > 5337 {
		t.Fatalf("slot 0 = %d, want uma Pedra Secreta (5334-5337)", got.Index)
	}
	if msg := decodePanel(expect(t, c, protocol.MsgMessagePanel)); !strings.HasPrefix(msg, "!Chegou o item") {
		t.Errorf("mensagem = %q, want \"!Chegou o item …\"", msg)
	}
}

// With more than one baú, one is spent and the prize goes to a free slot.
func TestOpenChestFromAStack(t *testing.T) {
	addr, stop := startServerClockVol(t, chestDB(itemBauPedraSecreta, 3, false), map[int]int{itemBauPedraSecreta: 210})
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	useCarry(t, c, 0)
	if left := carrySlot(t, c, 0); left.Index != itemBauPedraSecreta || itemAmount(left) != 2 {
		t.Errorf("slot 0 = %d×%d, want 3212×2", left.Index, itemAmount(left))
	}
	if prize := carrySlot(t, c, 1); prize.Index < 5334 || prize.Index > 5337 {
		t.Errorf("slot 1 = %d, want uma Pedra Secreta", prize.Index)
	}
}

// A full bag refuses and keeps the baú — the legacy would spend it and drop the
// prize on the floor.
func TestOpenChestFullBagKeepsTheChest(t *testing.T) {
	addr, stop := startServerClockVol(t, chestDB(itemBauPedraSecreta, 2, true), map[int]int{itemBauPedraSecreta: 210})
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	useCarry(t, c, 0)
	if msg := decodePanel(expect(t, c, protocol.MsgMessagePanel)); msg != msgFullCarry {
		t.Errorf("mensagem = %q, want %q", msg, msgFullCarry)
	}
	if kept := carrySlot(t, c, 0); kept.Index != itemBauPedraSecreta || itemAmount(kept) != 2 {
		t.Errorf("slot 0 = %d×%d, want 3212×2 intacto", kept.Index, itemAmount(kept))
	}
}

// 5750 carries the Kappa potion's volatile (200); it must open as a baú, not be
// drunk as a buff.
func TestBauRunas5750OpensAsAChest(t *testing.T) {
	addr, stop := startServerClockVol(t, chestDB(itemBauRunas3, 1, false), map[int]int{itemBauRunas3: 200})
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	useCarry(t, c, 0)
	got := carrySlot(t, c, 0)
	if (got.Index < 5110 || got.Index > 5133) && got.Index != itemBauRunas {
		t.Errorf("slot 0 = %d, want uma runa (5110-5133) ou mais Baús de Runas", got.Index)
	}
}

// The mount from the Baú da Montaria is born like every other adult: vitality
// rolled in the server's band, not the legacy's flat 20.
func TestBauMontariaRollsVitality(t *testing.T) {
	addr, stop := startServerClockVol(t, chestDB(3217, 1, false), map[int]int{3217: 210})
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	useCarry(t, c, 0)
	got := carrySlot(t, c, 0)
	if got.Index != itemSleipnir && got.Index != itemSvadilfari {
		t.Fatalf("slot 0 = %d, want Sleipnir ou Svadilfari", got.Index)
	}
	if v := got.Effects[1].Value; v < adultVitalityMin || v > adultVitalityMax {
		t.Errorf("vitalidade = %d, want %d-%d", v, adultVitalityMin, adultVitalityMax)
	}
}
