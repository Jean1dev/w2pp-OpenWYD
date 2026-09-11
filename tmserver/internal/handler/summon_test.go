package handler

import (
	"context"
	"encoding/binary"
	"io"
	"log/slog"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/content"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// TestSummonCount trava o teto por criatura em Evocação 300, onde o conjunto
// sai cheio desde 2026-09-11 (antes 320), e o degrau abaixo dele.
//
// O teto é o que impede uma maestria alta de estourar o desenho: sem ele a
// conta continua crescendo e o Condor, com o menor divisor de todos, enche a
// party sozinho.
func TestSummonCount(t *testing.T) {
	tests := []struct {
		nome          string
		iv, evo, want int
	}{
		// O teto de cada criatura, em Evocação 300.
		{"Condor no teto", 1, 300, 12},
		{"Javali no teto", 2, 300, 10},
		{"Lobo no teto", 3, 300, 10},
		{"Urso no teto", 4, 300, 9},
		{"Tigre no teto", 5, 300, 8},
		{"Gorila no teto", 6, 300, 7},
		{"Dragão no teto", 7, 300, 5},
		{"Succubus no teto", 8, 300, 4},

		// O caso do jogo: um BM logo abaixo de 300 ainda sai um a menos.
		{"Succubus em 299", 8, 299, 3},
		{"Dragão em 299", 7, 299, 4},

		// Maestria no máximo não passa do teto.
		{"Condor no máximo da maestria", 1, 400, 12},
		{"Tigre no máximo da maestria", 5, 400, 8},
		{"Succubus no máximo da maestria", 8, 400, 4},

		// Abaixo do teto a maestria ainda manda.
		{"Condor pela metade", 1, 150, 6},
		{"Dragão pela metade", 7, 150, 2},
		{"Succubus pela metade", 8, 150, 2},

		// Sem maestria nenhuma não sai bicho — o lançamento devolve a mana.
		{"Condor sem Evocação", 1, 0, 0},
		{"Succubus sem Evocação", 8, 0, 0},
		{"Succubus abaixo do primeiro degrau", 8, 74, 0},

		// A Invocação Final não depende da maestria.
		{"Invocação Final", 9, 0, 1},
		{"Invocação Final com maestria", 9, 400, 1},

		// Fora da faixa não invoca nada.
		{"instanceValue inexistente", 10, 400, 0},
		{"instanceValue zero", 0, 400, 0},
	}
	for _, tt := range tests {
		if got := summonCount(tt.iv, tt.evo); got != tt.want {
			t.Errorf("%s: summonCount(%d, %d) = %d, want %d", tt.nome, tt.iv, tt.evo, got, tt.want)
		}
	}
}

// plainMobTemplate builds an 816-byte STRUCT_MOB with known flat stats and no
// Merchant byte — an ordinary monster. Skill-targeting tests want this one.
func plainMobTemplate(name string) []byte {
	b := make([]byte, 816)
	copy(b[0:16], name)
	const cs = 92
	binary.LittleEndian.PutUint32(b[cs+0:], 10)   // Level
	binary.LittleEndian.PutUint32(b[cs+8:], 20)   // Damage
	binary.LittleEndian.PutUint32(b[cs+16:], 100) // MaxHp
	binary.LittleEndian.PutUint32(b[cs+24:], 100) // Hp
	binary.LittleEndian.PutUint16(b[cs+34:], 99)  // Int: never hesitates in battle
	return b
}

// summonTemplate is plainMobTemplate plus the Merchant byte every real
// BaseSummon file carries (16; Dragao_Negro ships 64).
//
// The byte is the whole point of this helper existing separately. nonCombatNPC
// treats ANY non-zero Merchant as a service NPC, and a pet marked that way drops
// out of runsMobAI — it stops attacking AND stops ticking its own lifespan. The
// summon fixture used to leave it at zero, so every test here passed while the
// real evocations stood around doing nothing in game. Anything exercising the
// summon path must build its template through this.
func summonTemplate(name string) []byte {
	b := plainMobTemplate(name)
	b[92+12] = 16 // CurrentScore.Merchant
	return b
}

// summonDB is a BM ready to evoke: skill 56 learned (bit 56%24=8), Evocação 60
// (BaseSpecial[2] → 2 Condors), Int 100 for the stat scaling.
func summonDB(evocacao int16) *fakeDB {
	db := newDB()
	db.loadResult = world.CharacterState{
		Slot: 0, Name: "Beast", Class: 2, X: 5, Y: 5,
		HP: 1000, MaxHP: 1000, MP: 500, MaxMP: 500, Damage: 200, AC: 40,
		Level: 50, Int: 100, LearnedSkill: 1 << 8,
		BaseSpecial: [4]int16{0, 0, evocacao, 0},
	}
	return db
}

// evokeSpell is the Evocar Condor catalog row (skill 56: InstanceType 11,
// InstanceValue 1 → summon id 0).
func evokeSpell() *content.SkillData {
	return content.NewSkillData([]content.Spell{
		{Index: 56, ManaSpent: 10, InstanceType: 11, InstanceValue: 1, MaxTarget: 1, Name: "Evocar Condor"},
	})
}

// startServerSummon wires a summon-capable server: spells + summon templates +
// the AI tick (10ms), optionally one extra monster spawned before the loop.
func startServerSummon(t *testing.T, db world.Persistence, mob []byte, mobX, mobY int16) (string, func(), *atomic.Uint32) {
	mobs := [][]byte{summonTemplate("Condor")}
	return startServerSummonWith(t, db, evokeSpell(), mobs, mob, mobX, mobY)
}

func startServerSummonWith(t *testing.T, db world.Persistence, spells *content.SkillData, summonMobs [][]byte, mob []byte, mobX, mobY int16) (string, func(), *atomic.Uint32) {
	return startServerSummonTick(t, db, spells, summonMobs, mob, mobX, mobY, 10*time.Millisecond)
}

// startServerSummonTick is startServerSummonWith with the AI-tick period exposed:
// the summon lifespan is 20 ticks of affectTickPeriod, so a slow tick keeps a pet
// alive for the whole test and isolates what the handlers do from what expiry does.
func startServerSummonTick(t *testing.T, db world.Persistence, spells *content.SkillData, summonMobs [][]byte, mob []byte, mobX, mobY int16, tick time.Duration) (string, func(), *atomic.Uint32) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	clock := &atomic.Uint32{}
	clock.Store(serverTime)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := New(Config{Log: log, Spells: spells, SummonMobs: summonMobs})
	w := world.New(world.Config{GridDim: 16, Now: clock.Load}, log, db, d.Handle)
	if mob != nil {
		w.SpawnMobAt(world.MobSpawn{Template: mob, X: mobX, Y: mobY, GenIndex: -1})
	}
	w.SetTickHandler(tick, d.Tick)
	w.SetSessionEndHandler(d.SessionEnd)
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
	}, clock
}

