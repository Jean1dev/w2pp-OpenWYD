package siteapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/internal/domain"
)

// chuvaCaindo is a config where the item rain really drops: every one of the
// five conditions the game gates on is satisfied.
func chuvaCaindo() domain.WorldEventConfig {
	return domain.WorldEventConfig{
		Enabled: true, ItemIndex: 412, Rate: 100,
		StartIndex: 1, CurrentIndex: 1, EndIndex: 10,
	}
}

func comEventos(c *cenario, cfg domain.WorldEventConfig) {
	c.banco.mu.Lock()
	defer c.banco.mu.Unlock()
	c.banco.eventosJogo = cfg
}

func leEventos(t *testing.T, c *cenario) []linhaEvento {
	t.Helper()
	rec := c.pede("GET", "/site/v1/eventos", "")
	confereStatus(t, rec, http.StatusOK, "")
	var r struct {
		Eventos []linhaEvento `json:"eventos"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &r); err != nil {
		t.Fatalf("corpo não é JSON: %v: %s", err, rec.Body.String())
	}
	return r.Eventos
}

func acha(t *testing.T, linhas []linhaEvento, chave string) linhaEvento {
	t.Helper()
	for _, l := range linhas {
		if l.Chave == chave {
			return l
		}
	}
	t.Fatalf("não veio o evento %q: %+v", chave, linhas)
	return linhaEvento{}
}

// §: os rótulos e os estados são os da tela do painel, palavra por palavra.
func TestEventosUsaOsRotulosDoPainel(t *testing.T) {
	c := novoCenario(t)
	cfg := chuvaCaindo()
	cfg.DoubleExpEnabled = true
	cfg.NewbieEventEnabled = false
	cfg.KefraLiveEnabled = false // o painel mostra isto como "Metade da XP: sim"
	cfg.TowerWarEnabled = true
	cfg.TowerWarHour = 23
	cfg.BossRespawnHours = 12
	comEventos(c, cfg)

	linhas := leEventos(t, c)
	quer := []linhaEvento{
		{"xp_em_dobro", "XP em dobro", "ligado", true},
		{"evento_de_novato", "Evento de novato", "desligado", false},
		{"metade_da_xp", "Metade da XP", "sim", true},
		{"guerra_de_torres", "Guerra de Torres", "todo dia às 23h", true},
		{"chefes_sozinhos", "Chefes sozinhos", "voltam em 12h", true},
		{"chuva_de_item", "Chuva de item", "caindo", true},
	}
	if len(linhas) != len(quer) {
		t.Fatalf("vieram %d eventos, queria %d: %+v", len(linhas), len(quer), linhas)
	}
	for i, q := range quer {
		if linhas[i] != q {
			t.Errorf("evento %d = %+v, queria %+v", i, linhas[i], q)
		}
	}
}

// §: KefraLive LIGADO significa que a XP NÃO está pela metade. O painel inverte
// este campo na tela, e é a inversão que engana quem lê o banco direto.
func TestEventosInverteAMetadeDaXP(t *testing.T) {
	c := novoCenario(t)
	cfg := chuvaCaindo()
	cfg.KefraLiveEnabled = true
	comEventos(c, cfg)

	l := acha(t, leEventos(t, c), "metade_da_xp")
	if l.Estado != "não" || l.Ligado {
		t.Errorf("com KefraLive ligado, a metade da XP tem de ser \"não\" e desligada: %+v", l)
	}
}

// §: a tabela-verdade da chuva de item, congelada.
//
// Esta regra é uma CÓPIA de panel.motivoParado/panel.caindo (as funções lá não
// são exportadas). Este teste existe para as duas não divergirem em silêncio: se
// o jogo mudar as condições, é aqui que a cópia aparece errada.
func TestEventosChuvaSegueAMesmaRegraDoPainel(t *testing.T) {
	casos := []struct {
		nome   string
		muda   func(*domain.WorldEventConfig)
		estado string
	}{
		{"tudo pronto", func(*domain.WorldEventConfig) {}, "caindo"},
		{"desligada", func(c *domain.WorldEventConfig) { c.Enabled = false }, "desligada"},
		{"sem item", func(c *domain.WorldEventConfig) { c.ItemIndex = 0 }, "parada"},
		{"item acima do que o pacote carrega", func(c *domain.WorldEventConfig) { c.ItemIndex = 32768 }, "parada"},
		{"chance zero", func(c *domain.WorldEventConfig) { c.Rate = 0 }, "parada"},
		{"numeração começa em zero", func(c *domain.WorldEventConfig) { c.StartIndex = 0 }, "parada"},
		{"atual antes do primeiro", func(c *domain.WorldEventConfig) { c.StartIndex = 5; c.CurrentIndex = 4 }, "parada"},
		{"tiragem acabou", func(c *domain.WorldEventConfig) { c.CurrentIndex = 10; c.EndIndex = 10 }, "parada"},
	}
	for _, caso := range casos {
		t.Run(caso.nome, func(t *testing.T) {
			c := novoCenario(t)
			cfg := chuvaCaindo()
			caso.muda(&cfg)
			comEventos(c, cfg)

			l := acha(t, leEventos(t, c), "chuva_de_item")
			if l.Estado != caso.estado {
				t.Errorf("estado = %q, queria %q", l.Estado, caso.estado)
			}
			if querLigado := caso.estado == "caindo"; l.Ligado != querLigado {
				t.Errorf("ligado = %v, queria %v", l.Ligado, querLigado)
			}
		})
	}
}

// §: a fila da equipe não vai para uma página pública.
func TestEventosNaoPublicaNumeroDeOperacao(t *testing.T) {
	c := novoCenario(t)
	cfg := chuvaCaindo()
	cfg.CurrentIndex, cfg.EndIndex = 3, 700 // 697 por entregar
	comEventos(c, cfg)

	rec := c.pede("GET", "/site/v1/eventos", "")
	confereStatus(t, rec, http.StatusOK, "")
	corpo := rec.Body.String()
	// "Ainda por entregar", o item escolhido, a numeração da tiragem e o aviso
	// são coisas da tela da equipe.
	for _, proibido := range []string{"restam", "697", "atual", "item_index", "anunciar", "conta"} {
		if strings.Contains(corpo, proibido) {
			t.Errorf("a resposta pública vazou %q: %s", proibido, corpo)
		}
	}
}
