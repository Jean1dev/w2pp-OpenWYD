package handler

import (
	"strings"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/combat"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

func TestTextoStatus(t *testing.T) {
	cases := []struct {
		name string
		st   estadoStatus
		want []string
	}{
		{
			// A Mortal with nothing but DEX: no tier protects it and it carries no
			// PvP gear, so the sheet is just the two rolls.
			// DEX 300 → dodge 150, accuracy 60, mirror 90 (9.0%).
			name: "mortal so com dex",
			st: estadoStatus{
				Esquiva: 150, Precisao: 60, EsquivaEspelho: 90,
				Tier: classMasterMortal,
			},
			want: []string{
				"Acerto 91.0% · Esquiva 9.0% — contra alguém igual a você.",
				"Precisão 60 (tira da esquiva do alvo) · a sua esquiva 150 em 1000, teto 650.",
				"Bônus de XP: digite /xp.",
			},
		},
		{
			// DEX 700 with a mount lending 20 evasion → dodge 370, accuracy 140,
			// mirror 230. An Arch is only protected from Mortals.
			name: "arch com bloco pvp",
			st: estadoStatus{
				Esquiva: 370, Precisao: 140, EsquivaEspelho: 230,
				Reflect: 40, AtaquePvP: 12, DefesaPvP: 8,
				Tier: classMasterArch, DropBonus: 18,
			},
			want: []string{
				"Acerto 77.0% · Esquiva 23.0% — contra alguém igual a você.",
				"Precisão 140 (tira da esquiva do alvo) · a sua esquiva 370 em 1000, teto 650.",
				"Contra jogador: absorve 40 de cada golpe · e mais 8% do que sobrou · você bate +12%",
				"Defesa de Evolução: um Mortal te acerta com 20% do dano dele.",
				"Bônus de drop dos seus itens: +18%",
				"Bônus de XP: digite /xp.",
			},
		},
		{
			// Perfuração leads the PvP line: it is the only number that ignores
			// armour outright.
			name: "celestial completo",
			st: estadoStatus{
				Esquiva: 520, Precisao: 200, EsquivaEspelho: 320,
				Perfuracao: 480, Reflect: 40, AtaquePvP: 12, DefesaPvP: 8,
				Tier:        classMasterCelestial,
				TemMontaria: true, MontariaPvP: 40, MontariaPvE: 25,
				AbsHp: 20, DropBonus: 26,
			},
			want: []string{
				"Acerto 68.0% · Esquiva 32.0% — contra alguém igual a você.",
				"Precisão 200 (tira da esquiva do alvo) · a sua esquiva 520 em 1000, teto 650.",
				"Contra jogador: perfuração +480 que passa pela defesa · absorve 40 de cada golpe",
				"  e mais 8% do que sobrou · você bate +12%",
				"Defesa de Evolução: um Mortal te acerta com 10% do dano dele.",
				"Defesa de Evolução: um Arch te acerta com 40% do dano dele.",
				"A sua montaria absorve 40% do golpe de jogador e 25% do de monstro.",
				"Jóia da Absorção: metade dos golpes devolve 20% do dano em vida, até 350.",
				"Bônus de drop dos seus itens: +26%",
				"Bônus de XP: digite /xp.",
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := textoStatus(tc.st)
			if len(got) != len(tc.want) {
				t.Fatalf("linhas = %d, want %d:\n%s", len(got), len(tc.want), strings.Join(got, "\n"))
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("linha %d =\n  %q\nwant\n  %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

// The two halves reported must be the ones the attack path rolls, not a restated
// copy: dodge is ParryRate with no attacker, accuracy is precisaoDe, and the two
// together must reproduce exactly what parryRate returns for this character
// swinging at a copy of itself.
func TestStatusEsquivaEAcertoBatemComORoll(t *testing.T) {
	d := New(Config{})
	for _, dex := range []int16{12, 300, 700, 1200, 3500} {
		e := &world.Entity{ID: 3, Dex: dex, Parry: 20}
		espelho := &world.Entity{ID: 4, Dex: dex, Parry: 20}

		precisao := precisaoDe(e, int(effectiveDex(e)))
		esperado := combat.ParryRate(int(effectiveDex(espelho)), espelho.Parry, precisao, int(e.Rsv))

		if got := d.parryRate(e, espelho); got != esperado {
			t.Errorf("DEX %d: a conta do /status deu %d, o roll do combate dá %d", dex, esperado, got)
		}
		// And the defender half on its own is the same expression with a zero
		// attacker, which is what the screen prints as "a sua esquiva".
		if esquiva := combat.ParryRate(int(effectiveDex(e)), e.Parry, 0, 0); esquiva < esperado {
			t.Errorf("DEX %d: esquiva crua %d menor que a esquiva já descontada %d", dex, esquiva, esperado)
		}
	}
}

func TestPctMilesimos(t *testing.T) {
	cases := []struct {
		v    int
		want string
	}{
		{0, "0.0%"},
		{1, "0.1%"},
		{90, "9.0%"},
		{230, "23.0%"},
		{650, "65.0%"}, // o teto do roll
		{1000, "100.0%"},
	}
	for _, tc := range cases {
		if got := pctMilesimos(tc.v); got != tc.want {
			t.Errorf("pctMilesimos(%d) = %q, want %q", tc.v, got, tc.want)
		}
	}
}

// Same rule as the /xp lines: the panel cuts past MessageLength-2 CP1252 bytes.
func TestTextoStatusCabeNoPainel(t *testing.T) {
	st := estadoStatus{
		Esquiva: 650, Precisao: 9999, EsquivaEspelho: 650,
		Perfuracao: 999999, Reflect: 99999, AtaquePvP: 999, DefesaPvP: 999,
		Tier:        classMasterCelestial,
		TemMontaria: true, MontariaPvP: 100, MontariaPvE: 100,
		AbsHp: 100, DropBonus: 999,
	}
	for i, linha := range textoStatus(st) {
		if n := len(protocol.ClientText(linha)); n > linhaPainelMax {
			t.Errorf("linha %d tem %d bytes (max %d): %q", i, n, linhaPainelMax, linha)
		}
	}
}

// The wiring test: /status has to arrive through the whisper route, like every
// other slash command.
func TestCommandStatus(t *testing.T) {
	addr, stop, _ := startServerClock(t, newDB())
	defer stop()
	conn := enterWorld(t, addr)
	defer conn.Close()

	whisperFrame(t, conn, "status", "")
	got := decodePanel(expect(t, conn, protocol.MsgMessagePanel))
	if !strings.HasPrefix(got, "Acerto ") {
		t.Fatalf("/status = %q, want the hit/dodge headline", got)
	}
}
