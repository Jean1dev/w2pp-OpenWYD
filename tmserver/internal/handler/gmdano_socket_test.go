package handler

import (
	"net"
	"strings"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
)

// linhasDeAviso junta as linhas de aviso que chegaram até a conexão secar.
func linhasDeAviso(t *testing.T, c net.Conn) []string {
	t.Helper()
	var out []string
	for range 40 {
		ty, p, ok := readMaybe(t, c)
		if !ok {
			break
		}
		if ty == protocol.MsgMessagePanel {
			out = append(out, decodePanel(p))
		}
	}
	return out
}

// TestGMDanoPeloSocket: o caminho inteiro do comando, como o cliente o manda —
// sussurro para "gm" com "dano [nome]" no corpo —, e responde na linha de aviso
// do servidor, para o próprio GM e para um alvo pelo nome.
func TestGMDanoPeloSocket(t *testing.T) {
	addr, stop, _ := startServerClock(t, gmDB())
	defer stop()
	mod := enterWorldAs(t, addr, "mod")
	defer mod.Close()
	alvo := enterWorldAs(t, addr, "player")
	defer alvo.Close()

	for _, linha := range []string{"dano", "dano Player", "ataque"} {
		gmFrame(t, mod, linha)
		avisos := linhasDeAviso(t, mod)
		if len(avisos) == 0 {
			t.Fatalf("/gm %s: nenhuma resposta chegou ao GM", linha)
		}
		if !strings.HasPrefix(avisos[0], "Ataque de ") {
			t.Errorf("/gm %s: primeira linha = %q, want começando com \"Ataque de \"", linha, avisos[0])
		}
		if !strings.Contains(strings.Join(avisos, "\n"), "Atributos:") {
			t.Errorf("/gm %s: falta a linha dos atributos:\n%s", linha, strings.Join(avisos, "\n"))
		}
	}
}

// TestGMDanoRecusaJogador: o comando passa pelo portão de GM.
func TestGMDanoRecusaJogador(t *testing.T) {
	addr, stop, _ := startServerClock(t, gmDB())
	defer stop()
	c := enterWorldAs(t, addr, "player")
	defer c.Close()
	gmFrame(t, c, "dano")
	if avisos := linhasDeAviso(t, c); len(avisos) != 0 {
		t.Fatalf("um jogador comum recebeu o relatório: %q", avisos)
	}
}

// TestGMDanoNomeDesconhecidoResponde: nome que não está em jogo recebe uma
// resposta em palavras. Silêncio aqui é indistinguível de "o servidor não
// conhece o comando" — foi o que aconteceu em jogo em 2026-09-12.
func TestGMDanoNomeDesconhecidoResponde(t *testing.T) {
	addr, stop, _ := startServerClock(t, gmDB())
	defer stop()
	mod := enterWorldAs(t, addr, "mod")
	defer mod.Close()

	gmFrame(t, mod, "dano Ninguem")
	avisos := linhasDeAviso(t, mod)
	if len(avisos) == 0 || !strings.Contains(avisos[0], "Não achei Ninguem em jogo") {
		t.Fatalf("resposta = %q, want avisando que o jogador não está em jogo", avisos)
	}
}
