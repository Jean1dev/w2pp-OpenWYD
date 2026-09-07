package handler

import (
	"log/slog"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// TestCatalisadorGrupoCobreATabelaDoLegado percorre a tabela inteira de
// _MSG_UseItem.cpp:5039-5056, cria por cria.
//
// Vale escrever por extenso porque os grupos NÃO são contíguos: 2340 e 2345
// pertencem ao mesmo catalisador com quatro linhagens de outro grupo no meio.
// Um laço que derivasse o grupo por aritmética estaria testando a mesma conta
// que o código faz, e concordaria com ele mesmo se os dois estivessem errados.
func TestCatalisadorGrupoCobreATabelaDoLegado(t *testing.T) {
	esperado := map[int16]int{
		2333: 0, 2334: 0, 2335: 0,
		2336: 1, 2337: 1, 2338: 1, 2339: 1,
		2341: 1, 2342: 1, 2343: 1, 2344: 1,
		2340: 2, 2345: 2, 2357: 2,
		2346: 3, 2347: 3, 2348: 3,
		2351: 4, 2352: 4, 2353: 4, 2358: 4,
		2354: 5, 2355: 5, 2356: 5,
		2349: 6, 2350: 6,
	}
	for cria, grupo := range esperado {
		got, ok := catalisadorGrupo(cria)
		if !ok || got != grupo {
			t.Errorf("cria %d: grupo %d (ok=%v), want %d", cria, got, ok, grupo)
		}
	}

	// Fora da tabela: as três primeiras crias, que sobem por limiar de nível e
	// não têm catalisador, e a última linha, que o legado deixou de fora.
	for _, cria := range []int16{2330, 2331, 2332, 2359, 2360, 0} {
		if _, ok := catalisadorGrupo(cria); ok {
			t.Errorf("cria %d ganhou um grupo e não devia ter", cria)
		}
	}
}

// catalisadorFixture veste a cria e põe o catalisador na mochila.
func catalisadorFixture(t *testing.T, cria, catalisador int16, nivel uint8) (*Dispatcher, *world.World, *world.Session, *world.Entity) {
	t.Helper()
	log := slog.New(slog.DiscardHandler)
	d := New(Config{Log: log})
	w := world.New(world.Config{GridDim: 16}, log, nil, d.Handle)
	e := &world.Entity{
		ID: 0, Mode: world.MobUser, Name: "Heroi", Level: 100,
		BaseMaxHP: 1000, MaxHP: 1000, HP: 1000,
	}
	m := world.Item{Index: cria}
	putShort(&m.Effects[0], 20000)
	m.Effects[1].Effect = nivel
	e.Equip[mountEquipSlot] = m
	e.Carry[0] = world.Item{Index: catalisador}
	return d, w, &world.Session{Conn: 0, Mode: world.UserPlay}, e
}

func corpoNoSlotDaMontaria() protocol.MsgUseItemBody {
	return protocol.MsgUseItemBody{
		SourType: world.ItemPlaceCarry, SourPos: 0,
		DestType: 0, DestPos: mountEquipSlot,
	}
}

func TestCatalisadorCertoCriaAMontariaAdulta(t *testing.T) {
	// Andaluz N: cria 2340 pertence ao grupo 2, o Catalisador de Mencar (3346).
	d, w, s, e := catalisadorFixture(t, 2340, 3346, 40)
	d.useCatalisador(w, s, e, corpoNoSlotDaMontaria(), 0)

	mount := e.Equip[mountEquipSlot]
	if mount.Index != 2370 {
		t.Fatalf("montaria = %d, want 2370 (a cria +30)", mount.Index)
	}
	// O nível zera: a adulta começa a própria subida, qualquer que tenha sido a
	// da cria (_MSG_UseItem.cpp:5071).
	if mount.Effects[1].Effect != 0 {
		t.Errorf("nível = %d, want 0", mount.Effects[1].Effect)
	}
	// E a vitalidade herda o nível da cria com uma rolagem por cima.
	if v := mount.Effects[1].Value; v < 40 || v > 59 {
		t.Errorf("vitalidade = %d, want entre 40 e 59 (nível 40 + rand%%20)", v)
	}
	if !e.Carry[0].Empty() {
		t.Errorf("o catalisador não foi consumido: %+v", e.Carry[0])
	}
}

func TestCatalisadorErradoNaoCriaNada(t *testing.T) {
	// 2340 é do grupo 2; 3344 é o do grupo 0. A recusa tem de deixar tudo como
	// estava — este item custa caro e o erro é fácil de cometer.
	d, w, s, e := catalisadorFixture(t, 2340, 3344, 40)
	d.useCatalisador(w, s, e, corpoNoSlotDaMontaria(), 0)

	if idx := e.Equip[mountEquipSlot].Index; idx != 2340 {
		t.Errorf("montaria = %d, want 2340 intacta", idx)
	}
	if e.Carry[0].Index != 3344 {
		t.Errorf("o catalisador errado foi consumido: %+v", e.Carry[0])
	}
}

func TestCatalisadorNaoMexeEmMontariaAdulta(t *testing.T) {
	// Uma adulta no slot não tem grupo, então nenhum catalisador serve. Sem esta
	// guarda o +30 levaria a montaria para 2400, que é a faixa dos âmagos.
	d, w, s, e := catalisadorFixture(t, 2370, 3346, 40)
	d.useCatalisador(w, s, e, corpoNoSlotDaMontaria(), 0)

	if idx := e.Equip[mountEquipSlot].Index; idx != 2370 {
		t.Errorf("montaria = %d, want 2370 intacta", idx)
	}
	if e.Carry[0].Empty() {
		t.Error("consumiu o catalisador numa montaria que já é adulta")
	}
}

func TestCatalisadorExigeOSlotDaMontaria(t *testing.T) {
	// O cliente precisa apontar o slot 14 como destino, como no âmago. Sem isso
	// não dá para saber em que a pessoa quis usar.
	d, w, s, e := catalisadorFixture(t, 2340, 3346, 40)
	body := corpoNoSlotDaMontaria()
	body.DestPos = 3
	d.useCatalisador(w, s, e, body, 0)

	if idx := e.Equip[mountEquipSlot].Index; idx != 2340 {
		t.Errorf("montaria = %d, want 2340 intacta", idx)
	}
	if e.Carry[0].Empty() {
		t.Error("consumiu o catalisador apontado para o slot errado")
	}
}

func TestCatalisadorSemMontariaNoSlot(t *testing.T) {
	d, w, s, e := catalisadorFixture(t, 0, 3346, 0)
	e.Equip[mountEquipSlot] = world.Item{}
	d.useCatalisador(w, s, e, corpoNoSlotDaMontaria(), 0)

	if !e.Equip[mountEquipSlot].Empty() {
		t.Errorf("apareceu montaria do nada: %+v", e.Equip[mountEquipSlot])
	}
	if e.Carry[0].Empty() {
		t.Error("consumiu o catalisador com o slot vazio")
	}
}