// petFromCreateMob extracts (mobID, name, level, damage, maxHP) from a
// MSG_CreateMob body (offsets: createmob.go).
func petFromCreateMob(payload []byte) (id int, name string, level, damage, maxHP int32) {
	id = int(binary.LittleEndian.Uint16(payload[4:6]))
	name = cstr(payload[6:22])
	level = int32(binary.LittleEndian.Uint32(payload[124:128]))
	damage = int32(binary.LittleEndian.Uint32(payload[132:136]))
	maxHP = int32(binary.LittleEndian.Uint32(payload[140:144]))
	return
}

// collectPets reads frames for up to timeout, returning the pet CreateMobs
// (names ending in the legacy '^' marker).
func collectPets(t *testing.T, c net.Conn, timeout time.Duration) map[int][]byte {
	t.Helper()
	pets := make(map[int][]byte)
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		ty, payload, ok := readMaybeRaw(t, c)
		if !ok || ty != protocol.MsgCreateMob {
			continue
		}
		if _, name, _, _, _ := petFromCreateMob(payload); strings.HasSuffix(name, "^") {
			id, _, _, _, _ := petFromCreateMob(payload)
			pets[id] = payload
		}
	}
	return pets
}

func babyMountItem(index int16, hp uint16) world.Item {
	it := world.Item{Index: index}
	putShort(&it.Effects[0], hp)
	it.Effects[1] = world.Effect{Effect: 5, Value: 10} // mount sanc/life bytes
	it.Effects[2] = world.Effect{Effect: 30, Value: 1} // feed/kill bytes
	return it
}

func babyMountDB(carry, equip world.Item) *fakeDB {
	db := newDB()
	st := world.CharacterState{
		Slot: 0, Name: "Hero", X: 5, Y: 5,
		HP: 1000, MaxHP: 1000, Level: 50,
	}
	st.Carry[0] = carry
	st.Equip[mountEquipSlot] = equip
	db.loadResult = st
	return db
}

func babyMountSummons() [][]byte {
	mobs := make([][]byte, 40)
	mob := summonTemplate("Porco")
	binary.LittleEndian.PutUint16(mob[140:], 315) // Equip[0] face; 315+(2330-2330)
	mobs[10] = mob                                // MountProcess: 2330 - 2320
	return mobs
}

func startServerBabyMount(t *testing.T, db world.Persistence) (string, func(), *atomic.Uint32) {
	return startServerSummonWith(t, db, nil, babyMountSummons(), nil, 0, 0)
}

func expectPetRemove(t *testing.T, c net.Conn, petID int) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		h, _, ok := readMaybeHeaderRaw(t, c)
		if !ok {
			continue
		}
		if h.Type == protocol.MsgRemoveMob && int(h.ID) == petID {
			return
		}
	}
	t.Fatalf("no RemoveMob for pet %d", petID)
}

func TestBabyMountSpawnsOnEquip(t *testing.T) {
	addr, stop, _ := startServerBabyMount(t, babyMountDB(babyMountItem(2330, 80), world.Item{}))
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	tradeItemFrame(t, c, world.ItemPlaceCarry, 0, world.ItemPlaceEquip, mountEquipSlot, 0)
	expect(t, c, protocol.MsgTradingItem)
	expect(t, c, protocol.MsgSendItem)
	expect(t, c, protocol.MsgSendItem)
	ue := expect(t, c, protocol.MsgUpdateEquip)
	if got := le16(ue[mountEquipSlot*2:]); got != 2330 {
		t.Fatalf("mount visual = %d, want baby mount item 2330", got)
	}

	pets := collectPets(t, c, time.Second)
	if len(pets) != 1 {
		t.Fatalf("baby mount pets = %d, want 1", len(pets))
	}
	for _, payload := range pets {
		if name := cstr(payload[6:22]); name != "Porco^" {
			t.Errorf("baby mount name = %q, want Porco^", name)
		}
		if face := le16(payload[22:24]); face != 315 {
			t.Errorf("baby mount face = %d, want 315", face)
		}
		if createType := le16(payload[172:174]); createType&3 != 3 {
			t.Errorf("baby mount CreateType = %#x, want summon appear bits", createType)
		}
		if hp := int32(le(payload[148:152])); hp != 80 {
			t.Errorf("baby mount HP = %d, want item HP 80", hp)
		}
	}
}

func TestBabyMountDeadDoesNotSpawn(t *testing.T) {
	addr, stop, _ := startServerBabyMount(t, babyMountDB(babyMountItem(2330, 0), world.Item{}))
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	tradeItemFrame(t, c, world.ItemPlaceCarry, 0, world.ItemPlaceEquip, mountEquipSlot, 0)
	expect(t, c, protocol.MsgTradingItem)
	expect(t, c, protocol.MsgSendItem)
	expect(t, c, protocol.MsgSendItem)
	expect(t, c, protocol.MsgUpdateEquip)

	if pets := collectPets(t, c, 500*time.Millisecond); len(pets) != 0 {
		t.Fatalf("dead baby mount spawned %d pet(s), want none", len(pets))
	}
}

func TestBabyMountUnequipDespawnsPet(t *testing.T) {
	addr, stop, _ := startServerBabyMount(t, babyMountDB(babyMountItem(2330, 80), world.Item{}))
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	tradeItemFrame(t, c, world.ItemPlaceCarry, 0, world.ItemPlaceEquip, mountEquipSlot, 0)
	expect(t, c, protocol.MsgTradingItem)
	expect(t, c, protocol.MsgSendItem)
	expect(t, c, protocol.MsgSendItem)
	expect(t, c, protocol.MsgUpdateEquip)
	pets := collectPets(t, c, time.Second)
	if len(pets) != 1 {
		t.Fatalf("baby mount pets = %d, want 1", len(pets))
	}
	petID := 0
	for id := range pets {
		petID = id
	}

	tradeItemFrame(t, c, world.ItemPlaceEquip, mountEquipSlot, world.ItemPlaceCarry, 1, 0)
	expect(t, c, protocol.MsgTradingItem)
	expect(t, c, protocol.MsgSendItem)
	expect(t, c, protocol.MsgSendItem)
	expect(t, c, protocol.MsgUpdateEquip)
	expectPetRemove(t, c, petID)
}

func TestBabyMountDoesNotExpireLikeSkillSummon(t *testing.T) {
	addr, stop, _ := startServerBabyMount(t, babyMountDB(babyMountItem(2330, 80), world.Item{}))
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	tradeItemFrame(t, c, world.ItemPlaceCarry, 0, world.ItemPlaceEquip, mountEquipSlot, 0)
	expect(t, c, protocol.MsgTradingItem)
	expect(t, c, protocol.MsgSendItem)
	expect(t, c, protocol.MsgSendItem)
	expect(t, c, protocol.MsgUpdateEquip)
	pets := collectPets(t, c, time.Second)
	if len(pets) != 1 {
		t.Fatalf("baby mount pets = %d, want 1", len(pets))
	}
	petID := 0
	for id := range pets {
		petID = id
	}

	deadline := time.Now().Add(2500 * time.Millisecond)
	for time.Now().Before(deadline) {
		h, _, ok := readMaybeHeaderRaw(t, c)
		if !ok {
			continue
		}
		if h.Type == protocol.MsgRemoveMob && int(h.ID) == petID {
			t.Fatalf("baby mount pet %d expired; item-backed summons must not use Type-24 lifespan", petID)
		}
	}
}

