package handler

import (
	"context"
	"encoding/binary"
	"io"
	"log/slog"
	"net"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/content"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

const itemPedraAmunra = 3464

// amunraRules is the item catalog at one moment: the Pedra Amunra granting
// `bonus` to each attribute. The refine scaling of 2026-08-17 doubled the +9
// stone from 100 to 200; any later catalog or formula change does the same.
func amunraRules(bonus int16) map[int][]content.BaseEffect {
	return map[int][]content.BaseEffect{itemPedraAmunra: {
		{Eff: efStr, Val: bonus}, {Eff: efInt, Val: bonus}, {Eff: efDex, Val: bonus}, {Eff: efCon, Val: bonus},
	}}
}

// loginWithRules logs the character in under the given catalog, returns the
// Str/Int/Dex/Con the client is shown and what the logout saved.
func loginWithRules(t *testing.T, rules map[int][]content.BaseEffect, st world.CharacterState) ([4]int16, world.CharacterSave) {
	t.Helper()
	db := newDB()
	db.loadResult = st
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := New(Config{Log: log, ItemEffects: rules, ItemPos: map[int]int{itemPedraAmunra: accessoryPosMask}})
	w := world.New(world.Config{GridDim: 16}, log, db, d.Handle)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { _ = w.Serve(ctx, ln); close(done) }()
	defer func() { cancel(); <-done }()
	c := loginAndSelect(t, ln.Addr().String())
	defer c.Close()

	send(t, c, protocol.MsgCharacterLogin, (&protocol.MsgCharacterLoginBody{}).Encode())
	var shown [4]int16
	for got := false; !got; {
		ty, p := readAny(t, c)
		if ty == protocol.MsgUpdateScore {
			for i := range shown {
				shown[i] = int16(binary.LittleEndian.Uint16(p[32+2*i:])) // STRUCT_SCORE Str..Con
			}
			got = true
		}
	}
	send(t, c, protocol.MsgCharacterLogout, nil)
	for {
		if ty, _ := readAny(t, c); ty == protocol.MsgCNFCharacterLogout {
			break
		}
	}
	save, n := db.lastSavedChar()
	if n == 0 {
		t.Fatal("logout saved nothing")
	}
	return shown, save
}

// stateFromSave is what the next login reads back from the database.
func stateFromSave(s world.CharacterSave, equip [world.MaxEquip]world.Item) world.CharacterState {
	return world.CharacterState{Slot: s.Slot, Name: "Hero", Class: 3, Level: int(s.Level), X: 2100, Y: 2100,
		HP: s.HP, MaxHP: s.MaxHP, MP: s.MP, MaxMP: s.MaxMP, ClassMaster: classMasterMortal,
		Str: s.Str, Int: s.Int, Dex: s.Dex, Con: s.Con,
		HasBase: true, BaseStr: s.BaseStr, BaseInt: s.BaseInt, BaseDex: s.BaseDex, BaseCon: s.BaseCon,
		BaseMaxHP: s.BaseMaxHP, BaseMaxMP: s.BaseMaxMP, Equip: equip}
}

func amunraEquip() [world.MaxEquip]world.Item {
	var eq [world.MaxEquip]world.Item
	eq[9] = world.Item{Index: itemPedraAmunra}
	return eq
}

// The player report of 2026-10-08: a Huntress built 12/12/2512/860 wore a
// Pedra Amunra saved at +100 and logged in after the stone became +200. The
// base used to be rebuilt as stored - today's bonus (112 - 200 = -88) and kept
// for good. With the base saved, the rule change moves only the bonus.
func TestBaseScoreSurvivesItemRuleChange(t *testing.T) {
	base := [4]int16{12, 12, 2512, 860}
	st := world.CharacterState{Slot: 0, Name: "Hero", Class: 3, Level: 399, X: 2100, Y: 2100,
		HP: 2000, MaxHP: 2000, MP: 500, MaxMP: 500, ClassMaster: classMasterMortal,
		// CurrentScore saved under the old rule (+100), base saved alongside.
		Str: base[0] + 100, Int: base[1] + 100, Dex: base[2] + 100, Con: base[3] + 100,
		HasBase: true, BaseStr: base[0], BaseInt: base[1], BaseDex: base[2], BaseCon: base[3],
		BaseMaxHP: 2000, BaseMaxMP: 500, Equip: amunraEquip()}

	shown, save := loginWithRules(t, amunraRules(200), st)
	for i, want := range base {
		if shown[i] != want+200 {
			t.Errorf("attribute %d shown %d, want base %d + 200", i, shown[i], want)
		}
	}
	if got := [4]int16{save.BaseStr, save.BaseInt, save.BaseDex, save.BaseCon}; got != base {
		t.Fatalf("saved base %v, want %v", got, base)
	}

	// The rule changes again (and the stone comes off): the base still holds.
	shown, save = loginWithRules(t, amunraRules(300), stateFromSave(save, amunraEquip()))
	for i, want := range base {
		if shown[i] != want+300 {
			t.Errorf("after a second change: attribute %d shown %d, want %d", i, shown[i], want+300)
		}
	}
	shown, save = loginWithRules(t, amunraRules(300), stateFromSave(save, [world.MaxEquip]world.Item{}))
	if shown != base {
		t.Fatalf("without the stone: shown %v, want the base %v", shown, base)
	}
	if got := [4]int16{save.BaseStr, save.BaseInt, save.BaseDex, save.BaseCon}; got != base {
		t.Fatalf("base drifted to %v", got)
	}
}

// A row saved before the base was persisted (HasBase false) still derives it
// once from the CurrentScore, and that login's save stores it, so from then on
// the next rule change can no longer move it.
func TestBaseScoreLegacyRowDerivesOnceThenPersists(t *testing.T) {
	st := world.CharacterState{Slot: 0, Name: "Hero", Class: 3, Level: 399, X: 2100, Y: 2100,
		HP: 2000, MaxHP: 2000, MP: 500, MaxMP: 500, ClassMaster: classMasterMortal,
		Str: 112, Int: 112, Dex: 2612, Con: 960, Equip: amunraEquip()}

	_, save := loginWithRules(t, amunraRules(100), st)
	want := [4]int16{12, 12, 2512, 860}
	if got := [4]int16{save.BaseStr, save.BaseInt, save.BaseDex, save.BaseCon}; got != want {
		t.Fatalf("derived and saved base %v, want %v", got, want)
	}
	shown, _ := loginWithRules(t, amunraRules(200), stateFromSave(save, amunraEquip()))
	for i := range want {
		if shown[i] != want[i]+200 {
			t.Errorf("attribute %d shown %d, want %d", i, shown[i], want[i]+200)
		}
	}
}
