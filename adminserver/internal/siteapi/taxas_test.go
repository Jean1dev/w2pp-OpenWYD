package siteapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/internal/domain"
)

func comTaxas(c *cenario, cfg domain.DropBonusConfig) {
	c.banco.mu.Lock()
	defer c.banco.mu.Unlock()
	c.banco.drop = cfg
}

func leTaxas(t *testing.T, c *cenario) respostaTaxas {
	t.Helper()
	rec := c.pede("GET", "/site/v1/taxas", "")
	confereStatus(t, rec, http.StatusOK, "")
	var r respostaTaxas
	if err := json.Unmarshal(rec.Body.Bytes(), &r); err != nil {
		t.Fatalf("corpo não é JSON: %v: %s", err, rec.Body.String())
	}
	return r
}

func chances(fatias []fatiaTaxa) []int32 {
	out := make([]int32, 0, len(fatias))
	for _, f := range fatias {
		out = append(out, f.Chance)
	}
	return out
}

func mesmasChances(a []int32, b ...int32) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// §: faixa que ninguém editou sai com a escada do legado, que é a que o jogo
// sorteia. Os números são os mesmos que o teste da tela do painel afirma.
func TestTaxasUsaOPadraoQuandoNinguemEditou(t *testing.T) {
	c := novoCenario(t)
	comTaxas(c, domain.DropBonusConfig{Ligado: true})

	r := leTaxas(t, c)
	if !r.Drop.Ligado {
		t.Error("o interruptor tinha de vir ligado")
	}
	if len(r.Drop.Faixas) != 4 {
		t.Fatalf("vieram %d faixas, queria 4", len(r.Drop.Faixas))
	}
	primeira := r.Drop.Faixas[0]
	if primeira.Chave != "mesmo_nivel" || primeira.Rotulo != "Mesmo nível" {
		t.Errorf("a primeira faixa não usa o rótulo do painel: %+v", primeira)
	}
	if primeira.Editada {
		t.Error("faixa não editada não pode vir marcada como editada")
	}
	// Refino do legado na faixa 0: limiares 6/22/75/90 viram 6, 16, 53, 15, 10.
	if got := chances(primeira.Refino); !mesmasChances(got, 6, 16, 53, 15, 10) {
		t.Errorf("chances do refino = %v, queria [6 16 53 15 10]", got)
	}
	if primeira.Refino[0].Rotulo != "Refino +2" || primeira.Refino[4].Rotulo != "Nada" {
		t.Errorf("os rótulos do refino não são os do painel: %+v", primeira.Refino)
	}
	// Magnitude do legado na faixa 0: 45% de sair o degrau 0, que é por que
	// tanto drop de mesmo nível sai vazio.
	mag := primeira.Magnitude
	if got := chances(mag); !mesmasChances(got, 2, 4, 18, 31, 45) {
		t.Errorf("chances da magnitude = %v, queria [2 4 18 31 45]", got)
	}
	if mag[0].Rotulo != "melhor" || mag[0].Valor != "degrau 4" || mag[4].Valor != "degrau 0" {
		t.Errorf("a magnitude não descreve os degraus como o painel: %+v", mag)
	}
}

// §: limiar que não avança apaga o resultado, e isso é dito em vez de escondido.
// O legado usa 100 no último limiar de propósito, para a faixa nunca "não dar
// nada" — é o caso da faixa 2.
func TestTaxasMarcaResultadoInalcancavel(t *testing.T) {
	c := novoCenario(t)
	comTaxas(c, domain.DropBonusConfig{Ligado: true})

	r := leTaxas(t, c)
	terceira := r.Drop.Faixas[2] // "49 a 73 acima": refino 6/35/85/100
	nada := terceira.Refino[4]
	if nada.Rotulo != "Nada" || nada.Chance != 0 || !nada.Inativa {
		t.Errorf("o resultado \"Nada\" tinha de vir com chance 0 e marcado como inativo: %+v", nada)
	}
}

// §: faixa editada vence o padrão, e a resposta diz que foi editada.
func TestTaxasUsaAFaixaEditada(t *testing.T) {
	c := novoCenario(t)
	comTaxas(c, domain.DropBonusConfig{
		Ligado: true,
		Faixas: []domain.DropBonusBand{{
			Distancia: 0,
			Limite:    [4]int32{10, 20, 30, 40},
			Degrau:    [5]int32{9, 8, 7, 6, 5},
			Refino:    [4]int32{20, 40, 60, 80},
		}},
	})

	r := leTaxas(t, c)
	primeira := r.Drop.Faixas[0]
	if !primeira.Editada {
		t.Error("a faixa editada tinha de vir marcada")
	}
	if got := chances(primeira.Refino); !mesmasChances(got, 20, 20, 20, 20, 20) {
		t.Errorf("chances do refino editado = %v, queria [20 20 20 20 20]", got)
	}
	// As outras três continuam no padrão.
	if r.Drop.Faixas[1].Editada || r.Drop.Faixas[2].Editada || r.Drop.Faixas[3].Editada {
		t.Error("só a faixa 0 foi editada")
	}
}

// §: a XP entra como ESTADO, com o rótulo do painel, e NENHUM multiplicador.
//
// Não existe taxa de XP exibida: a Mesa de XP precifica uma morte a partir de um
// formulário (monstro, zona, nível), então qualquer número seria escolha minha.
// E o efetivo de 42,5% da migração 0039 é comentário de migração, não tela.
func TestTaxasTrazXPComoEstadoESemMultiplicador(t *testing.T) {
	c := novoCenario(t)
	comTaxas(c, domain.DropBonusConfig{Ligado: true})
	cfg := chuvaCaindo()
	cfg.DoubleExpEnabled = false
	cfg.NewbieEventEnabled = false
	cfg.KefraLiveEnabled = false
	comEventos(c, cfg)

	r := leTaxas(t, c)
	quer := []linhaEvento{
		{"xp_em_dobro", "XP em dobro", "desligado", false},
		{"evento_de_novato", "Evento de novato", "desligado", false},
		{"metade_da_xp", "Metade da XP", "sim", true},
	}
	if len(r.XP) != len(quer) {
		t.Fatalf("vieram %d linhas de XP, queria %d: %+v", len(r.XP), len(quer), r.XP)
	}
	for i, q := range quer {
		if r.XP[i] != q {
			t.Errorf("linha %d = %+v, queria %+v", i, r.XP[i], q)
		}
	}

	rec := c.pede("GET", "/site/v1/taxas", "")
	confereStatus(t, rec, http.StatusOK, "")
	corpo := rec.Body.String()
	for _, proibido := range []string{"42,5", "42.5", "multiplicador", "efetivo", "kefra"} {
		if strings.Contains(strings.ToLower(corpo), proibido) {
			t.Errorf("a resposta trouxe %q, que é número interpretado: %s", proibido, corpo)
		}
	}
}

// §: interruptor desligado é dito, não escondido.
func TestTaxasDizQuandoOBonusEstaDesligado(t *testing.T) {
	c := novoCenario(t)
	comTaxas(c, domain.DropBonusConfig{Ligado: false})

	if r := leTaxas(t, c); r.Drop.Ligado {
		t.Error("com o bônus desligado, a resposta tem de dizer que está desligado")
	}
}
