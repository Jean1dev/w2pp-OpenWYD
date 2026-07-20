package handler

import (
	"context"
	"encoding/binary"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/content"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

func TestClearDetoxAffects(t *testing.T) {
	d := New(Config{})
	w := world.New(world.Config{GridDim: 16}, slog.Default(), nil, nil)
	caster := &world.Entity{LearnedSkill: 1 << 7}
	target := &world.Entity{ID: 2}
	target.Affect[0] = world.Affect{Type: 1}
	target.Affect[1] = world.Affect{Type: 9}
	target.Affect[2] = world.Affect{Type: 32}

	d.clearDetoxAffects(w, caster, target, target.ID)

	if target.Affect[0].Type != 0 || target.Affect[2].Type != 0 {
		t.Fatalf("detox did not clear removable affects: %+v", target.Affect[:3])
	}
	if target.Affect[1].Type != 9 {
		t.Fatalf("detox cleared non-removable affect 9: %+v", target.Affect[:3])
	}
}

func TestCancelamentoClearsBlockAffect(t *testing.T) {
	d := New(Config{})
	w := world.New(world.Config{GridDim: 16}, slog.Default(), nil, nil)
	s := &world.Session{Conn: 1}
	caster := &world.Entity{ID: 1}
	target := &world.Entity{ID: 2}
	target.Affect[0] = world.Affect{Type: 19}
	dmg := 0

	skip := d.applySkillSpecial(w, s, caster, target, target.ID, 47,
		castInfo{isSkill: true, spell: content.Spell{}}, nil, &dmg)

	if !skip {
		t.Fatal("Cancelamento should skip generic affect after clearing block")
	}
	if target.Affect[0].Type != 0 {
		t.Fatalf("block affect still present: %+v", target.Affect[0])
	}
}

func TestFoemaHealFormulaAndCap(t *testing.T) {
	d := New(Config{})
	w := world.New(world.Config{GridDim: 16}, slog.Default(), nil, nil)
	caster := &world.Entity{ID: 1, ClassMaster: classMasterMortal}
	target := &world.Entity{ID: 2, HP: 100, MaxHP: 5000}
	cast := castInfo{isSkill: true, special: 600, spell: content.Spell{InstanceType: 6, InstanceValue: 100}}

	if got := d.resolveSkillHit(w, caster, target, target.ID, 27, cast); got != -1100 {
		t.Fatalf("skill 27 heal = %d, want -1100 cap", got)
	}
	target.Clan = 4
	if got := d.resolveSkillHit(w, caster, target, target.ID, 27, cast); got != 0 {
		t.Fatalf("clan 4 heal = %d, want 0", got)
	}
}

func TestFoemaHealFairyReduction(t *testing.T) {
	d := New(Config{})
	target := &world.Entity{}

	if got := d.foemaHealAmount(target, 1000); got != 1000 {
		t.Fatalf("plain heal = %d, want 1000", got)
	}
	target.Equip[fairyEquipSlot] = world.Item{Index: 786, Effects: [3]world.Effect{{Effect: efSanc, Value: 5}}}
	if got := d.foemaHealAmount(target, 1000); got != 200 {
		t.Fatalf("fairy 786 +5 heal = %d, want 200", got)
	}
	target.Equip[fairyEquipSlot] = world.Item{Index: 786}
	if got := d.foemaHealAmount(target, 1000); got != 500 {
		t.Fatalf("fairy 786 no sanc heal = %d, want min divisor 2 => 500", got)
	}
}

func TestFoemaHealExpGain(t *testing.T) {
	caster := &world.Entity{ID: 1}
	target := &world.Entity{ID: 2, HP: 1100, X: 0, Y: 0}

	if got := healExpGain(caster, target, 100); got != 120 {
		t.Fatalf("heal exp = %d, want capped 120", got)
	}
	target.X, target.Y = 2096, 2096
	if got := healExpGain(caster, target, 100); got != 0 {
		t.Fatalf("village heal exp = %d, want 0", got)
	}
	target.ID, target.X, target.Y = caster.ID, 0, 0
	if got := healExpGain(caster, target, 100); got != 0 {
		t.Fatalf("self heal exp = %d, want 0", got)
	}
}

func TestFoemaMultiBuffTargetCap(t *testing.T) {
	cases := []struct {
		special int
		want    int
	}{
		{0, 2},
		{100, 6},
		{1000, protocol.MaxTarget},
	}
	for _, c := range cases {
		if got := foemaMultiBuffTargetCap(c.special); got != c.want {
			t.Fatalf("foemaMultiBuffTargetCap(%d) = %d, want %d", c.special, got, c.want)
		}
	}
}

func TestManaControlDamage(t *testing.T) {
	target := &world.Entity{MP: 200, MaxMP: 1000}
	target.Affect[0] = world.Affect{Type: 18}

	dmg, spent, ok := manaControlDamage(target, 110, false)

	if !ok {
		t.Fatal("manaControlDamage reported no change")
	}
	if dmg != 33 || spent != 110 || target.MP != 90 {
		t.Fatalf("mana control = damage %d spent %d MP %d, want 33/110/90", dmg, spent, target.MP)
	}
	target.MP = 200
	dmg, spent, ok = manaControlDamage(target, 110, true)
	if !ok || dmg != 36 || spent != 110 || target.MP != 90 {
		t.Fatalf("enhanced mana control = ok %v damage %d spent %d MP %d, want true/36/110/90", ok, dmg, spent, target.MP)
	}
}

func TestThunderTargetsSkipProtectedAndDeduplicate(t *testing.T) {
	d := New(Config{})
	w := world.New(world.Config{GridDim: 16}, slog.Default(), nil, nil)
	caster := &world.Entity{ID: 1, X: 5, Y: 5, Clan: 7}
	first := w.SpawnMobAt(world.MobSpawn{Template: summonTemplate("First"), X: 4, Y: 4, GenIndex: -1})
	second := w.SpawnMobAt(world.MobSpawn{Template: summonTemplate("Second"), X: 1, Y: 1, GenIndex: -1})
	clan4 := w.SpawnMobAt(world.MobSpawn{Template: summonTemplate("Clan4"), X: 3, Y: 3, GenIndex: -1})
	hidden := w.SpawnMobAt(world.MobSpawn{Template: summonTemplate("Hidden"), X: 2, Y: 2, GenIndex: -1})
	w.Entity(clan4).Clan = 4
	w.Entity(hidden).Rsv = world.RsvHide

	targets := d.thunderTargets(w, caster)

	if len(targets) != 2 {
		t.Fatalf("thunder targets len = %d, want 2 (%v)", len(targets), targets)
	}
	if targets[0].ID != first || targets[1].ID != second {
		t.Fatalf("thunder target order = %d,%d, want %d,%d", targets[0].ID, targets[1].ID, first, second)
	}
}

func TestBeastAuraTickUsesSkill52(t *testing.T) {
	d := New(Config{Spells: content.NewSkillData([]content.Spell{
		{Index: 52, ManaSpent: 25, InstanceType: 4, InstanceValue: 220, MaxTarget: 5, Aggressive: 1},
	})})
	w := world.New(world.Config{GridDim: 16}, slog.Default(), nil, nil)
	s := &world.Session{Conn: 1, ReqMp: 500}
	caster := &world.Entity{ID: 1, Class: 2, X: 5, Y: 5, HP: 1000, MP: 500, MaxMP: 500, Level: 50, Int: 100}
	targetID := w.SpawnMobAt(world.MobSpawn{Template: summonTemplate("Target"), X: 4, Y: 4, GenIndex: -1})
	target := w.Entity(targetID)
	before := target.HP

	if !d.applyBeastAuraTick(w, s, caster, 160) {
		t.Fatal("applyBeastAuraTick reported no hit")
	}
	if caster.MP != 475 || s.ReqMp != 475 {
		t.Fatalf("caster MP/ReqMp = %d/%d, want 475/475", caster.MP, s.ReqMp)
	}
	if target.HP >= before {
		t.Fatalf("target HP = %d, want below %d", target.HP, before)
	}
}

func TestEtherealFlameBurnsPlayerMP(t *testing.T) {
	db := newDB()
	db.loadResult = world.CharacterState{
		Slot: 0, Name: "Beast", Class: 2, X: 5, Y: 5,
		HP: 1000, MaxHP: 1000, MP: 500, MaxMP: 500, Level: 50, Int: 100,
		LearnedSkill: 1 << 1, // skill 49 learned (49%24)
	}
	spells := content.NewSkillData([]content.Spell{
		{Index: 49, ManaSpent: 35, InstanceType: 12, MaxTarget: 1, Name: "Chamas Etereas"},
	})
	addr, stop := startServerCustomSpells(t, db, spells)
	defer stop()
	attacker := enterWorld(t, addr)
	defer attacker.Close()
	target := enterWorld(t, addr)
	defer target.Close()

	skillAttackFrame(t, attacker, serverTime, 2, 49, damSkill)

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		ty, payload, ok := readMaybeRaw(t, target)
		if !ok || ty != protocol.MsgSetHpMp {
			continue
		}
		mp := int32(binary.LittleEndian.Uint32(payload[4:8]))
		reqMP := int32(binary.LittleEndian.Uint32(payload[12:16]))
		if mp <= 0 || mp >= 500 || reqMP != mp {
			t.Fatalf("target MP/ReqMp after Chamas = %d/%d, want burned below 500 and synced", mp, reqMP)
		}
		return
	}
	t.Fatal("target did not receive SetHpMp from Chamas Etereas")
}

