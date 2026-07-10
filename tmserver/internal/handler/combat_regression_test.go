package handler

import (
	"context"
	"encoding/binary"
	"io"
	"log/slog"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// startServerSkillsMob is the REGRESSION harness for player→mob combat: unlike
// startServerClock it loads a skill catalog (as production does — that is what
// activates the skill-validation path in attack) and spawns one monster next to
// the player spawn. Guards the "char não tira dano do mob" regression (B12).
func startServerSkillsMob(t *testing.T, persist world.Persistence) (string, func(), *world.World) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	clock := &atomic.Uint32{}
	clock.Store(serverTime)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := New(Config{Log: log, Spells: testSpells()})
	w := world.New(world.Config{GridDim: 16, Now: clock.Load}, log, persist, d.Handle)
	w.SpawnMob(punchingBagMob(), 6, 5) // adjacent to the player spawn (5,5)
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

// punchingBagMob is an 816-byte STRUCT_MOB monster (Merchant=0) with a big HP
// pool and no AC, so any resolved hit shows as an HP drop.
func punchingBagMob() []byte {
	b := make([]byte, 816)
	copy(b[0:16], "Saco_Pancada")
	const cs = 92
	binary.LittleEndian.PutUint32(b[cs+0:], 10)     // Level
	binary.LittleEndian.PutUint32(b[cs+16:], 50000) // MaxHp
	binary.LittleEndian.PutUint32(b[cs+24:], 50000) // Hp
	return b
}

// meleeVariants are the client field combinations a melee attack may plausibly
// arrive with. The 12000 client's exact SkillIndex/Dam values for melee are
// UNVERIFIED (only the legacy's `-2 melee / -1 skill` sentinel check is in the
// partial source), and gating damage on that assumption is exactly what broke
// player→mob combat once before (B12): the server MUST resolve melee damage for
// ALL of these, because damage is server-authoritative anyway.
var meleeVariants = []struct {
	name  string
	skill int
	claim int32
}{
	{"sentinel melee (skill -1, dam -2)", -1, -2},
	{"skillindex 0 with melee sentinel", 0, -2},
	{"skillindex 0 with zero dam", 0, 0},
	{"garbage client damage", -1, 1234},
}

// TestMeleeAlwaysDamagesMob is the B12 regression: with the skill catalog
// loaded (production config), every plausible melee encoding must still take HP
// off a monster. If any variant deals 0 damage, the attack pipeline started
// trusting an UNVERIFIED client field again.
func TestMeleeAlwaysDamagesMob(t *testing.T) {
	for _, v := range meleeVariants {
		t.Run(v.name, func(t *testing.T) {
			addr, stop, w := startServerSkillsMob(t, skillCombatDB(0)) // no skills learned
			defer stop()
			c := enterWorld(t, addr)
			defer c.Close()

			skillAttackFrame(t, c, serverTime, world.MaxUser, v.skill, v.claim)

			ty, payload, ok := readMaybe(t, c)
			if !ok || ty != protocol.MsgAttack {
				t.Fatalf("attacker got %#x ok=%v, want the MsgAttack echo (attack was dropped)", ty, ok)
			}
			var got protocol.MsgAttackBody
			if err := got.Decode(payload); err != nil {
				t.Fatal(err)
			}
			if len(got.Dam) != 1 || got.Dam[0].Damage <= 0 {
				t.Fatalf("resolved damage = %+v, want > 0 (melee must always land)", got.Dam)
			}
			if got.CurrentMp != 500 {
				t.Errorf("melee spent mana: CurrentMp = %d, want 500", got.CurrentMp)
			}
			// The echoed Dam is written AFTER the loop applied it to the target's
			// HP (same code path), so Dam > 0 ⇔ the mob really lost HP.
			_ = w
		})
	}
}

// TestMeleePvPDamageAppliesInCity is the issue #67 regression: a hardcoded,
// unconditional "town safe zone" gate used to zero all melee/aggressive-skill
// damage against a player standing inside one of the 5 city rectangles
// (world.Village) — which includes every character's default spawn point
// (world.CitySpawn). None of the legacy bypass conditions (PKMode, Guilty,
// attribute-map bit, RvR/Castle/GTorre war state) are implemented, so that
// gate could never correctly unblock itself and PvP damage silently never
// landed wherever players actually gather to fight. Melee must land here
// exactly like it does in the open field.
func TestMeleePvPDamageAppliesInCity(t *testing.T) {
	db := newDB()
	db.loadResult = world.CharacterState{
		Slot: 0, Name: "Hero", X: 2086, Y: 2093, // inside Armia (city.go's cities[0])
		HP: 1000, MaxHP: 1000, Damage: 200, AC: 40,
	}
	addr, stop, _ := startServerClock(t, db)
	defer stop()

	attacker := enterWorld(t, addr) // conn 1 (attacker)
	defer attacker.Close()
	target := enterWorld(t, addr) // conn 2 (target), in view
	defer target.Close()

	attackFrame(t, attacker, serverTime, 2, 0)

	ty, payload, ok := readMaybe(t, target)
	if !ok || ty != protocol.MsgAttack {
		t.Fatalf("target got %#x ok=%v, want MsgAttack broadcast", ty, ok)
	}
	var got protocol.MsgAttackBody
	if err := got.Decode(payload); err != nil {
		t.Fatal(err)
	}
	if len(got.Dam) != 1 || got.Dam[0].TargetID != 2 {
		t.Fatalf("Dam = %+v", got.Dam)
	}
	if got.Dam[0].Damage <= 0 {
		t.Errorf("server damage in city = %d, want > 0 (PvP must not be blocked near spawn/city)", got.Dam[0].Damage)
	}
}
