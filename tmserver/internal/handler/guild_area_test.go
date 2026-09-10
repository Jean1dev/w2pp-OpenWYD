package handler

import (
	"context"
	"io"
	"log/slog"
	"net"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/content"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// mapaDeAtributos loads the shipped AttributeMap.dat, the grid the rule reads.
func mapaDeAtributos(t *testing.T) *content.Grid {
	t.Helper()
	g, err := content.LoadGrid(filepath.Join("..", "..", "..", "Release", "TMsrv", "run", "AttributeMap.dat"), content.AttributeMapDim)
	if err != nil {
		t.Skipf("AttributeMap.dat unavailable: %v", err)
	}
	return g
}

// TestAreaDeGuildEhOBit20DasCidades pins where the rule bites, on the real map:
// the guild garden of Armia (where the report was taken) is in; the square
// beside it, the war fields and the thin 0x20 strips outside every city are out.
func TestAreaDeGuildEhOBit20DasCidades(t *testing.T) {
	d := New(Config{Attributes: mapaDeAtributos(t)})
	dentro := [][2]int16{
		{2088, 2148}, // Armia's guild spawn, the garden in the report
		{2531, 1700}, // Azran's
		{2460, 1976}, // Erion's
		{3614, 3124}, // Nippleheim's
		{1066, 1760}, // Noatum's
	}
	for _, p := range dentro {
		if _, ok := d.guildAreaOwner(p[0], p[1]); !ok {
			t.Errorf("(%d,%d) devia ser área de guild", p[0], p[1])
		}
	}
	fora := [][2]int16{
		{2107, 2144}, // Balmus, just outside the garden
		{2086, 2093}, // Armia's city spawn
		{210, 220},   // Armia's guild WAR field (0x44, no 0x20)
		{300, 230},   // a 0x20 strip outside every city: zone 5 in the legacy
	}
	for _, p := range fora {
		if _, ok := d.guildAreaOwner(p[0], p[1]); ok {
			t.Errorf("(%d,%d) não devia ser área de guild", p[0], p[1])
		}
	}
}

// TestAreaDeGuildQuemFicaQuemSai covers every verdict. The zone without an owner
// is the case from the report: the legacy compared guild 0 with owner 0 and let
// a guildless player in.
func TestAreaDeGuildQuemFicaQuemSai(t *testing.T) {
	d := New(Config{Attributes: mapaDeAtributos(t)})
	jardim := func(guild uint16) *world.Entity { return &world.Entity{X: 2088, Y: 2148, Guild: guild} }

	casos := []struct {
		nome   string
		dono   uint16
		acesso world.AccessLevel
		e      *world.Entity
		sai    bool
		msg    string
	}{
		{"sem dono, sem guild", 0, world.AccessPlayer, jardim(0), true, msgAreaDeGuild},
		{"sem dono, com guild", 0, world.AccessPlayer, jardim(7), true, msgAreaDeGuild},
		{"sem dono, moderador", 0, world.AccessModerator, jardim(0), false, ""},
		{"dono, membro", 5, world.AccessPlayer, jardim(5), false, ""},
		{"dono, outra guild", 5, world.AccessPlayer, jardim(7), true, msgZonaDeOutraGuilda},
		{"dono, sem guild", 5, world.AccessPlayer, jardim(0), true, msgZonaDeOutraGuilda},
		{"dono, admin", 5, world.AccessAdmin, jardim(0), false, ""},
		{"fora da área", 0, world.AccessPlayer, &world.Entity{X: 2107, Y: 2144}, false, ""},
	}
	for _, tc := range casos {
		t.Run(tc.nome, func(t *testing.T) {
			d.guildZones[0].ChargeGuild = tc.dono
			msg, sai := d.guildAreaVerdict(tc.acesso, tc.e)
			if sai != tc.sai || msg != tc.msg {
				t.Errorf("veredito = (%q, %v), esperado (%q, %v)", msg, sai, tc.msg, tc.sai)
			}
		})
	}
}

// TestAreaDeGuildSemMapaNaoFazNada: a server booted without the maps mounted
// has no way to know the ground, and must not guess.
func TestAreaDeGuildSemMapaNaoFazNada(t *testing.T) {
	d := New(Config{})
	if _, sai := d.guildAreaVerdict(world.AccessPlayer, &world.Entity{X: 2088, Y: 2148}); sai {
		t.Error("sem AttributeMap a regra expulsou alguém")
	}
}

// TestAreaDeGuildMandaEmboraNoJogo is the report end to end: a guildless player
// standing in Armia's guild garden is sent out with the reason on the next tick,
// and a moderator on the same ground stays.
func TestAreaDeGuildMandaEmboraNoJogo(t *testing.T) {
	db := &fakeDB{accounts: map[string]*fakeAccount{
		"mod":    {id: 20, pass: "secret", role: "moderator", chars: []world.CharSummary{{Slot: 0, Name: "Mod"}}},
		"player": {id: 22, pass: "secret", role: "player", chars: []world.CharSummary{{Slot: 0, Name: "Player"}}},
	}}
	db.loads = map[int64]world.CharacterState{
		20: {Slot: 0, Name: "Mod", Level: 350, X: 2090, Y: 2150, HP: 1000, MaxHP: 1000},
		22: {Slot: 0, Name: "Player", Level: 350, X: 2088, Y: 2148, HP: 1000, MaxHP: 1000},
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	clock := &atomic.Uint32{}
	clock.Store(serverTime)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := New(Config{Log: log, Now: func() time.Time { return time.Unix(0, 0) }, Attributes: mapaDeAtributos(t)})
	w := world.New(world.Config{GridDim: 4096, Now: clock.Load}, log, db, d.Handle)
	w.SetTickHandler(10*time.Millisecond, func(w *world.World) { clock.Add(100); d.Tick(w) })
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { _ = w.Serve(ctx, ln); close(done) }()
	defer func() {
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("server did not stop")
		}
	}()

	mod := enterWorldAs(t, ln.Addr().String(), "mod")
	defer mod.Close()
	player := enterWorldAs(t, ln.Addr().String(), "player")
	defer player.Close()

	ouviuAviso := func(c net.Conn) bool {
		for i := 0; i < 40; i++ {
			ty, p, ok := readMaybe(t, c)
			if !ok {
				return false
			}
			if ty == protocol.MsgMessagePanel && decodePanel(p) == msgAreaDeGuild {
				return true
			}
		}
		return false
	}
	if !ouviuAviso(player) {
		t.Error("o jogador sem guild no jardim de guild de Armia não foi mandado embora")
	}
	if ouviuAviso(mod) {
		t.Error("o moderador foi mandado embora da área de guild")
	}
}