func TestBabyMountSpawnsOnLogin(t *testing.T) {
	addr, stop, _ := startServerBabyMount(t, babyMountDB(world.Item{}, babyMountItem(2330, 80)))
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	pets := collectPets(t, c, time.Second)
	if len(pets) != 1 {
		t.Fatalf("login baby mount pets = %d, want 1", len(pets))
	}
}

func TestBabyMountGrowsWhenPetKillsMob(t *testing.T) {
	mount := babyMountItem(2330, 80)
	mount.Effects[1].Effect = 5
	mount.Effects[2].Value = 1
	db := babyMountDB(world.Item{}, mount)

	mob := make([]byte, 816)
	copy(mob[0:16], "Rato")
	mob[16] = 5 // hostile clan
	const cs = 92
	binary.LittleEndian.PutUint32(mob[cs+0:], 6) // higher than mount level 5
	binary.LittleEndian.PutUint32(mob[cs+8:], 1)
	binary.LittleEndian.PutUint32(mob[cs+16:], 1)
	binary.LittleEndian.PutUint32(mob[cs+24:], 1)
	binary.LittleEndian.PutUint16(mob[cs+34:], 99)

	addr, stop, clock := startServerSummonWith(t, db, nil, babyMountSummons(), mob, 4, 3)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()
	actionFrameAt(t, c, serverTime, 5, 5)

	mobID := world.MaxUser
	petID := -1
	petStruck := false
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		clock.Add(300)
		h, payload, ok := readMaybeHeaderRaw(t, c)
		if !ok {
			continue
		}
		switch h.Type {
		case protocol.MsgCreateMob:
			if id, name, _, _, _ := petFromCreateMob(payload); strings.HasSuffix(name, "^") {
				petID = id
			}
		case protocol.MsgAttack, protocol.MsgAttackOne:
			var b protocol.MsgAttackBody
			if err := b.Decode(payload); err != nil {
				continue
			}
			if petID >= 0 && int(b.AttackerID) == petID {
				for _, dam := range b.Dam {
					if int(dam.TargetID) == mobID {
						petStruck = true
					}
				}
			}
		case protocol.MsgRemoveMob:
			if int(h.ID) == mobID && petStruck {
				send(t, c, protocol.MsgCharacterLogout, nil)
				expect(t, c, protocol.MsgCNFCharacterLogout)
				save, n := db.lastSavedChar()
				if n == 0 {
					t.Fatal("character was not saved on logout")
				}
				savedMount, ok := savedItemAt(save.Equip, mountEquipSlot)
				if !ok {
					t.Fatal("saved equip slot 14 is empty; want baby mount")
				}
				if savedMount.Eff2 != 5 {
					t.Fatalf("saved mount level = %d, want 5", savedMount.Eff2)
				}
				if savedMount.EffV3 != 2 {
					t.Fatalf("saved mount growth = %d, want 2 after pet kill", savedMount.EffV3)
				}
				return
			}
		}
	}
	switch {
	case petID < 0:
		t.Fatal("no baby mount pet was summoned")
	case !petStruck:
		t.Fatal("baby mount pet never attacked the monster")
	default:
		t.Fatal("monster never died to the baby mount pet")
	}
}

// TestEvocationSpawnsScaledSummons is the happy path: Evocação 60 evokes two
// Condors, each owner-scaled per pSummonBonus[0] — Damage 20 + Int·80% +
// Evo·300% = 280, MaxHp 100 + Int·100% + Evo·400% = 440 — at the owner's level,
// named with the pet marker.
func TestEvocationSpawnsScaledSummons(t *testing.T) {
	addr, stop, _ := startServerSummon(t, summonDB(60), nil, 0, 0)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	skillAttackFrame(t, c, serverTime, 1, 56, damSkill)

	pets := collectPets(t, c, time.Second)
	if len(pets) != 2 {
		t.Fatalf("pets spawned = %d, want 2 (Evocação 60 ÷ 25)", len(pets))
	}
	for id, payload := range pets {
		if id < world.MaxUser {
			t.Errorf("pet id %d in the player range", id)
		}
		_, name, lvl, dmg, hp := petFromCreateMob(payload)
		if name != "Condor^" {
			t.Errorf("pet name = %q, want Condor^", name)
		}
		if lvl != 50 {
			t.Errorf("pet level = %d, want the owner's 50", lvl)
		}
		// O Int do dono não entra mais na conta: summonBonus tem a parte do Int
		// zerada, então o que sobra é a base do template mais a Evocação. O dono
		// deste teste tem Int 100 e não muda nada aqui, o que é o ponto.
		if dmg != 529 {
			t.Errorf("pet damage = %d, want 529 (base 20 + 60·849%%)", dmg)
		}
		if hp != 3839 {
			t.Errorf("pet maxHP = %d, want 3839 (base 100 + 60·6232%%)", hp)
		}
	}
}

// TestEvocationNaoOcupaSlotDeMembro: o pet nasce, aparece no chão, e NÃO entra
// na lista de grupo do cliente.
//
// Contrato invertido de propósito (petsNoPainelDeGrupo). A versão anterior deste
// teste exigia o CNFAddParty do pet, que é o que o legado manda
// (Server.cpp:3224-3229) — e é justamente o que enchia os doze slots de membro
// com os bichos do próprio dono, sem sobrar lugar para gente.
//
// O pet continua na PartyList do líder no servidor: é lá que mora o vínculo, e
// a contagem da re-invocação depende dele. O que mudou é só o que sai no fio.
func TestEvocationNaoOcupaSlotDeMembro(t *testing.T) {
	addr, stop, _ := startServerSummon(t, summonDB(30), nil, 0, 0)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	skillAttackFrame(t, c, serverTime, 1, 56, damSkill)

	nasceu := false
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		ty, payload, ok := readMaybeRaw(t, c)
		if !ok {
			continue
		}
		switch ty {
		case protocol.MsgCreateMob:
			if _, name, _, _, _ := petFromCreateMob(payload); strings.HasSuffix(name, "^") {
				nasceu = true
			}
		case protocol.MsgCNFAddParty:
			if len(payload) < 10 {
				continue
			}
			if partyID := binary.LittleEndian.Uint16(payload[8:10]); int(partyID) >= world.MaxUser {
				t.Fatalf("o pet %d entrou na lista de grupo do cliente e comeu um slot de membro", partyID)
			}
		}
	}
	if !nasceu {
		t.Error("nenhum pet apareceu no chão; o teste não chegou a exercer nada")
	}
}