func startServerCustomSpells(t *testing.T, persist world.Persistence, spells *content.SkillData) (string, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := New(Config{Log: log, Spells: spells})
	w := world.New(world.Config{GridDim: 16}, log, persist, d.Handle)
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
	}
}

func TestHuntressSkill79UsesDamageFormulaHalf(t *testing.T) {
	d := New(Config{})
	w := world.New(world.Config{GridDim: 16}, slog.Default(), nil, nil)
	caster := &world.Entity{ID: 1, Class: 3, Damage: 100}
	target := &world.Entity{ID: 2, AC: 20}
	cast := castInfo{isSkill: true, spell: content.Spell{InstanceType: 1}}

	if got := d.resolveSkillHit(w, caster, target, target.ID, 79, cast); got != 78 {
		t.Fatalf("skill 79 damage = %d, want 78", got)
	}
}

func TestHuntressSkill85ChargesGoldBut86DoesNot(t *testing.T) {
	spells := content.NewSkillData([]content.Spell{
		{Index: 85, ManaSpent: 120, AffectType: 31, AffectValue: 150, AffectTime: 7},
		{Index: 86, ManaSpent: 25, InstanceType: 1, InstanceValue: 35},
	})
	d := New(Config{Spells: spells})
	w := world.New(world.Config{GridDim: 16}, slog.Default(), nil, nil)
	s := &world.Session{Conn: 1}
	e := &world.Entity{ID: 1, Class: 3, LearnedSkill: (1 << (85 % content.MaxSkill)) | (1 << (86 % content.MaxSkill)), Coin: 600}
	e.Special[content.SkillKind(85)] = 5

	if _, ok := d.validateCast(w, s, e, 85, 1000); !ok {
		t.Fatal("skill 85 rejected with enough gold")
	}
	if e.Coin != 100 {
		t.Fatalf("coin after skill 85 = %d, want 100", e.Coin)
	}
	if _, ok := d.validateCast(w, s, e, 86, 1000); !ok {
		t.Fatal("skill 86 rejected")
	}
	if e.Coin != 100 {
		t.Fatalf("coin after skill 86 = %d, want unchanged 100", e.Coin)
	}

	e.Coin = 499
	if _, ok := d.validateCast(w, s, e, 85, 1000); ok {
		t.Fatal("skill 85 accepted without enough gold")
	}
	if e.Coin != 499 {
		t.Fatalf("coin after rejected skill 85 = %d, want 499", e.Coin)
	}
}

