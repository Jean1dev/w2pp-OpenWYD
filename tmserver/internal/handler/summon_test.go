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

func TestSummonCount(t *testing.T) {
	tests := []struct {
		iv, evo, want int
	}{
		{1, 60, 2},  // Condor: Evocação/30
		{1, 29, 0},  // below the first threshold → nothing spawns (cast refunds)
		{2, 90, 3},  // Javali: /30
		{3, 120, 3}, // Lobo: /40
		{5, 79, 1},  // Tigre: /40
		{6, 160, 2}, // Gorila: /80
		{7, 79, 0},  // Dragão Negro: /80
		{8, 0, 1},   // Succubus: always exactly one
		{9, 0, 1},   // Invocação Final value 9: one zero-bonus template
		{10, 400, 0},
	}
	for _, tt := range tests {
		if got := summonCount(tt.iv, tt.evo); got != tt.want {
			t.Errorf("summonCount(%d, %d) = %d, want %d", tt.iv, tt.evo, got, tt.want)
		}
	}
}

// summonTemplate builds an 816-byte BaseSummon-shaped STRUCT_MOB (same offsets
// as aggressiveMob): known flat stats so the owner-scaling asserts are exact.
func summonTemplate(name string) []byte {
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
		case protocol.MsgAttack:
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
		t.Fatalf("pets spawned = %d, want 2 (Evocação 60 / 30)", len(pets))
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
		if dmg != 280 {
			t.Errorf("pet damage = %d, want 280 (20 + 100·80%% + 60·300%%)", dmg)
		}
		if hp != 440 {
			t.Errorf("pet maxHP = %d, want 440 (100 + 100·100%% + 60·400%%)", hp)
		}
	}
}

func TestEvocationSendsAddPartyForSummon(t *testing.T) {
	addr, stop, _ := startServerSummon(t, summonDB(30), nil, 0, 0)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	skillAttackFrame(t, c, serverTime, 1, 56, damSkill)

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		ty, payload, ok := readMaybeRaw(t, c)
		if !ok || ty != protocol.MsgCNFAddParty {
			continue
		}
		if len(payload) != protocol.MsgCNFAddPartyBodySize {
			t.Fatalf("CNFAddParty payload = %d, want %d", len(payload), protocol.MsgCNFAddPartyBodySize)
		}
		leaderConn := binary.LittleEndian.Uint16(payload[0:2])
		partyID := binary.LittleEndian.Uint16(payload[8:10])
		name := strings.TrimRight(string(payload[10:26]), "\x00")
		if int(partyID) >= world.MaxUser {
			if leaderConn != 30000 {
				t.Fatalf("summon LeaderConn = %d, want 30000 for non-leader slot", leaderConn)
			}
			if name != "Condor^" {
				t.Fatalf("summon party name = %q, want Condor^", name)
			}
			return
		}
	}
	t.Fatal("no CNFAddParty frame for summoned pet")
}

func TestEvocationDoesNotDuplicateExistingSummon(t *testing.T) {
	addr, stop, _ := startServerSummon(t, summonDB(30), nil, 0, 0)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	skillAttackFrame(t, c, serverTime, 1, 56, damSkill)
	if pets := collectPets(t, c, 500*time.Millisecond); len(pets) != 1 {
		t.Fatalf("first cast pets = %d, want 1", len(pets))
	}

	skillAttackFrame(t, c, serverTime+1000, 1, 56, damSkill)
	if pets := collectPets(t, c, 500*time.Millisecond); len(pets) != 0 {
		t.Fatalf("second cast spawned %d more pets, want 0 because existing summon counts", len(pets))
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

// TestSummonPartySlotClearedOnDespawn: DespawnMob only sends RemoveMob, so the
// sweep has to drop the pet's party row too or the client keeps the slot.
func TestSummonPartySlotClearedOnDespawn(t *testing.T) {
	addr, stop := summonPartySrv(t, summonDB(30))
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	pet := evokeOne(t, c, 1)
	removePartyFrame(t, c, 1)
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		h, payload, ok := readMaybeHeaderRaw(t, c)
		if !ok {
			continue
		}
		if h.Type == protocol.MsgRemoveParty && int(protocolLe16(payload[0:2])) == pet {
			return
		}
	}
	t.Fatalf("no RemoveParty row for pet %d; the client keeps a ghost party slot", pet)
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
		case protocol.MsgAttack:
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
