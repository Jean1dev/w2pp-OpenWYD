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

// TestPetNaoRecebeXPDoAbate: quando o dono mata um monstro, a XP do grupo paga só
// jogadores. O legado filtra `party > 0 && party < MAX_USER` (MobKilled.cpp:444).
//
// Os pets moram na PartyList do líder, e o laço de pagamento pagava a lista
// inteira: cada pet perto do corpo ganhava XP, subia de nível e recebia o
// MsgMotion de comemoração, que o cliente anima parando o bicho. A cada monstro
// morto os pets "travavam" no meio da luta.
func TestPetNaoRecebeXPDoAbate(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	clock := &atomic.Uint32{}
	clock.Store(serverTime)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := New(Config{Log: log, Spells: evokeSpell(), SummonMobs: [][]byte{summonTemplate("Condor")}})
	w := world.New(world.Config{GridDim: 16, Now: clock.Load}, log, summonDB(60), d.Handle)

	// Um monstro de um golpe só, que vale bastante XP.
	alvo := plainMobTemplate("Ogro")
	binary.LittleEndian.PutUint64(alvo[32:], 5_000_000) // STRUCT_MOB.Exp
	binary.LittleEndian.PutUint32(alvo[92+0:], 50)      // Level: o mesmo do dono, para o abate valer XP
	binary.LittleEndian.PutUint32(alvo[92+16:], 10)     // MaxHp
	binary.LittleEndian.PutUint32(alvo[92+24:], 10)     // Hp
	mobID := w.SpawnMobAt(world.MobSpawn{Template: alvo, X: 6, Y: 5, GenIndex: -1})
	if mobID < 0 {
		t.Fatal("não consegui criar o monstro")
	}
	w.SetTickHandler(time.Hour, d.Tick) // sem IA: só o golpe do dono mata
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { _ = w.Serve(ctx, ln); close(done) }()

	c := enterWorld(t, ln.Addr().String())
	defer c.Close()

	skillAttackFrame(t, c, serverTime, 1, 56, damSkill) // evoca o Condor
	pets := collectPets(t, c, 400*time.Millisecond)
	if len(pets) == 0 {
		cancel()
		<-done
		t.Fatal("o Condor não nasceu")
	}

	clock.Add(2 * attackCadence)
	attackFrame(t, c, clock.Load(), mobID, -1)
	deadline := time.Now().Add(400 * time.Millisecond)
	for time.Now().Before(deadline) {
		h, _, ok := readMaybeHeaderRaw(t, c)
		if _, ehPet := pets[int(h.ID)]; ok && h.Type == protocol.MsgMotion && ehPet {
			t.Errorf("o pet %d recebeu a comemoração de nível: ganhou XP do abate", h.ID)
		}
	}

	cancel()
	<-done
	if w.Entity(mobID) != nil {
		t.Fatal("o monstro não morreu; o teste não exercitou o pagamento de XP")
	}
	for id := range pets {
		pet := w.Entity(id)
		if pet == nil {
			continue
		}
		if pet.Exp != 0 || pet.Level != 50 {
			t.Errorf("o pet %d ficou com Exp=%d e nível %d; nasce com Exp 0 no nível do dono (50) e não ganha XP de abate", id, pet.Exp, pet.Level)
		}
	}
}