// TestEvocationRecastReplacesTheSet: re-casting wipes what is out and summons
// the whole set again beside the caster.
//
// This asserts a DELIBERATE divergence, and the old test asserted the opposite
// ("second cast spawns nothing, the existing pet counts"). The legacy tops up
// and teleports strays home with the Effect=8 recall jump
// (Server.cpp:3008-3020); this port never carried that recall, so a re-cast
// could not bring anything back and the set just drifted. Re-summoning whole is
// what the recall was reaching for, and it is what keeps the head count honest:
// the set is always exactly `count`, never an accumulation.
func TestEvocationRecastReplacesTheSet(t *testing.T) {
	addr, stop, _ := startServerSummon(t, summonDB(30), nil, 0, 0)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	skillAttackFrame(t, c, serverTime, 1, 56, damSkill)
	primeiro := collectPets(t, c, 500*time.Millisecond)
	if len(primeiro) != 1 {
		t.Fatalf("first cast pets = %d, want 1", len(primeiro))
	}

	skillAttackFrame(t, c, serverTime+1000, 1, 56, damSkill)
	segundo := collectPets(t, c, 500*time.Millisecond)
	if len(segundo) != 1 {
		t.Fatalf("re-cast spawned %d pets, want 1 — the set comes back whole", len(segundo))
	}
	// And it is a NEW creature: the old one was reaped, not kept and topped up.
	for id := range segundo {
		if _, jaExistia := primeiro[id]; jaExistia {
			t.Errorf("pet %d survived the re-cast; the old set has to go", id)
		}
	}
}

// TestEvocationRefundsWhenNothingSpawns: Evocação below the threshold spawns
// nothing, so the cast's mana comes back (_MSG_Attack.cpp:830-834) — the attack
// echo carries the untouched MP @40.
func TestEvocationRefundsWhenNothingSpawns(t *testing.T) {
	addr, stop, _ := startServerSummon(t, summonDB(20), nil, 0, 0)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	skillAttackFrame(t, c, serverTime, 1, 56, damSkill)

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		ty, payload, ok := readMaybeRaw(t, c)
		if !ok {
			continue
		}
		if ty == protocol.MsgCreateMob {
			if _, name, _, _, _ := petFromCreateMob(payload); strings.HasSuffix(name, "^") {
				t.Fatalf("pet %q spawned with Evocação 20, want none", name)
			}
			continue
		}
		if ty != protocol.MsgAttack {
			continue
		}
		if mp := int32(binary.LittleEndian.Uint32(payload[40:44])); mp != 500 {
			t.Fatalf("post-cast MP = %d, want 500 (failed evocation must refund)", mp)
		}
		return
	}
	t.Fatal("no attack echo received")
}

// TestSummonExpires: the Type-24 lifespan (20 affect ticks) runs out and the
// pet is removed (DeleteMob(idx,3), Server.cpp:5843).
func TestSummonExpires(t *testing.T) {
	// Evocação 30 → exactly one pet, keeping the wire quiet.
	addr, stop, _ := startServerSummon(t, summonDB(30), nil, 0, 0)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	skillAttackFrame(t, c, serverTime, 1, 56, damSkill)
	pets := collectPets(t, c, 500*time.Millisecond)
	if len(pets) != 1 {
		t.Fatalf("pets = %d, want 1", len(pets))
	}
	var petID int
	for id := range pets {
		petID = id
	}

	// 20 lifespan decrements at one per 8 world ticks (10ms each) ≈ 1.6s.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		h, _, ok := readMaybeHeaderRaw(t, c)
		if !ok {
			continue
		}
		if h.Type == protocol.MsgRemoveMob && int(h.ID) == petID {
			return
		}
	}
	t.Fatal("summon never expired (no RemoveMob for the pet)")
}

// TestSummonGoneAfterRelogin: leaving to the character screen orphans the pet;
// the next tick despawns it, so re-entering the world finds no pet (the lazy
// owner-gone cleanup in summonTick).
func TestSummonGoneAfterRelogin(t *testing.T) {
	addr, stop, _ := startServerSummon(t, summonDB(30), nil, 0, 0)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	skillAttackFrame(t, c, serverTime, 1, 56, damSkill)
	if pets := collectPets(t, c, 500*time.Millisecond); len(pets) != 1 {
		t.Fatalf("pets = %d, want 1", len(pets))
	}

	send(t, c, protocol.MsgCharacterLogout, nil)
	for { // drain until the logout confirmation
		ty, _, ok := readMaybeRaw(t, c)
		if !ok {
			t.Fatal("no logout confirmation")
		}
		if ty == protocol.MsgCNFCharacterLogout {
			break
		}
	}
	time.Sleep(100 * time.Millisecond) // a few AI ticks: the orphaned pet despawns

	var body protocol.MsgCharacterLoginBody
	send(t, c, protocol.MsgCharacterLogin, body.Encode())
	if pets := collectPets(t, c, 500*time.Millisecond); len(pets) != 0 {
		t.Fatalf("pets after relogin = %d, want 0 (owner-gone cleanup)", len(pets))
	}
}

// --- issue #234: summons must not outlive the party bond that holds them ---

// trackPets folds the pet lifecycle frames arriving over timeout into live: a pet
// CreateMob adds an id, a RemoveMob (HEADER.ID) drops it. What remains in live is
// what the client still has on screen.
func trackPets(t *testing.T, c net.Conn, timeout time.Duration, live map[int]bool) map[int]bool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		h, payload, ok := readMaybeHeaderRaw(t, c)
		if !ok {
			continue
		}
		switch h.Type {
		case protocol.MsgCreateMob:
			if id, name, _, _, _ := petFromCreateMob(payload); strings.HasSuffix(name, "^") {
				live[id] = true
			}
		case protocol.MsgRemoveMob:
			delete(live, int(h.ID))
		}
	}
	return live
}

// summonPartySrv is a summon-capable server whose AI tick is slower than any
// assert window, so a pet can only vanish because a party handler removed it
// synchronously — never through the lifespan countdown or summonTick's orphan
// reaper, which would otherwise mask a missing sweep.
func summonPartySrv(t *testing.T, db world.Persistence) (string, func()) {
	t.Helper()
	addr, stop, _ := startServerSummonTick(t, db, evokeSpell(),
		[][]byte{summonTemplate("Condor")}, nil, 0, 0, 5*time.Second)
	return addr, stop
}

// summonPartyDB gives the leader (account 7) Evocação 30 → 1 pet and the member
// (account 11) 60 → 2. The pet budget is shared party-wide — generateSummon counts
// EVERY pet in the leader's PartyList, whoever evoked it (Server.cpp:2991-2997) —
// so the member needs the larger allowance to fit one pet of its own alongside
// the leader's.
func summonPartyDB() *fakeDB {
	db := summonDB(30)
	member := db.loadResult
	member.BaseSpecial[2] = 60
	db.loads = map[int64]world.CharacterState{7: db.loadResult, 11: member}
	return db
}

// evokeOne casts Evocar Condor from conn and returns the single pet's id.
func evokeOne(t *testing.T, c net.Conn, conn int) int {
	t.Helper()
	skillAttackFrame(t, c, serverTime, conn, 56, damSkill)
	pets := collectPets(t, c, 400*time.Millisecond)
	if len(pets) != 1 {
		t.Fatalf("conn %d evoked %d pets, want 1", conn, len(pets))
	}
	for id := range pets {
		return id
	}
	return 0
}

