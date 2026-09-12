package handler

import (
	"strings"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/internal/level"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
)

// The zone is derived from the tile in the handler, so the tests state tiles and
// let level.ZoneForTile do the picking: the field, a tile inside the Água Arcano
// block (10,27) and one inside Pesadelo Arcano (9,1).
var (
	zonaCampo    = level.ZoneForTile(2100, 2100)
	zonaAgua     = level.ZoneForTile(10*128+5, 27*128+5)
	zonaPesadelo = level.ZoneForTile(9*128+5, 1*128+5)
)

func TestTextoXP(t *testing.T) {
	cases := []struct {
		name string
		st   estadoXP
		want []string
	}{
		{
			// Nothing equipped, no event: the answer still has to say so, because
			// "the command printed nothing" is indistinguishable from "the command
			// is broken".
			name: "sem nada",
			st:   estadoXP{Zona: zonaCampo, TaxaPercent: 100},
			want: []string{
				"Bônus de XP de itens aqui (Campo): nenhum.",
				"Servidor: EXP 0.5x · Kefra: vivo",
			},
		},
		{
			name: "bau, fada, montaria e equipamento",
			st: estadoXP{
				Zona:     zonaCampo,
				Bau:      100,
				BauTicks: 840, // 840 × 8s = 1h52
				Parcelas: expBonusParcelas{
					Fada: 16, Montaria: 12, Grade7: 6, Grade7Pecas: 3, Joia: 4, JoiaPecas: 2,
				},
				TaxaPercent: 100,
				Eventos:     expEventsView{KefraLive: true},
			},
			// Five pieces do not fit one 94-byte panel line, so the list wraps
			// instead of losing its tail to the encoder.
			want: []string{
				"Bônus de XP de itens aqui (Campo): +138%",
				"Vem de: Baú de XP +100% (1h52m) · Fada +16% · Montaria +12% · 3 de grade 7 +6%",
				"  2 com joia +4%",
				"Servidor: EXP 1x · Kefra: derrotado",
			},
		},
		{
			// The Fada Suprema's +30 is paid in Água, so it is folded into the
			// fairy's own number instead of being reported as a separate item the
			// player cannot find in their inventory.
			name: "fada suprema fora do pesadelo",
			st: estadoXP{
				Zona:        zonaAgua,
				Parcelas:    expBonusParcelas{Fada: 16},
				Suprema:     30,
				TaxaPercent: 100,
				Eventos:     expEventsView{KefraLive: true},
			},
			want: []string{
				"Bônus de XP de itens aqui (Água Arcano): +46%",
				"Vem de: Fada +46%",
				"Os +30% da sua fada não valem dentro do Pesadelo.",
				"Servidor: EXP 1x · Kefra: derrotado",
			},
		},
		{
			// The same character standing in Pesadelo: 16%, not 46%. This is the
			// case the command exists for.
			name: "fada suprema dentro do pesadelo",
			st: estadoXP{
				Zona:        zonaPesadelo,
				Parcelas:    expBonusParcelas{Fada: 16},
				Suprema:     30,
				TaxaPercent: 100,
				Eventos:     expEventsView{KefraLive: true},
			},
			want: []string{
				"Bônus de XP de itens aqui (Pesadelo Arcano): +16%",
				"Vem de: Fada +16%",
				"A sua fada daria +30% a mais, mas isso não vale dentro do Pesadelo.",
				"Servidor: EXP 1x · Kefra: derrotado",
			},
		},
		{
			// The silent one: at limiteBonusXP or above, the whole bonus is thrown
			// away. The headline has to read 0, and the reason has to be said.
			name: "teto de 500",
			st: estadoXP{
				Zona:        zonaCampo,
				Bau:         500,
				TaxaPercent: 100,
				Eventos:     expEventsView{KefraLive: true},
			},
			want: []string{
				"Bônus de XP de itens aqui (Campo): +0%",
				"Os seus itens somam +500%, mas de 500% para cima o jogo ignora o bônus inteiro.",
				"Vem de: Baú de XP +500%",
				"Servidor: EXP 1x · Kefra: derrotado",
			},
		},
		{
			// In a party the paid bonus is the best among whoever is in the fight,
			// which no command can know in advance — so the rule is stated and no
			// number is promised.
			name: "em grupo",
			st: estadoXP{
				Zona:        zonaCampo,
				Bau:         100,
				EmGrupo:     true,
				TaxaPercent: 100,
				Eventos:     expEventsView{KefraLive: true},
			},
			want: []string{
				"Bônus de XP de itens aqui (Campo): +100%",
				"Vem de: Baú de XP +100%",
				"Em grupo vale o maior bônus entre quem está na luta, não o seu.",
				"Servidor: EXP 1x · Kefra: derrotado",
			},
		},
		{
			name: "taxa da mesa e evento de novato",
			st: estadoXP{
				Zona:        zonaCampo,
				Bau:         100,
				TaxaPercent: 150,
				Eventos:     expEventsView{KefraLive: true, DoubleMode: true, NewbieEvent: true},
			},
			want: []string{
				"Bônus de XP de itens aqui (Campo): +100%",
				"Vem de: Baú de XP +100%",
				"Servidor: EXP 2x · Kefra: derrotado · novato +25% até o nível 100",
				"Taxa desta zona na Mesa de XP: 150%",
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := textoXP(tc.st)
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

// The panel silently cuts anything past MessageLength-2 CP1252 bytes, so every
// line the command can produce has to fit — including the longest zone name, a
// fully loaded breakdown and the accents that cost a byte each.
func TestTextoXPCabeNoPainel(t *testing.T) {
	st := estadoXP{
		Zona:     zonaPesadelo,
		Bau:      100,
		BauTicks: 324000,
		Parcelas: expBonusParcelas{
			Fada: 32, Montaria: 12, Grade7: 32, Grade7Pecas: 16, Joia: 32, JoiaPecas: 16,
		},
		Suprema:     30,
		EmGrupo:     true,
		TaxaPercent: 1000,
		Eventos:     expEventsView{KefraLive: true, DoubleMode: true, NewbieEvent: true},
	}
	for i, linha := range textoXP(st) {
		if n := len(protocol.ClientText(linha)); n > linhaPainelMax {
			t.Errorf("linha %d tem %d bytes (max %d): %q", i, n, linhaPainelMax, linha)
		}
	}
}

func TestJuntarPartes(t *testing.T) {
	cases := []struct {
		name    string
		prefixo string
		partes  []string
		largura int
		want    []string
	}{
		{
			name:    "vazio nao gera linha",
			prefixo: "Vem de: ",
			partes:  nil,
			largura: 94,
			want:    nil,
		},
		{
			name:    "cabe em uma linha",
			prefixo: "Vem de: ",
			partes:  []string{"a", "b"},
			largura: 94,
			want:    []string{"Vem de: a · b"},
		},
		{
			// The wrap keeps each piece whole and moves it down; it never cuts one
			// in half, which is the failure the panel would produce on its own.
			name:    "quebra em duas linhas",
			prefixo: "Vem de: ",
			partes:  []string{"aaaaa", "bbbbb", "ccccc"},
			largura: 20,
			want:    []string{"Vem de: aaaaa", "  bbbbb · ccccc"},
		},
		{
			// A single piece longer than the line still goes out whole: moving it
			// down would leave a line holding nothing but the prefix.
			name:    "parte sozinha maior que a linha",
			prefixo: "Vem de: ",
			partes:  []string{"aaaaaaaaaaaaaaaaaaaaaaaaa"},
			largura: 10,
			want:    []string{"Vem de: aaaaaaaaaaaaaaaaaaaaaaaaa"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := juntarPartes(tc.prefixo, tc.partes, tc.largura)
			if len(got) != len(tc.want) {
				t.Fatalf("linhas = %q, want %q", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("linha %d = %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestTempoAfeto(t *testing.T) {
	cases := []struct {
		ticks uint32
		want  string
	}{
		{0, "0s"},
		{1, "8s"},
		{8, "1m"},      // 64s
		{450, "1h00m"}, // AFFECT_1H
		{840, "1h52m"},
	}
	for _, tc := range cases {
		if got := tempoAfeto(tc.ticks); got != tc.want {
			t.Errorf("tempoAfeto(%d) = %q, want %q", tc.ticks, got, tc.want)
		}
	}
}

// The wiring test: /xp has to reach the handler through the whisper route and
// come back on the panel channel, because the command keyword IS the whisper
// target and a keyword the server does not know is answered with "O jogador não
// está conectado."
func TestCommandXPBonus(t *testing.T) {
	for _, cmd := range []string{"xp", "bonus"} {
		t.Run(cmd, func(t *testing.T) {
			addr, stop, _ := startServerClock(t, newDB())
			defer stop()
			conn := enterWorld(t, addr)
			defer conn.Close()

			whisperFrame(t, conn, cmd, "")
			got := decodePanel(expect(t, conn, protocol.MsgMessagePanel))
			if !strings.HasPrefix(got, "Bônus de XP de itens aqui (") {
				t.Fatalf("/%s = %q, want the bonus headline", cmd, got)
			}
			// The server half always follows, so the player is told about the
			// event switches even with no gear at all.
			if got := decodePanel(expect(t, conn, protocol.MsgMessagePanel)); !strings.HasPrefix(got, "Servidor: EXP ") {
				t.Errorf("/%s segunda linha = %q, want the server line", cmd, got)
			}
		})
	}
}