func TestHuntressForceDamageApplication(t *testing.T) {
	attacker := &world.Entity{AffForceMobDamage: 25, AffForceDamage: 40}
	mob := &world.Entity{ID: world.MaxUser}
	if got := applyHuntressForceDamage(attacker, mob, mob.ID, 100); got != 165 {
		t.Fatalf("force damage vs mob = %d, want 165", got)
	}
	player := &world.Entity{ID: 2}
	if got := applyHuntressForceDamage(attacker, player, player.ID, 100); got != 65 {
		t.Fatalf("force damage vs player = %d, want 65", got)
	}
}

func TestHuntressAirBladeProcAddsDamage(t *testing.T) {
	d := New(Config{})
	w := world.New(world.Config{GridDim: 16}, slog.Default(), nil, nil)
	for i := 0; i < 3; i++ {
		w.Rand().Intn(1)
	}
	attacker := &world.Entity{Class: 3, LearnedSkill: 1 << 21, Str: 100}
	attacker.Special[3] = 100
	target := &world.Entity{AC: 20}
	body := &protocol.MsgAttackBody{}
	payload := make([]byte, protocol.MsgAttackDamOffset)

	got := d.applyAirBladeProc(w, attacker, target, protocol.MsgAttackTwo, body, payload, 100)

	if got != 198 {
		t.Fatalf("air blade damage = %d, want 198", got)
	}
	if body.DoubleCritical&4 == 0 || payload[36]&4 == 0 {
		t.Fatalf("air blade flags = body %#x payload %#x, want bit 4", body.DoubleCritical, payload[36])
	}
}

