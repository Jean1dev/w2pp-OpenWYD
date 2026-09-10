package handler

import (
	"encoding/binary"
	"net"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/combine"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/content"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
)

// sendAilynAsTheClient is sendAilynCombine the way the real client builds it: a
// stackable catalyst goes out carrying EF_AMOUNT 1 even though the bag stores it
// with no amount effect at all. Byte-for-byte comparison refused that recipe
// outright, and silently — the +10 bug the players hit first.
func sendAilynAsTheClient(t *testing.T, c net.Conn) {
	t.Helper()
	var body protocol.MsgCombineItemBody
	for i := 0; i < 7; i++ {
		body.InvenPos[i] = uint8(i)
	}
	body.Item[0] = protocol.WireItem{Index: aylinTarget, Effects: [3]protocol.WireEffect{{Effect: efSanc, Value: 9}, {Effect: efDamage, Value: 11}}}
	body.Item[1] = protocol.WireItem{Index: aylinTarget, Effects: [3]protocol.WireEffect{{Effect: efSanc, Value: 9}, {Effect: efDamage, Value: 22}}}
	body.Item[2] = protocol.WireItem{Index: itemPedraDoSabio, Effects: [3]protocol.WireEffect{{Effect: efAmount, Value: 1}}}
	for i := 3; i < 7; i++ {
		body.Item[i] = protocol.WireItem{Index: anctDiamond, Effects: [3]protocol.WireEffect{{Effect: efAmount, Value: 1}}}
	}
	send(t, c, protocol.MsgCombineItemAilyn, body.Encode())
}

// ailynDaMesa pins the +10 chance through the Mesa das Máquinas, the same row a
// moderator saves on /rates/maquinas. The row IS the final chance: 1 is the
// lowest there is and 100 never fails.
func ailynDaMesa(taxa int32, bands ...combine.Band) combine.RateConfig {
	return combine.NewRateConfig(1, []combine.RateRow{{Family: "Ailyn", Key: chaveMais10Chance, Rate: taxa}}, bands)
}

// faixaParaTudo is one band covering every ReqLvl of both +10 kinds, so a test
// can apply a multiplier without caring which tier the test item sits in.
func faixaParaTudo(multPct int32) []combine.Band {
	return []combine.Band{
		{SlotKind: combine.SlotWeapon, ReqLvlMin: 0, ReqLvlMax: 100000, Label: "Tudo", MultPct: multPct},
		{SlotKind: combine.SlotArmour, ReqLvlMin: 0, ReqLvlMax: 100000, Label: "Tudo", MultPct: multPct},
	}
}

// resultadoDaAilyn collects everything the +10 sends up to and including the
// CombineComplete: the text lines, the parm, and the coin the UpdateEtc reports.
type resultadoDaAilyn struct {
	textos    []string
	parm      int32
	moeda     int32
	viuMoeda  bool
	consumido map[int]bool // carry slots the server reported emptied
}

func lerResultadoDaAilyn(t *testing.T, c net.Conn) resultadoDaAilyn {
	t.Helper()
	r := resultadoDaAilyn{parm: -1, consumido: map[int]bool{}}
	for i := 0; i < 24; i++ {
		ty, p, ok := readMaybe(t, c)
		if !ok {
			t.Fatal("a máquina parou de responder antes do CombineComplete: a janela do cliente fica travada")
		}
		switch ty {
		case protocol.MsgMessagePanel:
			r.textos = append(r.textos, decodePanel(p))
		case protocol.MsgUpdateEtc:
			r.moeda, r.viuMoeda = int32(binary.LittleEndian.Uint32(p[28:32])), true
		case protocol.MsgSendItem:
			if len(p) >= 6 && binary.LittleEndian.Uint16(p[4:6]) == 0 {
				r.consumido[int(binary.LittleEndian.Uint16(p[2:4]))] = true
			}
		case protocol.MsgCombineComplete:
			r.parm = parmOf(t, p)
			return r
		}
	}
	t.Fatal("frames demais antes do CombineComplete")
	return r
}

// TestAilynMais10FalaOResultado is the +10 end to end, as the player meets it:
// the client's own description of the stones, the chance set on the panel, and
// the line in the chat that says which way the roll went.
//
// Both outcomes cost the same — the gold and the five catalysts go before the
// roll, as in _MSG_CombineItemAilyn.cpp:65-75 — so without the line the player
// could not tell a lost roll from a machine that ate the items.
func TestAilynMais10FalaOResultado(t *testing.T) {
	// The chance after the slash is exactly the Mesa's number for this item:
	// what the moderator typed, times the band of the item's tier.
	casos := []struct {
		nome       string
		mesa       combine.RateConfig
		wantParm   int32
		wantLinha  *regexp.Regexp
		wantChance int
	}{
		{"sucesso", ailynDaMesa(100), combineSuccess, regexp.MustCompile(`^Hero conseguiu em (\d+)/(\d+) passar #\d+ para \+10!$`), 100},
		{"falha", ailynDaMesa(1), combineFailed, regexp.MustCompile(`^Hero falhou em (\d+)/(\d+) ao passar #\d+ para \+10\.$`), 1},
	}
	for _, tc := range casos {
		t.Run(tc.nome, func(t *testing.T) {
			addr, stop := startServerAilynRates(t, ailynCost, true, tc.mesa)
			defer stop()
			c := enterWorld(t, addr)
			defer c.Close()

			sendAilynAsTheClient(t, c)
			r := lerResultadoDaAilyn(t, c)

			if r.parm != tc.wantParm {
				t.Fatalf("CombineComplete parm = %d, esperado %d — a receita não passou ou o sorteio não obedeceu à Mesa", r.parm, tc.wantParm)
			}
			if len(r.textos) != 1 {
				t.Fatalf("linhas no chat = %q, esperado exatamente um anúncio", r.textos)
			}
			m := tc.wantLinha.FindStringSubmatch(r.textos[0])
			if m == nil {
				t.Fatalf("anúncio = %q, não bate com %s", r.textos[0], tc.wantLinha)
			}
			sorteio, _ := strconv.Atoi(m[1])
			chance, _ := strconv.Atoi(m[2])
			if chance != tc.wantChance {
				t.Errorf("chance anunciada = %d, esperado %d (a que o sorteio usou)", chance, tc.wantChance)
			}
			// The line must agree with its own outcome: a roll at or under the
			// chance is a success (combine.Roll), anything above is a failure.
			if (sorteio <= chance) != (tc.wantParm == combineSuccess) {
				t.Errorf("anúncio %d/%d contradiz o desfecho parm=%d", sorteio, chance, tc.wantParm)
			}
			if !r.viuMoeda || r.moeda != 0 {
				t.Errorf("moeda após a +10 = %d (viu=%v), esperado 0: os 50M saem nos dois desfechos", r.moeda, r.viuMoeda)
			}
			for slot := 2; slot < 7; slot++ {
				if !r.consumido[slot] {
					t.Errorf("slot %d (pedra/joia) não foi consumido", slot)
				}
			}
		})
	}
}