// TestSoloSummonDespawnsOnRemoveParty is the issue-234 repro. A BM holding pets
// has Leader == 0 with the pets in its OWN PartyList, so the leave button routes
// to leaderLeaveParty — which used to zero the list and orphan them.
func TestSoloSummonDespawnsOnRemoveParty(t *testing.T) {
	addr, stop := summonPartySrv(t, summonDB(30))
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	pet := evokeOne(t, c, 1)
	removePartyFrame(t, c, 1)
	// The 5s tick puts expiry (20 decrements, one per 8 ticks) ~13min out, so this
	// RemoveMob can only be the leave.
	expectPetRemove(t, c, pet)
}

// TestEvocationAfterLeavingPartyStaysCapped is the second half of the report
// ("...e fazer novas evocações"): the orphans used to be invisible to
// generateSummon's head count, so a re-cast stacked a whole new set on top.
func TestEvocationAfterLeavingPartyStaysCapped(t *testing.T) {
	addr, stop := summonPartySrv(t, summonDB(30))
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	live := map[int]bool{evokeOne(t, c, 1): true}
	removePartyFrame(t, c, 1)
	trackPets(t, c, 400*time.Millisecond, live)
	if len(live) != 0 {
		t.Fatalf("pets alive after leaving = %v, want none", live)
	}
	skillAttackFrame(t, c, serverTime+1000, 1, 56, damSkill)
	trackPets(t, c, 400*time.Millisecond, live)
	if len(live) != 1 {
		t.Fatalf("pets alive after re-evoking = %d (%v), want 1 — evocations stacked", len(live), live)
	}
}

// partyWithPets: A (conn 1) leads, B (conn 2) joins, each evokes one pet. The
// returned ids are both visible to A, whose stream the asserts read.
func partyWithPets(t *testing.T, addr string) (a, b net.Conn, petA, petB int) {
	t.Helper()
	a = enterWorldAs(t, addr, "tester")
	b = enterWorldAs(t, addr, "tradeb")
	reqPartyFrame(t, a, 1, 2)
	expectPartyFrame(t, b, protocol.MsgSendReqParty)
	acceptPartyFrame(t, b, 1, "Beast")
	drainRaw(t, a)
	drainRaw(t, b)

	petA = evokeOne(t, a, 1)
	drainRaw(t, b)
	skillAttackFrame(t, b, serverTime, 2, 56, damSkill)
	pets := collectPets(t, a, 400*time.Millisecond) // A sees the member's pet appear
	if len(pets) != 1 {
		t.Fatalf("member evoked %d pets as seen by the leader, want 1", len(pets))
	}
	for id := range pets {
		petB = id
	}
	drainRaw(t, b)
	return a, b, petA, petB
}

// TestSummonDespawnsWhenOwnerLeavesParty: a member's pets sit in the LEADER's
// PartyList, so leaving must take them along — and only them (Server.cpp:8185).
func TestSummonDespawnsWhenOwnerLeavesParty(t *testing.T) {
	addr, stop := summonPartySrv(t, summonPartyDB())
	defer stop()
	a, b, petA, petB := partyWithPets(t, addr)
	defer a.Close()
	defer b.Close()

	removePartyFrame(t, b, 2)
	live := trackPets(t, a, 600*time.Millisecond, map[int]bool{petA: true, petB: true})
	if live[petB] {
		t.Errorf("leaver's pet %d survived the party leave", petB)
	}
	if !live[petA] {
		t.Errorf("leader's own pet %d was removed; only the leaver's should go", petA)
	}
}

// TestSummonDespawnsWhenKicked is the kick side of the same sweep.
func TestSummonDespawnsWhenKicked(t *testing.T) {
	addr, stop := summonPartySrv(t, summonPartyDB())
	defer stop()
	a, b, petA, petB := partyWithPets(t, addr)
	defer a.Close()
	defer b.Close()

	removePartyFrame(t, a, 2) // leader kicks the member
	live := trackPets(t, a, 600*time.Millisecond, map[int]bool{petA: true, petB: true})
	if live[petB] {
		t.Errorf("kicked member's pet %d survived", petB)
	}
	if !live[petA] {
		t.Errorf("leader's own pet %d was removed by the kick", petA)
	}
}

// TestSummonDespawnsOnPartyDisband: dissolving takes every pet in the list, not
// just the leader's — the deliberate divergence from Server.cpp:8242, which only
// zeroes Summoner and would leak ownerless mobs here (party.go leaderLeaveParty).
func TestSummonDespawnsOnPartyDisband(t *testing.T) {
	addr, stop := summonPartySrv(t, summonPartyDB())
	defer stop()
	a, b, petA, petB := partyWithPets(t, addr)
	defer a.Close()
	defer b.Close()

	removePartyFrame(t, a, 1) // leader leaves → party dissolves
	live := trackPets(t, a, 600*time.Millisecond, map[int]bool{petA: true, petB: true})
	if len(live) != 0 {
		t.Fatalf("pets alive after the party dissolved = %v, want none", live)
	}
}

// TestSummonPartySlotClearedOnDespawn: o pet sai do mundo quando o vínculo
// acaba, e a linha de grupo sai junto SE ela tiver sido mandada.
//
// As duas metades andam com petsNoPainelDeGrupo: com os pets fora do painel não
// há linha para derrubar, e exigir o RemoveParty seria exigir um pacote que
// ninguém mandou. O RemoveMob é cobrado nos dois modos — é ele que tira a
// criatura do chão.
func TestSummonPartySlotClearedOnDespawn(t *testing.T) {
	addr, stop := summonPartySrv(t, summonDB(30))
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	pet := evokeOne(t, c, 1)
	removePartyFrame(t, c, 1)

	saiuDoChao, saiuDoGrupo := false, false
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) && (!saiuDoChao || (petsNoPainelDeGrupo && !saiuDoGrupo)) {
		h, payload, ok := readMaybeHeaderRaw(t, c)
		if !ok {
			continue
		}
		switch h.Type {
		case protocol.MsgRemoveMob:
			if int(h.ID) == pet {
				saiuDoChao = true
			}
		case protocol.MsgRemoveParty:
			if len(payload) >= 2 && int(protocolLe16(payload[0:2])) == pet {
				saiuDoGrupo = true
			}
		}
	}
	if !saiuDoChao {
		t.Errorf("o pet %d ficou no chão depois de o vínculo acabar", pet)
	}
	if petsNoPainelDeGrupo && !saiuDoGrupo {
		t.Errorf("o pet %d saiu do chão e continuou no painel de grupo", pet)
	}
	if !petsNoPainelDeGrupo && saiuDoGrupo {
		t.Errorf("mandou RemoveParty do pet %d sem nunca ter mandado a linha", pet)
	}
}