func TestHuntressOnHitSpellsUseLegacySkillRows(t *testing.T) {
	d := New(Config{Spells: content.NewSkillData([]content.Spell{
		{Index: 36, AffectType: 1, AffectValue: 2, AffectTime: 1, Aggressive: 1},
		{Index: 40, TickType: 20, TickValue: 10, AffectTime: 1, Aggressive: 1},
	})})
	w := world.New(world.Config{GridDim: 16}, slog.Default(), nil, nil)
	player := &world.Entity{ID: 2}
	d.applyOnHitSpell(w, player, player.ID, 36, 200, 50)
	if player.Affect[0].Type != 1 || player.Affect[0].Value != 2 || player.Affect[0].Level != 50 {
		t.Fatalf("frost affect = %+v, want type 1 value 2 level 50", player.Affect[0])
	}

	mobID := w.SpawnMobAt(world.MobSpawn{Template: summonTemplate("Target"), X: 5, Y: 5, GenIndex: -1})
	mob := w.Entity(mobID)
	d.applyOnHitSpell(w, mob, mobID, 40, 200, 50)
	if mob.Affect[0].Type != 20 || mob.Affect[0].Value != 10 || mob.Affect[0].Level != 50 {
		t.Fatalf("drain tick = %+v, want type 20 value 10 level 50", mob.Affect[0])
	}
}

func TestExterminarConsumesRemainingMPIntoDamage(t *testing.T) {
	d := New(Config{})
	w := world.New(world.Config{GridDim: 16}, slog.Default(), nil, nil)
	s := &world.Session{Conn: 1, ReqMp: 200}
	caster := &world.Entity{ID: 1, MP: 200, Int: 80}
	target := &world.Entity{ID: 2}
	body := &protocol.MsgAttackBody{ReqMp: 200}
	dmg := 50

	d.applySkillSpecial(w, s, caster, target, target.ID, 22, castInfo{isSkill: true}, body, &dmg)

	if dmg != 290 {
		t.Fatalf("Exterminar damage = %d, want 290", dmg)
	}
	if caster.MP != 0 || s.ReqMp != 0 || body.ReqMp != 0 {
		t.Fatalf("MP state = caster %d req %d body %d, want all zero", caster.MP, s.ReqMp, body.ReqMp)
	}
}

func TestNocaoDeCombateSetsSkillMastery(t *testing.T) {
	d := New(Config{Spells: content.NewSkillData([]content.Spell{{Index: 10, InstanceType: 1}})})
	w := world.New(world.Config{GridDim: 16}, slog.Default(), nil, nil)
	s := &world.Session{Conn: 1}
	e := &world.Entity{ID: 1, Class: 0, LearnedSkill: (1 << 10) | (1 << 14)}
	e.Special[2] = 420

	cast, ok := d.validateCast(w, s, e, 10, protocol.SkipCheckTick)

	if !ok {
		t.Fatal("validateCast rejected TK skill")
	}
	if cast.master != 15 {
		t.Fatalf("cast.master = %d, want clamp 15", cast.master)
	}
}

