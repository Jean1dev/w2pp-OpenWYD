package handler

import (
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// TestCreateGuildFalaCadaRecusa: every rule /create can break answers with its
// own line. Before, only the coin one tried to — through a notice with no text —
// so a player typing /create and seeing nothing could not tell a typo from a
// rule, and "why can't I make my guild" had no answer in game.
func TestCreateGuildFalaCadaRecusa(t *testing.T) {
	quinta := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	domingo := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
	apto := world.Entity{Clan: 7, Citizen: 1, Coin: guildCreateCost}

	casos := []struct {
		nome   string
		agora  time.Time
		guilda string
		mudar  func(e *world.Entity)
		want   string
	}{
		{"sem nome", quinta, "", nil, msgGuildUso},
		{"nome longo demais", quinta, "NomeComMaisDe16Letras", nil, msgGuildUso},
		{"sem ouro", quinta, "Lobos", func(e *world.Entity) { e.Coin = guildCreateCost - 1 }, "Você precisa de 100000000 Gold."},
		{"já tem guilda", quinta, "Lobos", func(e *world.Entity) { e.Guild = 5 }, msgGuildJaTem},
		{"sem reino", quinta, "Lobos", func(e *world.Entity) { e.Clan = 0 }, msgGuildReino},
		{"sem cidadania", quinta, "Lobos", func(e *world.Entity) { e.Citizen = 0 }, msgGuildSemCidadania},
		{"domingo", domingo, "Lobos", nil, msgGuildDomingo},
		{"nome já usado", quinta, "Existente", nil, "Já existe uma guilda chamada Existente."},
		{"tudo certo", quinta, "Lobos", nil, ""},
	}
	for _, tc := range casos {
		t.Run(tc.nome, func(t *testing.T) {
			agora := tc.agora
			d := New(Config{Log: slog.New(slog.NewTextHandler(io.Discard, nil)), Now: func() time.Time { return agora }})
			w := world.New(world.Config{GridDim: 16}, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil)
			w.SetGuildName(3, "Existente")
			e := apto
			if tc.mudar != nil {
				tc.mudar(&e)
			}
			if got := d.guildCreateRefusal(w, &e, tc.guilda); got != tc.want {
				t.Errorf("recusa = %q, esperado %q", got, tc.want)
			}
		})
	}
}

// TestCreateGuildDizONumero: the success line names the guild's number — what its
// icon file is named after (b01NNNNNN.bmp) — and the name is registered at once,
// so a second guild cannot take it before the next boot.
func TestCreateGuildDizONumero(t *testing.T) {
	db := newDB()
	db.loads = map[int64]world.CharacterState{
		7:  {Slot: 0, Name: "Hero", X: 5, Y: 5, HP: 1000, MaxHP: 1000, Clan: 7, Citizen: 1, Coin: guildCreateCost},
		11: {Slot: 0, Name: "HeroB", X: 6, Y: 5, HP: 1000, MaxHP: 1000, Clan: 7, Citizen: 1, Coin: guildCreateCost},
	}
	addr, stop, _ := startServerClock(t, db)
	defer stop()
	a := enterWorldAs(t, addr, "tester")
	defer a.Close()
	drainRaw(t, a)

	whisperFrame(t, a, "create", "Knights")
	linha := ""
	for i := 0; i < 8 && linha == ""; i++ {
		ty, p, ok := readMaybe(t, a)
		if !ok {
			break
		}
		if ty == protocol.MsgMessagePanel {
			if len(p) == 0 {
				t.Fatal("o /create mandou MsgMessagePanel sem corpo — o cliente lê além do frame")
			}
			linha = decodePanel(p)
		}
	}
	if !strings.HasPrefix(linha, "Guilda Knights criada! Número da guilda: ") {
		t.Fatalf("mensagem de sucesso = %q, esperado o nome e o número", linha)
	}

	// The same name again, from another character: refused before dbServer, by
	// name, not as a generic failure.
	b := enterWorldAs(t, addr, "tradeb")
	defer b.Close()
	drainRaw(t, b)
	whisperFrame(t, b, "create", "Knights")
	recusa := ""
	for i := 0; i < 8 && recusa == ""; i++ {
		ty, p, ok := readMaybe(t, b)
		if !ok {
			break
		}
		if ty == protocol.MsgMessagePanel {
			recusa = decodePanel(p)
		}
	}
	if recusa != "Já existe uma guilda chamada Knights." {
		t.Errorf("segunda criação com o mesmo nome = %q", recusa)
	}
}
