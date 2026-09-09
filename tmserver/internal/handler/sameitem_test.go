package handler

import (
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// O caso exato do relato, lido do log de produção:
//
//	celula=2 slot=16 pacote_index=1774 pacote_efeitos=[61 1 0 0 0 0]
//	                 bolsa_index=1774  bolsa_efeitos=[0 0 0 0 0 0]
//
// O cliente descreve a Pedra do Sábio com EF_AMOUNT 1; a bolsa a guarda sem
// efeito de quantidade. As duas dizem "uma pedra", e a comparação byte a byte
// dizia que eram itens diferentes — que era, no fim, o motivo de a máquina +10
// não fazer nada.
func TestSameItemAceitaQuantidadeAusenteComoUm(t *testing.T) {
	bolsa := world.Item{Index: 1774} // sem efeitos, como o servidor guarda

	var pacote protocol.WireItem
	pacote.Index = 1774
	pacote.Effects[0] = protocol.WireEffect{Effect: efAmount, Value: 1}

	if !sameItem(pacote, bolsa) {
		t.Error("EF_AMOUNT 1 no pacote e ausente na bolsa foram tratados como itens diferentes")
	}
	// E o inverso, que é o mesmo par visto do outro lado.
	bolsaComAmount := world.Item{Index: 1774}
	bolsaComAmount.Effects[0] = world.Effect{Effect: efAmount, Value: 1}
	var pacoteSemAmount protocol.WireItem
	pacoteSemAmount.Index = 1774
	if !sameItem(pacoteSemAmount, bolsaComAmount) {
		t.Error("ausente no pacote e EF_AMOUNT 1 na bolsa foram tratados como diferentes")
	}
}

// A tolerância é sobre a GRAFIA da quantidade, não sobre a quantidade. Cinco
// pedras não são uma pedra, e confundir as duas abriria caminho para trocar uma
// pilha por uma unidade.
func TestSameItemAindaComparaAQuantidade(t *testing.T) {
	bolsa := world.Item{Index: 1774}
	bolsa.Effects[0] = world.Effect{Effect: efAmount, Value: 5}

	var pacote protocol.WireItem
	pacote.Index = 1774
	pacote.Effects[0] = protocol.WireEffect{Effect: efAmount, Value: 1}

	if sameItem(pacote, bolsa) {
		t.Error("uma unidade passou por uma pilha de cinco")
	}
}

// O resto continua byte-exato: é a checagem anti-dup que protege a troca de um
// item trocado entre a oferta e a confirmação.
func TestSameItemContinuaExatoNoResto(t *testing.T) {
	bolsa := world.Item{Index: 2455}
	bolsa.Effects[0] = world.Effect{Effect: efSanc, Value: 9}

	casos := []struct {
		nome  string
		monta func() protocol.WireItem
		igual bool
	}{
		{"mesmo item", func() protocol.WireItem {
			var w protocol.WireItem
			w.Index = 2455
			w.Effects[0] = protocol.WireEffect{Effect: efSanc, Value: 9}
			return w
		}, true},
		{"outro índice", func() protocol.WireItem {
			var w protocol.WireItem
			w.Index = 2456
			w.Effects[0] = protocol.WireEffect{Effect: efSanc, Value: 9}
			return w
		}, false},
		{"refino diferente", func() protocol.WireItem {
			var w protocol.WireItem
			w.Index = 2455
			w.Effects[0] = protocol.WireEffect{Effect: efSanc, Value: 8}
			return w
		}, false},
		{"efeito a mais", func() protocol.WireItem {
			var w protocol.WireItem
			w.Index = 2455
			w.Effects[0] = protocol.WireEffect{Effect: efSanc, Value: 9}
			w.Effects[1] = protocol.WireEffect{Effect: efDamage, Value: 20}
			return w
		}, false},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			if got := sameItem(c.monta(), bolsa); got != c.igual {
				t.Errorf("sameItem = %v, esperado %v", got, c.igual)
			}
		})
	}
}

// A ordem dos efeitos não conta, contanto que o conjunto seja o mesmo: o cliente
// insere a quantidade no primeiro espaço livre, que nem sempre é o mesmo do
// servidor.
func TestSameItemIgnoraAOrdemDosEfeitos(t *testing.T) {
	bolsa := world.Item{Index: 1774}
	bolsa.Effects[1] = world.Effect{Effect: efAmount, Value: 3}

	var pacote protocol.WireItem
	pacote.Index = 1774
	pacote.Effects[0] = protocol.WireEffect{Effect: efAmount, Value: 3}

	if !sameItem(pacote, bolsa) {
		t.Error("a mesma quantidade em posições diferentes virou item diferente")
	}
}