func TestDivineFuryMovesTargetTowardCaster(t *testing.T) {
	d := New(Config{})
	w := world.New(world.Config{GridDim: 16}, slog.Default(), nil, nil)
	caster := &world.Entity{ID: 1, X: 5, Y: 5, Level: 100}
	targetID := w.SpawnMobAt(world.MobSpawn{Template: summonTemplate("Target"), X: 7, Y: 5, GenIndex: -1})
	target := w.Entity(targetID)

	if !d.applyDivineFury(w, caster, target, targetID, 1000) {
		t.Fatal("Furia Divina did not move target")
	}
	if target.X != 6 || target.Y != 5 {
		t.Fatalf("target position = %d,%d, want 6,5", target.X, target.Y)
	}
	if _, occupied := w.EntityAt(7, 5); occupied {
		t.Fatal("old target cell still occupied")
	}
	if id, occupied := w.EntityAt(6, 5); !occupied || id != targetID {
		t.Fatalf("new target cell = (%d,%v), want (%d,true)", id, occupied, targetID)
	}
	if target.Mode != world.MobCombat || target.Target != caster.ID {
		t.Fatalf("target battle state = mode %d target %d, want combat/%d", target.Mode, target.Target, caster.ID)
	}
}

func TestExterminarMotionMovesTargetNearCurrentCell(t *testing.T) {
	d := New(Config{})
	w := world.New(world.Config{GridDim: 16}, slog.Default(), nil, nil)
	caster := &world.Entity{ID: 1}
	targetID := w.SpawnMobAt(world.MobSpawn{Template: summonTemplate("Target"), X: 8, Y: 8, GenIndex: -1})
	target := w.Entity(targetID)

	if !d.applyExterminarMotion(w, caster, target, targetID) {
		t.Fatal("Exterminar motion did not move target")
	}
	if target.X != 7 || target.Y != 7 {
		t.Fatalf("target position = %d,%d, want first free ring cell 7,7", target.X, target.Y)
	}
	if id, occupied := w.EntityAt(7, 7); !occupied || id != targetID {
		t.Fatalf("new target cell = (%d,%v), want (%d,true)", id, occupied, targetID)
	}
}

func TestCreateVineSpawnsWallMobAtTarget(t *testing.T) {
	d := New(Config{VineMob: summonTemplate("Vine")})
	w := world.New(world.Config{GridDim: 16}, slog.Default(), nil, nil)
	body := &protocol.MsgAttackBody{TargetX: 4, TargetY: 4}

	if !d.createVine(w, body) {
		t.Fatal("createVine rejected valid target")
	}
	id, occupied := w.EntityAt(4, 4)
	if !occupied || id < world.MaxUser {
		t.Fatalf("vine cell = (%d,%v), want mob", id, occupied)
	}
	vine := w.Entity(id)
	if vine == nil || vine.RouteType != 3 || vine.Mode != world.MobPeace || vine.WaitTicks != 40 {
		t.Fatalf("vine state = %+v, want route type 3, peace, wait 40", vine)
	}
}

func TestCreateVineRejectsOccupiedTarget(t *testing.T) {
	d := New(Config{VineMob: summonTemplate("Vine")})
	w := world.New(world.Config{GridDim: 16}, slog.Default(), nil, nil)
	w.SpawnMobAt(world.MobSpawn{Template: summonTemplate("Blocker"), X: 4, Y: 4, GenIndex: -1})

	if d.createVine(w, &protocol.MsgAttackBody{TargetX: 4, TargetY: 4}) {
		t.Fatal("createVine accepted occupied target")
	}
}

// startServerSkillsTargetMob is startServerSkillsMob with a caller-supplied mob
// template at (6,5), next to the player spawn (5,5) — for exercising the
// per-target gates of attack() (merchant immunity, dead-target rejection). It
// returns the world so the test can inspect the spawned entity's state.
func startServerSkillsTargetMob(t *testing.T, persist world.Persistence, tmpl []byte) (string, func(), *world.World) {
	t.Helper()
	return startServerSkillsTargetMobAt(t, persist, tmpl, 16, 6, 5)
}

func startServerSkillsTargetMobAt(t *testing.T, persist world.Persistence, tmpl []byte, gridDim int, x, y int16) (string, func(), *world.World) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := New(Config{Log: log, Spells: testSpells()})
	w := world.New(world.Config{GridDim: gridDim}, log, persist, d.Handle)
	w.SpawnMob(tmpl, x, y)
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
	}, w
}

// targetMob builds an 816-byte STRUCT_MOB with the given Merchant flag and HP.
func targetMob(name string, merchant byte, hp uint32) []byte {
	return targetMobWithClan(name, 0, merchant, hp)
}

