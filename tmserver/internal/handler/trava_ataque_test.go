package handler

import (
	"bytes"
	"io"
	"log/slog"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// golpeCorpoACorpo sends a melee attack that says where the attacker stands and
// where it is aiming, the two points the legacy reach test reads.
func golpeCorpoACorpo(t *testing.T, c net.Conn, tick uint32, alvo int, px, py, tx, ty uint16) {
	t.Helper()
	body := protocol.MsgAttackBody{
		PosX: px, PosY: py, TargetX: tx, TargetY: ty,
		SkillIndex: -1,
		Dam:        []protocol.DamEntry{{TargetID: int32(alvo), Damage: damMelee}},
	}
	wire, err := protocol.Encode(protocol.Header{Type: protocol.MsgAttack, ClientTick: tick}, body.Encode(), 9)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Write(wire); err != nil {
		t.Fatal(err)
	}
}

// ecoDoGolpe waits for the attacker's own attack echo — the server answers with
// the Type it received, Attack, AttackOne or AttackTwo — skipping anything else
// it happens to push meanwhile (regen, visibility). ok is false when no echo
// arrives: the attack was refused.
func ecoDoGolpe(t *testing.T, c net.Conn) (protocol.MsgAttackBody, bool) {
	t.Helper()
	fim := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(fim) {
		ty, payload, ok := readMaybe(t, c)
		if !ok {
			continue
		}
		if ty != protocol.MsgAttack && ty != protocol.MsgAttackOne && ty != protocol.MsgAttackTwo {
			continue
		}
		var got protocol.MsgAttackBody
		if err := got.Decode(payload); err != nil {
			t.Fatal(err)
		}
		return got, true
	}
	return protocol.MsgAttackBody{}, false
}

// umMobNoCampo is one attacker beside one 5000-HP field mob, on a server whose
// clock sits at serverTime.
func umMobNoCampo(t *testing.T) (net.Conn, *world.World, func()) {
	t.Helper()
	addr, stop, w := startServerSkillsTargetMob(t, skillCombatDB(0), targetMob("Orc", 0, 5000))
	c := enterWorld(t, addr)
	return c, w, func() { c.Close(); stop() }
}

func vidaDoMob(w *world.World) int32 {
	if m := w.Entity(world.MaxUser); m != nil {
		return m.HP
	}
	return -1
}

// Trava 1, _MSG_Attack.cpp:79-96: the attack's ClientTick has to sit within
// [now-120000, now+15000] of the SERVER clock. Before it came back, the 800 ms
// cadence only compared the client's clock with itself, so a client writing
// its own ticks attacked as fast as it could send.
func TestTravaJanelaRecusaRelogioAdiantado(t *testing.T) {
	c, w, fim := umMobNoCampo(t)
	defer fim()

	golpeCorpoACorpo(t, c, serverTime+20_000, world.MaxUser, 6, 5, 6, 5)
	if _, eco := ecoDoGolpe(t, c); eco {
		t.Fatal("um golpe 20 s adiante do relógio do servidor foi aceito")
	}
	if hp := vidaDoMob(w); hp != 5000 {
		t.Errorf("o mob levou dano de um golpe recusado: HP = %d", hp)
	}

	// A ordem do legado: o tick adiantado já moveu LastAttackTick antes de a
	// janela recusar (:75 antes de :79), então quem mandou fica preso na
	// cadência até o relógio de verdade alcançar o tick que ele inventou.
	golpeCorpoACorpo(t, c, serverTime+1_000, world.MaxUser, 6, 5, 6, 5)
	if _, eco := ecoDoGolpe(t, c); eco {
		t.Error("depois do tick adiantado, um golpe no relógio certo passou; " +
			"no legado quem adianta o relógio se tranca na cadência")
	}
}

func TestTravaJanelaRecusaRelogioAtrasado(t *testing.T) {
	c, w, fim := umMobNoCampo(t)
	defer fim()

	golpeCorpoACorpo(t, c, serverTime-130_000, world.MaxUser, 6, 5, 6, 5)
	if _, eco := ecoDoGolpe(t, c); eco {
		t.Fatal("um golpe 130 s atrás do relógio do servidor foi aceito")
	}
	if hp := vidaDoMob(w); hp != 5000 {
		t.Errorf("o mob levou dano de um golpe recusado: HP = %d", hp)
	}

	// O relógio atrasado não tranca nada: o próximo golpe honesto passa.
	golpeCorpoACorpo(t, c, serverTime+1_000, world.MaxUser, 6, 5, 6, 5)
	got, eco := ecoDoGolpe(t, c)
	if !eco {
		t.Fatal("o golpe honesto depois de um atrasado foi recusado")
	}
	if got.Dam[0].Damage <= 0 {
		t.Errorf("golpe honesto sem dano: %d", got.Dam[0].Damage)
	}
}

// A folga do legado é larga de propósito, e é ela que protege o jogador
// honesto: um cliente até 15 s adiantado continua batendo.
func TestTravaJanelaAceitaDentroDaFolga(t *testing.T) {
	c, _, fim := umMobNoCampo(t)
	defer fim()

	golpeCorpoACorpo(t, c, serverTime+14_000, world.MaxUser, 6, 5, 6, 5)
	got, eco := ecoDoGolpe(t, c)
	if !eco {
		t.Fatal("um golpe 14 s adiante, dentro da folga do legado, foi recusado")
	}
	if got.Dam[0].Damage <= 0 {
		t.Errorf("golpe dentro da folga sem dano: %d", got.Dam[0].Damage)
	}
}

// Trava 2, _MSG_Attack.cpp:424-426: melee farther than 23 by the packet's own
// coordinates is refused whole and in silence. 23 because the legacy
// overwrites every player's Range with 23 (CMob.cpp:696-697). The edge is
// BASE_GetDistance's: past 6 tiles it is the larger axis plus one, so 22 tiles
// apart is distance 23 (hits) and 23 apart is 24 (refused).
func TestTravaDistanciaDoCorpoACorpo(t *testing.T) {
	c, w, fim := umMobNoCampo(t)
	defer fim()

	golpeCorpoACorpo(t, c, serverTime, world.MaxUser, 100, 100, 123, 100)
	if _, eco := ecoDoGolpe(t, c); eco {
		t.Fatal("um golpe de corpo a corpo a distância 24 foi aceito")
	}
	if hp := vidaDoMob(w); hp != 5000 {
		t.Errorf("o mob levou dano de um golpe recusado: HP = %d", hp)
	}

	golpeCorpoACorpo(t, c, serverTime+1_000, world.MaxUser, 100, 100, 122, 100)
	got, eco := ecoDoGolpe(t, c)
	if !eco {
		t.Fatal("um golpe de corpo a corpo a distância 23, no limite, foi recusado")
	}
	if got.Dam[0].Damage <= 0 {
		t.Errorf("golpe no limite sem dano: %d", got.Dam[0].Damage)
	}
}

// golpeDuplo is one melee packet naming the same target twice, the smallest
// packet with a second melee entry.
func golpeDuplo(t *testing.T, c net.Conn, tick uint32, alvo int) {
	t.Helper()
	body := protocol.MsgAttackBody{
		PosX: 5, PosY: 5, TargetX: 6, TargetY: 5, SkillIndex: -1,
		Dam: []protocol.DamEntry{{TargetID: int32(alvo), Damage: damMelee}, {TargetID: int32(alvo), Damage: damMelee}},
	}
	wire, err := protocol.Encode(protocol.Header{Type: protocol.MsgAttackTwo, ClientTick: tick}, body.Encode(), 9)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Write(wire); err != nil {
		t.Fatal(err)
	}
}

// Trava 3, by the intent of _MSG_Attack.cpp:431: a second melee target is only
// for a Huntress or a character with skill 0x40. Anyone else keeps the first
// hit and loses the rest, in silence.
func TestTravaSegundoAlvoDoCorpoACorpo(t *testing.T) {
	casos := []struct {
		nome        string
		classe      int
		aprendida   int32
		segundoBate bool
	}{
		{"TransKnight sem a 0x40: só o primeiro alvo", 0, 0, false},
		{"TransKnight com a 0x40: os dois", 0, 0x40, true},
		{"Caçadora: os dois", 3, 0, true},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			db := skillCombatDB(c.aprendida)
			db.loadResult.Class = c.classe
			addr, stop, _ := startServerSkillsTargetMob(t, db, targetMob("Orc", 0, 5000))
			defer stop()
			conn := enterWorld(t, addr)
			defer conn.Close()

			golpeDuplo(t, conn, serverTime, world.MaxUser)
			got, eco := ecoDoGolpe(t, conn)
			if !eco || len(got.Dam) != 2 {
				t.Fatalf("eco = %v, Dam = %+v; queria o eco com as duas entradas", eco, got.Dam)
			}
			if got.Dam[0].Damage <= 0 {
				t.Errorf("o primeiro alvo ficou sem dano: %d", got.Dam[0].Damage)
			}
			if bateu := got.Dam[1].Damage > 0; bateu != c.segundoBate {
				t.Errorf("segundo alvo: dano %d, queria bater = %v", got.Dam[1].Damage, c.segundoBate)
			}
		})
	}
}

