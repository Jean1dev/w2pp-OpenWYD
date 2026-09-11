package handler

import (
	"fmt"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// Os "Ajudante" do campo de treino são carbúnculos que dão buff a quem começa.
// No legado é o caso CARBUNCLE_WIND do _MSG_Quest (Merchant 120,
// _MSG_Quest.cpp:109-110 e 2365-2396). São três no NPCGener: #3812 (2130,2035),
// #3813 (2083,2003) e #3814 (2091,1950).
const merchantAjudante = 120

// buffsDoAjudante são as quatro magias que o carbúnculo lança, uma por casa da
// SkillBar dele: Teleporte (41, afeto 2, velocidade), Escudo Mágico (43, afeto
// 11, defesa), Arma Mágica (44, afeto 9, dano) e Toque de Athena (45, afeto 15,
// "aprender skill": soma nas maestrias). O legado só confere se a casa não é 255
// e lança sempre a mesma magia daquela casa, seja qual for o número guardado nela.
// Os três templates trazem 0 nas casas 1-3, então todo Ajudante dá os quatro.
var buffsDoAjudante = [4]int{41, 43, 44, 45}

const (
	// ajudanteTempo e ajudanteNivel são os dois números fixos do
	// `SetAffect(conn, skill, 400, 100)` do legado. Com eles a velocidade dura
	// 64 ticks (~8,5 min) e os outros três 604 ticks (~80 min).
	ajudanteTempo = 400
	ajudanteNivel = 100

	// ajudanteNivelLimite: o legado recusa `CurrentScore.Level >= 100`, ou seja,
	// do nível 101 na tela para cima.
	ajudanteNivelLimite = 100

	msgAjudanteRecusa = "Seu nível não permite o uso disto." // _NN_Level_Limit2 (Language.txt:340)
	msgAjudanteBuff   = "Sente-se mais forte agora %s?"      // _SN_CARBUNCLEMSG (Language.txt:534)
	chaveAjudanteNega = "_NN_Level_Limit2"
)

// ajudanteDoCampo é o clique no carbúnculo: Mortal ou Arch abaixo do nível 101
// recebe os quatro buffs, e o bicho fala.
//
// A duração é a do legado, sem a política de duração do servidor
// (world.AffectDuration zero). Os buffs de magia de jogador passam por essa
// política, que corta a ~10 min. Aqui o pedido foi "os efeitos do NPC iguais aos
// do legado", e os 80 min fazem parte disso.
func (d *Dispatcher) ajudanteDoCampo(w *world.World, s *world.Session, e, npc *world.Entity) {
	if e.ClassMaster != classMasterMortal && e.ClassMaster != classMasterArch {
		d.say(w, npc, chaveAjudanteNega, msgAjudanteRecusa)
		return
	}
	if e.Level >= ajudanteNivelLimite {
		d.say(w, npc, chaveAjudanteNega, msgAjudanteRecusa)
		return
	}
	for casa, magia := range buffsDoAjudante {
		if npc.SkillBar[casa] == skillBarEmpty || d.spells == nil {
			continue
		}
		sp, ok := d.spells.Get(magia)
		if !ok {
			continue
		}
		e.SetAffect(sp.AffectType, sp.AffectValue, sp.AffectTime, sp.Aggressive,
			ajudanteTempo, ajudanteNivel, world.AffectDuration{})
	}
	d.refreshScore(e)
	d.sendScore(w, s, e)
	d.sendAffect(w, s, e)
	sendSay(w, npc, fmt.Sprintf(msgAjudanteBuff, e.Name))
}
