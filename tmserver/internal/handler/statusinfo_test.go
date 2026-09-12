package handler

import (
	"strings"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
)

func TestTextoStatus(t *testing.T) {
	cases := []struct {
		name string
		st   estadoStatus
		want []string
	}{
		{
			// A Mortal with nothing: no tier protects it, so the Defesa de
			// Evolução says nothing at all rather than printing three 100%s.
			name: "mortal sem nada",
			st:   estadoStatus{Defesa: 3811, Tier: classMasterMortal},
			want: []string{
				"Defesa 3811 · contra jogador vale 11433, porque a Defesa conta 3x em PvP.",
				"Bônus de drop dos seus itens: nenhum.",
				"Bônus de XP: digite /xp.",
			},
		},
		{
			// An Arch is only protected from Mortals; another Arch hits it whole,
			// so only one Defesa de Evolução line is produced.
			name: "arch com bloco pvp",
			st: estadoStatus{
				Defesa:    5000,
				Tier:      classMasterArch,
				AtaquePvP: 12,
				DefesaPvP: 8,
				Reflect:   40,
				DropBonus: 18,
			},
			want: []string{
				"Defesa 5000 · contra jogador vale 15000, porque a Defesa conta 3x em PvP.",
				"Defesa de Evolução: um Mortal te acerta com 20% do dano dele.",
				"Em PvP: ataque +12% · defesa +8% · absorve 40 por golpe",
				"Bônus de drop dos seus itens: +18%",
				"Bônus de XP: digite /xp.",
			},
		},
		{
			name: "celestial completo",
			st: estadoStatus{
				Defesa:      8000,
				Tier:        classMasterCelestial,
				AtaquePvP:   12,
				DefesaPvP:   8,
				Reflect:     40,
				Perfuracao:  480,
				TemMontaria: true,
				MontariaPvP: 40,
				MontariaPvE: 25,
				AbsHp:       20,
				DropBonus:   26,
			},
			want: []string{
				"Defesa 8000 · contra jogador vale 24000, porque a Defesa conta 3x em PvP.",
				"Defesa de Evolução: um Mortal te acerta com 10% do dano dele.",
				"Defesa de Evolução: um Arch te acerta com 40% do dano dele.",
				"Em PvP: ataque +12% · defesa +8% · absorve 40 por golpe",
				"  perfuração +480 que passa pela defesa",
				"A sua montaria come 40% do golpe de jogador e 25% do de monstro.",
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

// Same rule as the /xp lines: the panel cuts past MessageLength-2 CP1252 bytes.
func TestTextoStatusCabeNoPainel(t *testing.T) {
	st := estadoStatus{
		Defesa:      32767,
		Tier:        classMasterCelestial,
		AtaquePvP:   999,
		DefesaPvP:   999,
		Reflect:     99999,
		Perfuracao:  999999,
		TemMontaria: true,
		MontariaPvP: 100,
		MontariaPvE: 100,
		AbsHp:       100,
		DropBonus:   999,
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
	if !strings.HasPrefix(got, "Defesa ") {
		t.Fatalf("/status = %q, want the Defesa headline", got)
	}
}