// Trava 4, _MSG_Attack.cpp:347-351: a target off the attacker's screen, by the
// SERVER's positions, is dropped and the attacker's client is told to remove
// it. The packet's own coordinates are made to look close on purpose: this is
// the gate that does not believe them.
func TestTravaTelaUsaAPosicaoDoServidor(t *testing.T) {
	// Herói em (5,5), mob em (60,60): 55 casas, além das 33 da tela.
	addr, stop, w := startServerSkillsTargetMobAt(t, skillCombatDB(0), targetMob("Orc", 0, 5000), 64, 60, 60)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	golpeCorpoACorpo(t, c, serverTime, world.MaxUser, 59, 60, 60, 60)

	var removido, dano bool
	fim := time.Now().Add(600 * time.Millisecond)
	for time.Now().Before(fim) {
		h, payload, ok := lerQualquerQuadro(t, c)
		if !ok {
			continue
		}
		switch h.Type {
		case protocol.MsgRemoveMob:
			if int(h.ID) == world.MaxUser && len(payload) >= 4 && payload[0] == 1 {
				removido = true
			}
		case protocol.MsgAttack:
			var got protocol.MsgAttackBody
			if err := got.Decode(payload); err == nil && len(got.Dam) > 0 && got.Dam[0].Damage > 0 {
				dano = true
			}
		}
	}
	if dano {
		t.Error("o golpe num mob fora da tela saiu com dano")
	}
	if hp := vidaDoMob(w); hp != 5000 {
		t.Errorf("o mob fora da tela levou dano: HP = %d", hp)
	}
	if !removido {
		t.Error("o cliente de quem bateu não recebeu o RemoveMob do alvo fora da tela")
	}
}

