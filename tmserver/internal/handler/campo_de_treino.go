package handler

import (
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// attrCampoDeTreino é o bit do AttributeMap que marca o campo de treino
// (_MSG_Action.cpp:215). No mapa do jogo ele cobre um bloco só, logo ao norte de
// Armia: tiles (2064,1932)-(2159,2051), onde fica o Chefe de Treino.
const attrCampoDeTreino = 0x80

// nivelMaximoDoCampoDeTreino é o último nível que pode ficar no campo. O legado
// expulsa com `CurrentScore.Level >= 35`, e o nível do servidor começa em 0: 34 é
// o nível 35 na tela. É o "level 1 até 35" pedido, e o que a mensagem diz.
const nivelMaximoDoCampoDeTreino = 34

// msgCampoDeTreino é _NN_Newbie_zone (Language.txt:46).
const msgCampoDeTreino = "Somente nível 35 ou inferior pode entrar no campo de treinamento."

// Para onde o legado manda quem sai: Armia, 2091-2093 × 2101-2103
// (_MSG_Action.cpp:219, `DoTeleport(conn, 2091 + rand() % 3, 2101 + rand() % 3)`).
const (
	campoDeTreinoSaidaX = 2091
	campoDeTreinoSaidaY = 2101
)

// Onde nasce o Mortal novo (regra da equipe, 11/09/2026): dentro do campo, ao sul
// do Chefe de Treino (NPCGener, 2121,2040), numa área que o AttributeMap marca
// inteira com o bit do campo — 0x84 de x 2104 a 2128, y 2016 a 2044. O legado
// nascia em Armia, na posição do template BaseMob (2096,2096).
const (
	campoDeTreinoNascimentoX = 2116
	campoDeTreinoNascimentoY = 2030
)

// pontoDeEntrada é onde entra no mundo quem não tem posição guardada: o
// personagem novo no campo de treino, qualquer outro na última cidade.
func pontoDeEntrada(ultimaCidade int16, novo bool) (int16, int16) {
	if novo {
		return campoDeTreinoNascimentoX, campoDeTreinoNascimentoY
	}
	return world.CitySpawn(int(ultimaCidade))
}

// deveSairDoCampoDeTreino diz se e está no campo de treino sem poder: nível
// acima de 35 na tela, ou qualquer Arch ou Celestial, seja qual for o nível
// (`extra.ClassMaster != MORTAL`, _MSG_Action.cpp:215). A equipe fica, pela
// mesma autoridade do /gm, como na área de guild.
func (d *Dispatcher) deveSairDoCampoDeTreino(access world.AccessLevel, e *world.Entity) bool {
	if d.attributes == nil || access >= world.AccessModerator {
		return false
	}
	if d.attributes.At(int(e.X)/4, int(e.Y)/4)&attrCampoDeTreino == 0 {
		return false
	}
	return e.Level > nivelMaximoDoCampoDeTreino || e.ClassMaster != classMasterMortal
}

// guardCampoDeTreino tira do campo de treino quem não pode estar nele e manda
// para Armia com o aviso do legado.
//
// O legado confere só no passo (_MSG_Action.cpp:213-221). Aqui roda a cada tick,
// como a área de guild: pega também quem loga, renasce ou é teleportado lá
// dentro, e quem já estava lá quando a regra entrou.
func (d *Dispatcher) guardCampoDeTreino(w *world.World) {
	if d.attributes == nil {
		return
	}
	w.ForEachPlaying(-1, func(s *world.Session, e *world.Entity) {
		if !d.deveSairDoCampoDeTreino(s.AccessLevel, e) {
			return
		}
		// O destino espera uma entidade viva, como no ClearAreaGuild
		// (Server.cpp:6378) que a área de guild copia.
		if e.HP <= 0 {
			e.HP = 2
			d.sendScore(w, s, e)
		}
		x := int16(campoDeTreinoSaidaX + w.Rand().Intn(3))
		y := int16(campoDeTreinoSaidaY + w.Rand().Intn(3))
		// SetEntityPos apaga do grid quem estiver na casa de destino.
		if fx, fy, ok := d.freeCellAtOrNear(w, x, y); ok {
			x, y = fx, fy
		}
		d.log.Info("campo de treino: expulso", "conn", s.Conn, "account", s.AccountName,
			"level", e.Level, "class_master", e.ClassMaster, "x", e.X, "y", e.Y)
		sendClientMessage(w, s, msgCampoDeTreino)
		d.doTeleport(w, s, x, y)
	})
}
