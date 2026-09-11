package handler

import (
	"strings"
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/worldevents"
)

// TestGMGuerraTorreAvisaOServidorInteiro: o GM força a guerra e quem não é GM,
// em qualquer lugar, recebe o aviso na linha de aviso do servidor.
func TestGMGuerraTorreAvisaOServidorInteiro(t *testing.T) {
	addr, stop, _ := startServerClock(t, gmDB())
	defer stop()
	mod := enterWorldAs(t, addr, "mod")
	defer mod.Close()
	other := enterWorldAs(t, addr, "player")
	defer other.Close()

	for _, tc := range []struct{ cmd, want string }{
		{"guerra torre aviso 2", "A Guerra de Torres será iniciada em 2 minutos."},
		{"guerra torre fim", "A Guerra de Torres foi cancelada."},
		{"guerra torre abrir 10", "A Guerra de Torres começou!"},
		{"guerra torre fim", "Guerra de Torres finalizada. Nenhuma guilda conquistou a torre."},
	} {
		gmFrame(t, mod, tc.cmd)
		h, p, ok := expectHeader(t, other, protocol.MsgMessagePanel)
		if !ok {
			t.Fatalf("/gm %s: nenhum aviso chegou ao outro jogador", tc.cmd)
		}
		if h.ID != 0 {
			t.Errorf("/gm %s: HEADER.ID = %d, want 0 (aviso do servidor)", tc.cmd, h.ID)
		}
		if got := decodePanel(p); !strings.HasPrefix(got, tc.want) {
			t.Errorf("/gm %s: aviso = %q, want começando com %q", tc.cmd, got, tc.want)
		}
	}
}

// TestGMGuerraTorreRecusaJogador: o comando passa pelo portão de GM.
func TestGMGuerraTorreRecusaJogador(t *testing.T) {
	addr, stop, _ := startServerClock(t, gmDB())
	defer stop()
	c := enterWorldAs(t, addr, "player")
	defer c.Close()
	gmFrame(t, c, "guerra torre abrir")
	if ty, _, ok := readMaybe(t, c); ok && ty == protocol.MsgMessagePanel {
		t.Fatal("um jogador comum abriu a guerra")
	}
}

// TestLembreteDaGuerraAberta: com a guerra aberta, a cada 5 minutos o servidor
// inteiro é lembrado de quanto falta e com quem está a torre.
func TestLembreteDaGuerraAberta(t *testing.T) {
	d, w := torreDeTeste(t, segunda20h.Add(-5*time.Hour))
	w.SetGuildName(77, "Lendas")
	d.applyTowerAction(w, d.events.tower.ForceOpen(d.now(), 24*time.Minute))
	d.events.towerOwner = 77
	inicio := d.events.towerReminder

	d.now = func() time.Time { return inicio.Add(4 * time.Minute) }
	d.tickTowerWar(w)
	if !d.events.towerReminder.Equal(inicio) {
		t.Fatal("lembrete saiu antes de 5 minutos")
	}
	d.now = func() time.Time { return inicio.Add(5 * time.Minute) }
	d.tickTowerWar(w)
	if d.events.towerReminder.Equal(inicio) {
		t.Fatal("aos 5 minutos o lembrete não saiu")
	}
	got := d.towerStatusLine(w, d.now())
	want := "Guerra de Torres em andamento: faltam 19 minutos. Torre com a guilda [Lendas]."
	if got != want {
		t.Errorf("lembrete = %q, want %q", got, want)
	}
	if n := len(protocol.ClientText(got)); n > 94 {
		t.Errorf("lembrete tem %d bytes, a linha de aviso leva 94", n)
	}
	if d.events.tower.Phase() != worldevents.TowerOpen {
		t.Errorf("às 15h a guerra forçada caiu (fase %v)", d.events.tower.Phase())
	}
}