// TestSummonAssistsAgainstMob: a monster fight near the owner pulls the pet in
// (commandSummons) — the pet attacks the monster (mob-vs-mob combat) and the
// monster eventually dies to it, despawning with the death RemoveMob.
func TestSummonAssistsAgainstMob(t *testing.T) {
	// A weak hostile monster right next to the player: it engages the player,
	// the player's pet defends.
	mob := make([]byte, 816)
	copy(mob[0:16], "Rato")
	mob[16] = 5 // hostile clan
	const cs = 92
	binary.LittleEndian.PutUint32(mob[cs+0:], 1)   // Level
	binary.LittleEndian.PutUint32(mob[cs+8:], 1)   // Damage (harmless)
	binary.LittleEndian.PutUint32(mob[cs+16:], 50) // MaxHp
	binary.LittleEndian.PutUint32(mob[cs+24:], 50) // Hp
	binary.LittleEndian.PutUint16(mob[cs+34:], 99) // Int

	// (4,3): adjacent to where the pet spawns (freeCellNear picks (4,4)) and two
	// tiles from the player — the pet reaches it without pathing around anyone
	// (the blind no-heights step never sidesteps an occupied cell).
	addr, stop, clock := startServerSummon(t, summonDB(30), mob, 4, 3)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()
	actionFrameAt(t, c, serverTime, 5, 5) // register in the grid next to the mob

	skillAttackFrame(t, c, serverTime, 1, 56, damSkill)

	// One unified watch from cast time — the whole chain (mob strikes owner →
	// pet commanded → pet strikes mob → mob dies) can play out within a few
	// 10ms AI ticks, so no frame may be discarded while "waiting for the pet".
	mobID := world.MaxUser // first NPC slot: the pre-spawned monster
	petID := -1
	petStruck := false
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		// Advance the frozen clock so attack cadences (1000ms) keep firing.
		clock.Add(300)
		h, payload, ok := readMaybeHeaderRaw(t, c)
		if !ok {
			continue
		}
		switch h.Type {
		case protocol.MsgCreateMob:
			if id, name, _, _, _ := petFromCreateMob(payload); strings.HasSuffix(name, "^") {
				petID = id
			}
		case protocol.MsgAttack, protocol.MsgAttackOne:
			var b protocol.MsgAttackBody
			if err := b.Decode(payload); err != nil {
				continue
			}
			if petID >= 0 && int(b.AttackerID) == petID {
				for _, dam := range b.Dam {
					if int(dam.TargetID) == mobID {
						petStruck = true
					}
				}
			}
		case protocol.MsgRemoveMob:
			if int(h.ID) == mobID && petStruck {
				return // the pet fought the monster down
			}
		}
	}
	switch {
	case petID < 0:
		t.Fatal("no pet was summoned")
	case !petStruck:
		t.Fatal("pet never attacked the monster (assist/mob-vs-mob broken)")
	default:
		t.Fatal("monster never died to the pet")
	}
}

// alvoDeTeste monta uma entidade solta com os campos que validTarget lê. Não
// precisa de mundo: validTarget só consulta o World no ramo de alvo jogador, que
// nenhum caso daqui alcança.
func alvoDeTeste(id int, ajusta func(*world.Entity)) *world.Entity {
	e := &world.Entity{ID: id, Mode: world.MobIdle, HP: 100, MaxHP: 100}
	if ajusta != nil {
		ajusta(e)
	}
	return e
}

// TestPetNaoAtacaNpcDeCidadeNemOutroPet tranca as regras de mira do pet.
//
// Elas existiam em validTarget desde sempre, mas eram código MORTO: o pet nunca
// entrava no laço de IA (o Merchant do template o marcava como NPC de serviço),
// então nada disso rodava em servidor nenhum. Consertar aquilo é o que passou a
// carregar estas guardas de verdade — e um pet que ataca gente dentro da cidade
// é bem pior do que um pet parado.
func TestPetNaoAtacaNpcDeCidadeNemOutroPet(t *testing.T) {
	log := slog.New(slog.DiscardHandler)
	d := New(Config{Log: log})
	w := world.New(world.Config{GridDim: 16}, log, nil, d.Handle)

	// O pet: Summoner preenchido é o que o identifica como pet em validTarget.
	pet := alvoDeTeste(world.MaxUser+1, func(e *world.Entity) {
		e.Summoner = 3
		e.Clan = summonClan
		e.SegmentX, e.SegmentY = 5, 5
		e.X, e.Y = 5, 5
	})

	casos := []struct {
		nome  string
		alvo  *world.Entity
		quer  bool
		porqu string
	}{
		// Sem caso "jogador" aqui, de propósito. validTarget devolve false para um
		// pet mirando jogador (mobai.go, `e.Summoner != 0`), mas um alvo sem sessão
		// viva JÁ cai fora duas linhas depois, no SessionMode — então o caso
		// passaria com a guarda removida, e um teste que não sabe falhar é pior do
		// que teste nenhum: é garantia falsa, que foi o que deixou este bug chegar
		// em produção. A trava de verdade é estrutural e está em outro arquivo: a
		// única coisa que dá alvo a um pet é commandSummons, e ela só é alcançada
		// nos ramos !IsPlayer de combat.go e affect_tick.go. Um pet não tem outra
		// fonte de EnemyList.
		{
			nome: "npc de cidade",
			alvo: alvoDeTeste(world.MaxUser+2, func(e *world.Entity) {
				e.NonCombatNPC = true
				e.X, e.Y = 6, 5
			}),
			quer: false, porqu: "lojista, banqueiro e dador de quest não podem apanhar",
		},
		{
			nome: "outro pet",
			alvo: alvoDeTeste(world.MaxUser+3, func(e *world.Entity) {
				e.Summoner = 4
				e.Clan = summonClan
				e.X, e.Y = 6, 5
			}),
			quer: false, porqu: "pets não brigam entre si",
		},
		{
			nome: "monstro comum",
			alvo: alvoDeTeste(world.MaxUser+4, func(e *world.Entity) {
				e.Clan = 1
				e.X, e.Y = 6, 5
			}),
			quer: true, porqu: "é para isso que o bicho é evocado",
		},
	}
	for _, c := range casos {
		if got := validTarget(w, pet, c.alvo); got != c.quer {
			t.Errorf("pet contra %s: validTarget = %v, want %v — %s", c.nome, got, c.quer, c.porqu)
		}
	}
}

// TestMonstroPodeRevidarNoPet é a contrapartida: o pet não é intocável.
//
// Antes da correção ele era — o mesmo Merchant que o tirava da IA também o
// protegia de apanhar, e um bicho invulnerável parado no meio da briga é um
// escudo de graça.
func TestMonstroPodeRevidarNoPet(t *testing.T) {
	log := slog.New(slog.DiscardHandler)
	d := New(Config{Log: log})
	w := world.New(world.Config{GridDim: 16}, log, nil, d.Handle)

	monstro := alvoDeTeste(world.MaxUser+1, func(e *world.Entity) {
		e.Clan = 1
		e.SegmentX, e.SegmentY = 5, 5
		e.X, e.Y = 5, 5
	})
	pet := alvoDeTeste(world.MaxUser+2, func(e *world.Entity) {
		e.Summoner = 3
		e.Clan = summonClan
		e.X, e.Y = 6, 5
	})
	if !validTarget(w, monstro, pet) {
		t.Error("o monstro não pode revidar no pet — o pet virou escudo invulnerável")
	}
}