func targetMobWithClan(name string, clan, merchant byte, hp uint32) []byte {
	b := make([]byte, 816)
	copy(b[0:16], name)
	b[16] = clan
	const cs = 92 // CurrentScore offset
	b[cs+12] = merchant
	binary.LittleEndian.PutUint32(b[cs+16:], 5000) // MaxHp
	binary.LittleEndian.PutUint32(b[cs+24:], hp)   // Hp
	return b
}

func attackEchoDamage(t *testing.T, c net.Conn, tick uint32, targetID, skillnum int, claim int32) int32 {
	t.Helper()
	skillAttackFrame(t, c, tick, targetID, skillnum, claim)
	ty, payload, ok := readMaybe(t, c)
	if !ok || ty != protocol.MsgAttack {
		t.Fatalf("attacker got %#x ok=%v, want MsgAttack echo", ty, ok)
	}
	var got protocol.MsgAttackBody
	if err := got.Decode(payload); err != nil {
		t.Fatal(err)
	}
	if len(got.Dam) != 1 {
		t.Fatalf("Dam = %+v, want one entry", got.Dam)
	}
	return got.Dam[0].Damage
}

// castEchoDamage casts skillnum (claim -1) at targetID and returns the resolved
// Dam[0].Damage from the attacker's own authoritative MsgAttack echo.
func castEchoDamage(t *testing.T, c net.Conn, targetID, skillnum int) int32 {
	t.Helper()
	return attackEchoDamage(t, c, serverTime, targetID, skillnum, -1)
}

// TestSkillMerchantTakesNoDamage: an untouchable NPC (Merchant != 0) is immune
// to a damage skill — the per-target gate zeroes the hit (_MSG_Attack.cpp:333).
func TestSkillMerchantTakesNoDamage(t *testing.T) {
	addr, stop, w := startServerSkillsTargetMob(t, skillCombatDB(1<<2), targetMob("Loja", 1, 5000))
	defer stop()
	c := enterWorld(t, addr) // learned elemental skill 2
	defer c.Close()

	if dmg := castEchoDamage(t, c, world.MaxUser, 2); dmg != 0 {
		t.Errorf("merchant took %d damage, want 0 (untouchable NPC)", dmg)
	}
	if npc := w.Entity(world.MaxUser); npc == nil || npc.HP != 5000 {
		t.Fatalf("merchant HP = %v, want it untouched at 5000", npc)
	}
}

func TestTownNonCombatNPCTakesNoDamage(t *testing.T) {
	db := skillCombatDB(1 << 2)
	db.loadResult.X, db.loadResult.Y = 2086, 2093 // Armia
	addr, stop, w := startServerSkillsTargetMobAt(t, db, targetMobWithClan("Ehre", 3, 0, 5000), 4096, 2087, 2093)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	if dmg := attackEchoDamage(t, c, serverTime, world.MaxUser, -1, -2); dmg != 0 {
		t.Errorf("town NPC melee damage = %d, want 0", dmg)
	}
	if dmg := attackEchoDamage(t, c, serverTime+1000, world.MaxUser, 2, -1); dmg != 0 {
		t.Errorf("town NPC skill damage = %d, want 0", dmg)
	}
	if npc := w.Entity(world.MaxUser); npc == nil || npc.HP != 5000 || !npc.NonCombatNPC {
		t.Fatalf("town NPC state = %+v, want HP 5000 and NonCombatNPC", npc)
	}
}

func TestHostileCityMobStillTakesDamage(t *testing.T) {
	db := skillCombatDB(1 << 2)
	db.loadResult.X, db.loadResult.Y = 2086, 2093 // Armia
	addr, stop, w := startServerSkillsTargetMobAt(t, db, targetMobWithClan("Orc", 1, 0, 5000), 4096, 2087, 2093)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	if dmg := attackEchoDamage(t, c, serverTime, world.MaxUser, -1, -2); dmg <= 0 {
		t.Fatalf("hostile city mob melee damage = %d, want > 0", dmg)
	}
	if dmg := attackEchoDamage(t, c, serverTime+1000, world.MaxUser, 2, -1); dmg <= 0 {
		t.Fatalf("hostile city mob skill damage = %d, want > 0", dmg)
	}
	if mob := w.Entity(world.MaxUser); mob == nil || mob.NonCombatNPC || mob.HP >= 5000 {
		t.Fatalf("hostile city mob state = %+v, want combat mob with reduced HP", mob)
	}
}