// lerQualquerQuadro reads one frame of any type, visibility included.
func lerQualquerQuadro(t *testing.T, c net.Conn) (protocol.Header, []byte, bool) {
	t.Helper()
	_ = c.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	var sz [2]byte
	if _, err := io.ReadFull(c, sz[:]); err != nil {
		return protocol.Header{}, nil, false
	}
	buf := make([]byte, int(sz[0])|int(sz[1])<<8)
	copy(buf, sz[:])
	if _, err := io.ReadFull(c, buf[2:]); err != nil {
		return protocol.Header{}, nil, false
	}
	h, payload, _, err := protocol.Decode(buf)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	return h, payload, true
}

// The counter is what tells an honest client tripping a gate apart from a
// cheater: every refusal counts per account, and the log gets the 1st, 10th and
// 100th so neither case stays invisible nor floods it.
func TestRecusaDeAtaqueContaPorConta(t *testing.T) {
	var buf bytes.Buffer
	d := New(Config{Log: slog.New(slog.NewTextHandler(&buf, nil))})
	s := &world.Session{Conn: 3, AccountName: "fulano"}

	for range 100 {
		d.recusarAtaque(s, travaJanela)
	}
	d.recusarAtaque(s, travaDistancia)

	if got := s.AttackRefusals[travaJanela]; got != 100 {
		t.Errorf("recusas da janela = %d, want 100", got)
	}
	if got := s.AttackRefusals[travaDistancia]; got != 1 {
		t.Errorf("recusas da distância = %d, want 1", got)
	}
	var janela, distancia int
	for _, linha := range strings.Split(buf.String(), "\n") {
		if !strings.Contains(linha, "attack refused by a restored legacy gate") {
			continue
		}
		if !strings.Contains(linha, "account=fulano") {
			t.Errorf("linha de recusa sem a conta: %s", linha)
		}
		switch {
		case strings.Contains(linha, "gate=janela"):
			janela++
		case strings.Contains(linha, "gate=distancia"):
			distancia++
		}
	}
	if janela != 3 {
		t.Errorf("a janela foi ao log %d vezes em 100 recusas, want 3 (1ª, 10ª, 100ª)", janela)
	}
	if distancia != 1 {
		t.Errorf("a distância foi ao log %d vezes na 1ª recusa, want 1", distancia)
	}
}