// temInimigo diz se target já está na lista de inimigos de e.
func temInimigo(e *world.Entity, targetID int) bool {
	for _, id := range e.EnemyList {
		if id == targetID {
			return true
		}
	}
	return false
}

// TestPetOcupadoAindaRecebeOAtacanteDoDono é o buraco que deixou a queixa passar.
//
// Um pet OCIOSO já defendia — TestSummonAssistsAgainstMob provava isso —, e
// nenhum teste tinha um pet ocupado. Em jogo é raro estarem ociosos:
// assim que a evocação voltou a funcionar, todos passaram a ter alvo. Aí a
// guarda `pet.Target != 0` fazia o atacante do dono nunca entrar na lista de
// inimigos, e ninguém se virava. O legado chama SetBattle em todo membro vivo da
// party, faça ele o que estiver fazendo (Server.cpp:9964-9985); a escolha de
// quem bater fica com a seleção de alvo, que pega o mais perto.
//
// O "dono" aqui é um mob e não um jogador porque commandSummons não distingue os
// dois — ela só lê Leader/PartyList —, e o mundo não deixa um teste unitário
// fabricar entidade de jogador.
func TestPetOcupadoAindaRecebeOAtacanteDoDono(t *testing.T) {
	log := slog.New(slog.DiscardHandler)
	d := New(Config{Log: log})
	w := world.New(world.Config{GridDim: 32}, log, nil, d.Handle)

	novo := func(nome string, x, y int16) (int, *world.Entity) {
		id := w.SpawnMobAt(world.MobSpawn{Template: plainMobTemplate(nome), X: x, Y: y, GenIndex: -1})
		if id < 0 {
			t.Fatalf("não consegui criar %s", nome)
		}
		return id, w.Entity(id)
	}

	donoID, dono := novo("Dono", 10, 10)
	petID, pet := novo("Tigre", 11, 10)
	atacanteID, atacante := novo("Rato", 12, 10)

	pet.Clan = summonClan
	pet.Summoner = donoID
	pet.Leader = donoID
	pet.Target = atacanteID + 1 // ocupado com outra coisa
	dono.PartyList[0] = petID
	atacante.Clan = 5

	d.commandSummons(w, donoID, atacante)

	if !temInimigo(pet, atacanteID) {
		t.Error("o pet ocupado não registrou quem bateu no dono — ele nunca vai se virar")
	}
}

// TestSummonExpiradoSaiDoPainelDeGrupo cobre o que TestSummonExpires não vê.
//
// Aquele teste espera o MsgRemoveMob e dá por encerrado. Mas o painel de grupo
// do cliente é alimentado por outro pacote, e a linha do pet só sai com um
// MsgRemoveParty — sem ele o bicho some do chão e continua ocupando slot na
// lista. Doze linhas mortas depois, o jogador vê "não expiram" e "acumulam", e
// nenhum teste acusa nada: o que se lê ali é o mundo, e a mentira estava só na
// tela.
func TestSummonExpiradoSaiDoPainelDeGrupo(t *testing.T) {
	addr, stop, _ := startServerSummon(t, summonDB(30), nil, 0, 0)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	skillAttackFrame(t, c, serverTime, 1, 56, damSkill)
	pets := collectPets(t, c, 500*time.Millisecond)
	if len(pets) != 1 {
		t.Fatalf("pets = %d, want 1", len(pets))
	}
	var petID int
	for id := range pets {
		petID = id
	}

	saiuDoChao, saiuDoGrupo := false, false
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) && (!saiuDoChao || (petsNoPainelDeGrupo && !saiuDoGrupo)) {
		h, payload, ok := readMaybeHeaderRaw(t, c)
		if !ok {
			continue
		}
		switch h.Type {
		case protocol.MsgRemoveMob:
			if int(h.ID) == petID {
				saiuDoChao = true
			}
		case protocol.MsgRemoveParty:
			if len(payload) >= 2 && int(int16(binary.LittleEndian.Uint16(payload[0:2]))) == petID {
				saiuDoGrupo = true
			}
		}
	}
	if !saiuDoChao {
		t.Error("o pet não expirou (nenhum RemoveMob)")
	}
	if petsNoPainelDeGrupo && !saiuDoGrupo {
		t.Error("o pet sumiu do chão mas continuou no painel de grupo (nenhum RemoveParty) — é assim que a lista enche de linha morta")
	}
}

// TestEvocacoesBatemOsAlvosPorUnidade trava o que foi decidido, e não como foi
// decidido.
//
// Os multiplicadores em summonBonus são meio, não fim: foram resolvidos
// para estes números. Testar o multiplicador seria repetir a conta que o código
// faz; testar o RESULTADO é o que percebe se alguém mexeu numa base, num
// multiplicador ou na fórmula e mudou o bicho sem querer.
//
// A tolerância existe porque o multiplicador é inteiro e a divisão por 100
// trunca: o alvo não fecha exato, fecha em cima.
func TestEvocacoesBatemOsAlvosPorUnidade(t *testing.T) {
	const evocacao = 320 // onde os alvos foram medidos
	// O Int do dono não pode mais mudar nada: a parte do Int está zerada.
	// 3.366 é o Int de um Mortal 399 com tudo em Int, o pior caso.
	for _, ownerInt := range []int32{0, 300, 3366} {
		for _, c := range []struct {
			nome             string
			summonID         int
			baseDano, baseAC int32
			baseHP           int32
			alvoDano, alvoAC int32
			alvoHP           int32
		}{
			{"Condor", 0, 35, 15, 60, 2750, 600, 20000},
			{"Javali", 1, 35, 20, 100, 2550, 1800, 12500},
			{"Lobo", 2, 70, 40, 100, 3250, 1050, 17250},
			{"Urso", 3, 70, 60, 100, 2600, 2100, 10700},
			{"Tigre", 4, 75, 30, 100, 3750, 1200, 16300},
			{"Gorila", 5, 50, 45, 200, 3450, 1500, 14400},
			{"Dragão", 6, 100, 80, 350, 4250, 1800, 12500},
			{"Succubus", 7, 150, 110, 240, 6250, 1500, 14400},
		} {
			b := summonBonus[c.summonID]
			got := []struct {
				stat        string
				valor, alvo int32
			}{
				{"dano", c.baseDano + ownerInt*b.damInt/100 + evocacao*b.damEvo/100, c.alvoDano},
				{"AC", c.baseAC + ownerInt*b.acInt/100 + evocacao*b.acEvo/100, c.alvoAC},
				{"HP", c.baseHP + ownerInt*b.hpInt/100 + evocacao*b.hpEvo/100, c.alvoHP},
			}
			for _, g := range got {
				if diff := g.valor - g.alvo; diff > 3 || diff < -3 {
					t.Errorf("%s %s com Int %d = %d, alvo %d (diferença %d)",
						c.nome, g.stat, ownerInt, g.valor, g.alvo, diff)
				}
			}
		}
	}
}

