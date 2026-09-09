package handler

import (
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// TestColeiraDoPetPassaDoAlcanceDeVista fixa a aritmética que fabrica criaturas
// fantasma, e é a razão de removeMobParaODono existir.
//
// DespawnMob avisa da saída de um mob apenas quem está EM VISTA dele:
// ForEachInViewAt filtra por chebyshev <= ViewRange (world/api.go). Mas a coleira
// do pet é MAIOR que esse alcance — ele pode andar summonLeash tiles do dono. Na
// faixa entre um e outro o pet é apagado no servidor sem que o dono receba nada,
// e o cliente, que indexa entidades por id e nunca as descarta sozinho, continua
// desenhando aquele bicho para sempre.
//
// O efeito só aparece com o tempo: cada re-lançamento deixa para trás os que
// tinham se afastado, e a conta cresce. Doze lugares na lista de grupo, vinte
// gorilas na tela.
//
// Enquanto esta desigualdade valer, a notificação direta ao dono é obrigatória.
// Se um dia a coleira encolher para dentro do alcance de vista, este teste passa
// a falhar e aí sim removeMobParaODono vira redundante — de propósito: é o aviso
// de que a premissa mudou.
func TestColeiraDoPetPassaDoAlcanceDeVista(t *testing.T) {
	if summonLeash <= world.ViewRange {
		t.Fatalf("summonLeash=%d já cabe em ViewRange=%d; a premissa de removeMobParaODono mudou "+
			"e o comentário dela precisa ser revisto", summonLeash, world.ViewRange)
	}
	t.Logf("faixa cega: pet entre %d e %d tiles do dono sai sem que o DespawnMob o avise",
		world.ViewRange+1, summonLeash)
}