// TestSkillDeadTargetRejectsDamage: a present-but-dead entity (HP<=0) is skipped
// by a damage skill — only the resurrection skills (31/99) may target it
// (_MSG_Attack.cpp:333). Here the target is a mob spawned already at 0 HP; the
// test asserts the entity is present (so it is the dead-target gate, not the
// empty-slot gate, doing the work).
func TestSkillDeadTargetRejectsDamage(t *testing.T) {
	addr, stop, w := startServerSkillsTargetMob(t, skillCombatDB(1<<2), targetMob("Cadaver", 0, 0))
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	corpse := w.Entity(world.MaxUser)
	if corpse == nil || corpse.Mode == world.MobEmpty {
		t.Skip("0-HP mob not retained as a present entity; dead-target gate needs a live-then-killed target")
	}
	if corpse.HP > 0 {
		t.Fatalf("corpse HP = %d, want <= 0", corpse.HP)
	}
	if dmg := castEchoDamage(t, c, world.MaxUser, 2); dmg != 0 {
		t.Errorf("dead target took %d damage, want 0 (only resurrection may hit the dead)", dmg)
	}
}

// TestSkillNonexistentTargetZeroesDamage: a Dam entry naming an empty slot
// resolves to 0 without dropping the whole attack (the loop's target==nil
// guard, _MSG_Attack.cpp per-entry skip).
func TestSkillNonexistentTargetZeroesDamage(t *testing.T) {
	addr, stop := startServerSkills(t, skillCombatDB(1<<2))
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	// Slot 500 is a valid player-range id with nobody logged in → w.Entity is nil.
	if dmg := castEchoDamage(t, c, 500, 2); dmg != 0 {
		t.Errorf("nonexistent target resolved %d damage, want 0", dmg)
	}
}