// TestIntDoDonoNaoMexeNasEvocacoes é a metade da decisão que o teste acima só
// cobre de lado: com a parte do Int zerada, dois BMs com a mesma Evocação têm
// bichos idênticos, seja o dono all-Int ou com 500 de Constituição.
//
// Era o contrário no legado — o Int carregava de 60% a 78% do dano —, e é por
// isso que nenhum alvo de balanceamento era atingível antes.
func TestIntDoDonoNaoMexeNasEvocacoes(t *testing.T) {
	for i, b := range summonBonus {
		if b.damInt != 0 || b.acInt != 0 || b.hpInt != 0 {
			t.Errorf("summonBonus[%d] ainda escala pelo Int do dono: dano %d, AC %d, HP %d",
				i, b.damInt, b.acInt, b.hpInt)
		}
	}
}

// TestEvocacaoSobreviveAoRefreshScore é o bug que fazia a Succubus bater 1.
//
// refreshScore reconstrói o score do mob como BaseScore + equipamento, e roda em
// mob também — qualquer afeto que caia no bicho dispara. Com o bônus da evocação
// gravado só no score ATUAL, o primeiro afeto apagava tudo e a criatura voltava
// ao número do arquivo: 150 de dano numa Succubus. Contra um chefe de AC 4.500 a
// conta `dano − AC/2` cai abaixo de zero e o golpe vira o piso de 1.
//
// O teste chama refreshScore de propósito, que é o que nenhum teste fazia: todos
// mediam o pet recém-nascido, no único instante em que o número ainda estava lá.
func TestEvocacaoSobreviveAoRefreshScore(t *testing.T) {
	log := slog.New(slog.DiscardHandler)
	d := New(Config{
		Log:        log,
		Spells:     evokeSpell(),
		SummonMobs: [][]byte{summonTemplate("Condor")},
	})
	w := world.New(world.Config{GridDim: 32}, log, nil, d.Handle)

	// generateSummon guarda os pets na PartyList do LÍDER, e resolve o líder pelo
	// mundo. Um teste unitário não consegue fabricar entidade de jogador, então o
	// líder aqui é um mob de verdade — a função não distingue os dois, ela só lê
	// Leader e PartyList.
	liderID := w.SpawnMobAt(world.MobSpawn{Template: plainMobTemplate("Lider"), X: 5, Y: 5, GenIndex: -1})
	if liderID < 0 {
		t.Fatal("não consegui criar o líder")
	}
	dono := &world.Entity{
		ID: 0, Mode: world.MobUser, Name: "Beast", X: 5, Y: 5,
		HP: 1000, MaxHP: 1000, Level: 50, Int: 100, Leader: liderID,
		BaseSpecial: [4]int16{0, 0, 320, 0}, Special: [4]int16{0, 0, 320, 0},
	}
	s := &world.Session{Conn: 0, Mode: world.UserPlay}
	if !d.generateSummon(w, s, dono, 0, 1) {
		t.Fatal("não evocou nada")
	}

	var pet *world.Entity
	for _, m := range w.Entity(liderID).PartyList {
		if m >= world.MaxUser {
			pet = w.Entity(m)
		}
	}
	if pet == nil {
		t.Fatal("o pet não entrou na PartyList")
	}

	danoAoNascer, acAoNascer, hpAoNascer := pet.Damage, pet.AC, pet.MaxHP
	if danoAoNascer <= summonBonus[0].damEvo {
		t.Fatalf("o bônus nem chegou a ser aplicado: dano %d", danoAoNascer)
	}

	// O que acontece no jogo assim que qualquer afeto encosta no bicho.
	d.refreshScore(pet)

	if pet.Damage != danoAoNascer {
		t.Errorf("dano caiu de %d para %d depois do refreshScore — o bônus estava só no score atual",
			danoAoNascer, pet.Damage)
	}
	if pet.AC != acAoNascer {
		t.Errorf("AC caiu de %d para %d depois do refreshScore", acAoNascer, pet.AC)
	}
	if pet.MaxHP != hpAoNascer {
		t.Errorf("HP máximo caiu de %d para %d depois do refreshScore", hpAoNascer, pet.MaxHP)
	}
}

// TestTrocarDeCriaturaDispensaOBandoAnterior: com gorilas em campo, lançar
// tigre traz tigres — não um clique morto.
//
// O legado recusa (Server.cpp:2991-2997) e recusa CALADO, sem mensagem: o BM
// gasta o gesto e não entende por que nada aconteceu. Como re-invocar a mesma
// criatura já refaz o bando, a troca era a única porta que continuava fechada.
func TestTrocarDeCriaturaDispensaOBandoAnterior(t *testing.T) {
	log := slog.New(slog.DiscardHandler)
	// Duas criaturas com faces distintas, que é o que a função compara.
	condor := summonTemplate("Condor")
	tigre := summonTemplate("Tigre")
	// Equip[0] fica no deslocamento 140 (MobEquip): é a "face", e é por ela que
	// generateSummon distingue uma linhagem da outra.
	binary.LittleEndian.PutUint16(tigre[140:], 244)
	d := New(Config{Log: log, SummonMobs: [][]byte{condor, tigre}})
	w := world.New(world.Config{GridDim: 32}, log, nil, d.Handle)

	liderID := w.SpawnMobAt(world.MobSpawn{Template: plainMobTemplate("Lider"), X: 5, Y: 5, GenIndex: -1})
	if liderID < 0 {
		t.Fatal("não consegui criar o líder")
	}
	dono := &world.Entity{
		ID: 0, Mode: world.MobUser, X: 5, Y: 5, HP: 1000, MaxHP: 1000, Level: 50, Int: 100,
		Leader: liderID, BaseSpecial: [4]int16{0, 0, 320, 0}, Special: [4]int16{0, 0, 320, 0},
	}
	s := &world.Session{Conn: 0, Mode: world.UserPlay}
	lider := w.Entity(liderID)

	if !d.generateSummon(w, s, dono, 0, 2) {
		t.Fatal("a primeira evocação não saiu")
	}
	primeiros := map[int]bool{}
	for _, m := range lider.PartyList {
		if m >= world.MaxUser {
			primeiros[m] = true
		}
	}
	if len(primeiros) == 0 {
		t.Fatal("nenhum pet na primeira evocação")
	}

	// Agora a OUTRA criatura, com a primeira ainda em campo.
	if !d.generateSummon(w, s, dono, 1, 2) {
		t.Fatal("trocar de criatura foi recusado — é o clique morto que o jogador vê")
	}
	for _, m := range lider.PartyList {
		if m < world.MaxUser {
			continue
		}
		if primeiros[m] {
			t.Errorf("o pet %d da criatura anterior sobreviveu à troca", m)
		}
	}
}