// TestAilynMais10RecusaSemCobrar covers the other message the +10 owes: a wrong
// recipe names itself and keeps the gold and every item where they were.
func TestAilynMais10RecusaSemCobrar(t *testing.T) {
	addr, stop := startServerAilyn(t, ailynCost, false)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	sendAilynCombine(t, c, 2442) // Esmeralda where this grade wants Diamante
	r := lerResultadoDaAilyn(t, c)
	if r.parm != combineInvalid {
		t.Fatalf("parm = %d, esperado invalid(0)", r.parm)
	}
	if len(r.textos) != 1 || r.textos[0] != msgWrongCombination {
		t.Errorf("texto = %q, esperado [%q]", r.textos, msgWrongCombination)
	}
	if r.viuMoeda || len(r.consumido) > 0 {
		t.Errorf("a recusa cobrou (moeda=%v) ou consumiu %v", r.viuMoeda, r.consumido)
	}
}

// TestAilynMais10SemOuroDizOPreco: the gold refusal names the price, as
// _DN_D_Cost does, instead of a generic "no".
func TestAilynMais10SemOuroDizOPreco(t *testing.T) {
	addr, stop := startServerAilyn(t, ailynCost-1, true)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	sendAilynCombine(t, c, anctDiamond)
	r := lerResultadoDaAilyn(t, c)
	if r.parm != combineInvalid {
		t.Fatalf("parm = %d, esperado invalid(0)", r.parm)
	}
	if want := combineNeedsGold(ailynCost); len(r.textos) != 1 || r.textos[0] != want {
		t.Errorf("texto = %q, esperado [%q]", r.textos, want)
	}
}

// TestMensagensDasMaquinasCabemNoCliente: every outcome line is a Go literal with
// accents, and the panel is CP1252 with 94 usable bytes. A '?' in the encoding is
// a lost character the player would read as garbage.
func TestMensagensDasMaquinasCabemNoCliente(t *testing.T) {
	for _, msg := range []string{
		msgProcessingComplete, msgCombineFailed, msgWrongCombination,
		combineNeedsGold(ailynCost),
	} {
		encoded := protocol.ClientText(msg)
		for _, b := range encoded {
			if b == '?' {
				t.Errorf("%q tem caractere fora do CP1252 do cliente", msg)
				break
			}
		}
		if len(encoded) > 94 {
			t.Errorf("%q tem %d bytes, acima dos 94 do painel", msg, len(encoded))
		}
	}
}

// TestAnunciosCabemNoPainel: the broadcast is a MessagePanel like any other line
// — 94 usable bytes, and the client silently drops whatever does not fit. It
// builds the longest line each machine can produce, for every piece of
// equipment in the real catalog: a 16-byte name (MobName's full width) and a
// three-digit roll and chance.
func TestAnunciosCabemNoPainel(t *testing.T) {
	root := filepath.Join("..", "..", "..", "Release", "Common")
	items, err := content.LoadItemList(filepath.Join(root, "ItemList.csv"))
	if err != nil {
		t.Skipf("ItemList.csv unavailable: %v", err)
	}
	const nome = "NomeDeDezesseis!"
	equipamento := map[int]bool{2: true, 4: true, 8: true, 16: true, 32: true, 64: true, 128: true, 192: true}
	pos := items.Positions()
	pior, piorLinha := 0, ""
	for idx := range pos {
		if !equipamento[pos[idx]] {
			continue
		}
		e, ok := items.Get(idx)
		if !ok || e.Name == "" {
			continue
		}
		for _, linha := range []string{
			nome + " falhou em 104/100 ao passar " + e.Name + " para +10.",
			nome + " conseguiu em 104/100 passar " + e.Name + " para +10!",
			nome + " falhou em 104/100 ao compor " + e.Name + ".",
			nome + " falhou em 104/100 ao passar o ADD para " + e.Name + ".",
		} {
			if n := len(protocol.ClientText(linha)); n > pior {
				pior, piorLinha = n, linha
			}
		}
	}
	if pior > 94 {
		t.Errorf("o anúncio mais longo tem %d bytes, acima dos 94 do painel: %q", pior, piorLinha)
	}
	t.Logf("pior caso: %d bytes — %q", pior, piorLinha)
}