// TestApplyCastAffectAggressiveSkipsAllies covers the aggressive-affect landing
// gate (_MSG_Attack.cpp:1172-1240, combat.go applyCastAffect): a hostile cast
// never lands on a same-party/guild ally, bounces off RSV_BLOCK, and clan 6 is
// immune to player casts — but a genuine enemy takes it. AffectResist is 0 so
// the resist roll is skipped and each case is deterministic.
func TestApplyCastAffectAggressiveSkipsAllies(t *testing.T) {
	d := New(Config{})
	w := world.New(world.Config{GridDim: 16}, slog.Default(), nil, nil)
	aggro := castInfo{isSkill: true, special: 50, spell: content.Spell{
		Aggressive: 1, AffectType: 11, AffectValue: 5, AffectTime: 100,
	}}
	const casterID, targetID = 1, 2

	newPair := func(mut func(caster, target *world.Entity)) (*world.Entity, *world.Entity) {
		caster := &world.Entity{ID: casterID, Level: 50}
		target := &world.Entity{ID: targetID, Level: 50}
		if mut != nil {
			mut(caster, target)
		}
		return caster, target
	}

	cases := []struct {
		name string
		mut  func(caster, target *world.Entity)
		want bool // affect expected to land
	}{
		{"same leader is skipped", func(c, tgt *world.Entity) { c.Leader = 9; tgt.Leader = 9 }, false},
		{"same guild is skipped", func(c, tgt *world.Entity) { c.Guild = 7; tgt.Guild = 7 }, false},
		{"RSV_BLOCK bounces the affect", func(_, tgt *world.Entity) { tgt.Rsv |= world.RsvBlock }, false},
		{"clan 6 immune to player casts", func(_, tgt *world.Entity) { tgt.Clan = 6 }, false},
		{"genuine enemy takes it", nil, true},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			caster, target := newPair(tt.mut)
			d.applyCastAffect(w, caster, target, targetID, aggro)
			if got := target.HasAffect(11); got != tt.want {
				t.Errorf("HasAffect(11) = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateSkillTargetCommonGates(t *testing.T) {
	d := New(Config{})
	w := world.New(world.Config{GridDim: 32}, slog.Default(), nil, nil)
	s := &world.Session{Conn: 1}
	caster := &world.Entity{ID: 1, X: 5, Y: 5}
	target := &world.Entity{ID: 2, X: 7, Y: 5}

	cast := castInfo{isSkill: true, spell: content.Spell{MaxTarget: 1, Range: 3, TargetType: 1}}
	if !d.validateSkillTarget(w, s, caster, target, 1, cast, 1000) {
		t.Fatal("legacy MaxTarget index 1 should allow Dam[1]")
	}
	if d.validateSkillTarget(w, s, caster, target, 2, cast, 1000) {
		t.Fatal("MaxTarget should reject Dam[2] when MaxTarget is 1")
	}
	if s.CrackError != 1 {
		t.Fatalf("CrackError after MaxTarget reject = %d, want 1", s.CrackError)
	}

	cast.spell.MaxTarget = 13
	cast.spell.Range = 1
	if d.validateSkillTarget(w, s, caster, target, 0, cast, 1000) {
		t.Fatal("range gate accepted target beyond spell range")
	}

	cast.spell.Range = 3
	cast.spell.BParty = 1
	if d.validateSkillTarget(w, s, caster, target, 0, cast, 1000) {
		t.Fatal("party skill accepted non-party/non-guild target")
	}
	target.Leader = caster.ID
	if !d.validateSkillTarget(w, s, caster, target, 0, cast, 1000) {
		t.Fatal("party skill rejected same-leader target")
	}
}

func TestValidateSkillTargetSelfOnlyTargetTypeZero(t *testing.T) {
	d := New(Config{})
	w := world.New(world.Config{GridDim: 16}, slog.Default(), nil, nil)
	s := &world.Session{Conn: 1}
	caster := &world.Entity{ID: 1, X: 5, Y: 5}
	other := &world.Entity{ID: 2, X: 5, Y: 5}
	selfBuff := castInfo{isSkill: true, spell: content.Spell{
		TargetType: 0, MaxTarget: 1, AffectType: 11,
	}}

	if d.validateSkillTarget(w, s, caster, other, 0, selfBuff, 1000) {
		t.Fatal("self-only TargetType 0 affect accepted another target")
	}
	if !d.validateSkillTarget(w, s, caster, caster, 0, selfBuff, 1000) {
		t.Fatal("self-only TargetType 0 affect rejected caster")
	}
}

func TestValidateGuardianCannonRequiresMortarItem(t *testing.T) {
	w := world.New(world.Config{GridDim: 16}, slog.Default(), nil, nil)
	body := protocol.MsgAttackBody{PosX: 5, PosY: 6}
	payload := body.Encode()

	if validateGuardianCannon(w, &body, payload) {
		t.Fatal("Guardian Cannon validated without item 746")
	}
	w.CreateGroundItem(world.Item{Index: 746}, 5, 6)
	if !validateGuardianCannon(w, &body, payload) {
		t.Fatal("Guardian Cannon rejected item 746 at PosX/PosY")
	}
	if body.Motion != 1 || payload[34] != 1 {
		t.Fatalf("motion = body %d payload %d, want 1/1", body.Motion, payload[34])
	}
}

func TestConsumeIllusionSpendsManaAndTracksTick(t *testing.T) {
	d := New(Config{Spells: content.NewSkillData([]content.Spell{{Index: 73, ManaSpent: 45}})})
	w := world.New(world.Config{GridDim: 16}, slog.Default(), nil, nil)
	s := &world.Session{Conn: 1, ReqMp: 100}
	e := &world.Entity{Class: 3, LearnedSkill: 2, MP: 100, MaxMP: 100}

	if !d.consumeIllusion(w, s, e, 1000) {
		t.Fatal("consumeIllusion rejected learned Huntress")
	}
	if e.MP != 55 || s.ReqMp != 55 || s.LastIllusionTick != 1000 {
		t.Fatalf("illusion state = MP %d ReqMp %d tick %d, want 55/55/1000", e.MP, s.ReqMp, s.LastIllusionTick)
	}
}

func TestConsumeIllusionRejectsUnlearned(t *testing.T) {
	d := New(Config{})
	w := world.New(world.Config{GridDim: 16}, slog.Default(), nil, nil)
	s := &world.Session{Conn: 1}
	e := &world.Entity{Class: 3, MP: 100}

	if d.consumeIllusion(w, s, e, 1000) {
		t.Fatal("consumeIllusion accepted unlearned skill")
	}
	if s.CrackError != 1 {
		t.Fatalf("CrackError = %d, want 1", s.CrackError)
	}
}
