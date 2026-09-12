package handler

import (
	"context"

	"github.com/jeanluca/w2pp-openwyd/internal/level"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// subCelestialLevelReq e o nivel minimo para nascer um Sub. O 120 nao e numero
// solto: e onde a escada de pontos do Celestial ganha o primeiro degrau
// (celestialLevelSteps, +6 por nivel a partir do 120) e onde a armadura marca o
// primeiro marco. Nascer antes disso daria um Sub que ainda nao pagou nada.
const subCelestialLevelReq = 120

// criaSubCelestial e o ramo da Pedra Ideal que NASCE o Sub, em vez de virar
// Celestial. Devolve true quando tratou o uso — e quem trata o uso responde ao
// jogador, com recusa explicada.
//
// So engata em Celestial puro (ClassMaster 3). Um Arch cai fora daqui e segue
// para o caminho de sempre; um CS ja tem Sub e e recusado com essa razao.
func (d *Dispatcher) criaSubCelestial(w *world.World, s *world.Session, e *world.Entity, src int) bool {
	if e.ClassMaster == classMasterCelestialCS || e.ClassMaster == classMasterSCelestial {
		d.log.Info("sub celestial refused: already has one",
			"conn", s.Conn, "account", s.AccountName, "classmaster", e.ClassMaster)
		d.notify(w, s, NoticeReqNotMet)
		sendClientMessage(w, s, "Voce ja tem um Sub Celestial.")
		d.sendSlot(w, s, world.ItemPlaceCarry, src, e.Carry[src])
		return true
	}
	if e.ClassMaster != classMasterCelestial {
		return false // Arch: segue para o Arch->Celestial de sempre
	}
	sephirot := int(e.Equip[sephirotEquipSlot].Index)
	if e.Level < subCelestialLevelReq || sephirot < archSephirotMin || sephirot > archSephirotMax {
		d.log.Info("sub celestial refused",
			"conn", s.Conn, "account", s.AccountName, "level", e.Level, "sephirot", sephirot)
		d.notify(w, s, NoticeReqNotMet)
		if e.Level < subCelestialLevelReq {
			sendClientMessage(w, s, "O Sub Celestial exige nivel 120.")
		} else {
			sendClientMessage(w, s, "Use o Sephirot no espaco 11 para criar o Sub Celestial.")
		}
		d.sendSlot(w, s, world.ItemPlaceCarry, src, e.Carry[src])
		return true
	}
	// TUDO FORA menos o Sephirot, pela mesma razao da subida a Celestial: a vida
	// nova nasce no nivel 1 e as pocas dela saem da formula, nao do conjunto. Um
	// Sub nascido vestido de Celestial 120 carregaria pocas de um corpo que ele
	// nao tem. Recusar em vez de apagar: o conjunto vale meses.
	for i := 1; i < world.MaxEquip; i++ {
		if i == sephirotEquipSlot || e.Equip[i].Empty() {
			continue
		}
		d.log.Info("sub celestial refused: gear equipped",
			"conn", s.Conn, "account", s.AccountName, "slot", i, "item", e.Equip[i].Index)
		d.notify(w, s, NoticeReqNotMet)
		sendClientMessage(w, s, msgIdealStoneUnequipAll)
		d.sendSlot(w, s, world.ItemPlaceCarry, src, e.Carry[src])
		return true
	}

	principal, err := codificaVida(guardaVida(s, e))
	if err != nil {
		d.log.Warn("sub celestial: encode failed", "conn", s.Conn, "name", e.Name, "err", err)
		d.notify(w, s, NoticeDBError)
		d.sendSlot(w, s, world.ItemPlaceCarry, src, e.Carry[src])
		return true
	}
	staged := *e
	d.buildSubCelestialSnapshot(&staged, src, principal)
	barraQueSai := s.ShortSkill
	s.ShortSkill = [16]uint8{}
	d.persisteTrocaDeVidaComBarra(w, s, e, src, staged, "sub celestial created", barraQueSai)
	return true
}

// buildSubCelestialSnapshot deixa o personagem como o Sub recem-nascido: nivel 1
// na evolucao CS, com a vida principal guardada.
//
// O que NAO e zerado, e por que: bolsa, ouro, fama e cla sao do PERSONAGEM, nao
// da vida. Quem cria um Sub nao esta criando outro jogador.
func (d *Dispatcher) buildSubCelestialSnapshot(e *world.Entity, src int, principal string) {
	guardadoNivel := e.Level

	e.ClassMaster = classMasterCelestialCS
	e.SubCelestialGuardada = principal
	e.SubCelestialLevel = uint16(guardadoNivel)
	e.SubCelestialAtivo = 1

	e.Level, e.Exp = 1, 0
	e.ResetAffects()
	b := level.BaseAttributes(e.Class)
	e.BaseStr, e.BaseInt, e.BaseDex, e.BaseCon = int16(b[0]), int16(b[1]), int16(b[2]), int16(b[3])
	e.Str, e.Int, e.Dex, e.Con = e.BaseStr, e.BaseInt, e.BaseDex, e.BaseCon
	e.BaseSpecial, e.Special = [4]int16{}, [4]int16{}
	e.ScoreBonus, e.SkillBonus, e.SpecialBonus = 0, 0, 0
	e.LearnedSkill, e.SecLearnedSkill = 1<<30, 0
	e.SkillBar = [4]uint8{}
	e.CelLv40, e.CelLv90, e.CelCircle = 0, 0, 0
	e.BaseMaxHP, e.BaseMaxMP = level.BasePools(e.Class, e.ClassMaster, e.Level, int32(e.BaseCon), int32(e.BaseInt))
	e.HP, e.MaxHP, e.MP, e.MaxMP = e.BaseMaxHP, e.BaseMaxHP, e.BaseMaxMP, e.BaseMaxMP

	// A barra de atalho do Sub nasce limpa junto com as habilidades: manter a do
	// Celestial deixaria todo botao apontando para magia que o Sub nao tem.
	// Quem chama zera a da Sessao — ver criaSubCelestial.

	// O Sephirot e gasto pelo nascimento, como na fusao do Rei.
	e.Equip[sephirotEquipSlot] = world.Item{}
	consumeOneItem(&e.Carry[src])
	d.refreshScore(e)
}

// usePedraMisteriosa troca a vida ativa pela guardada. Era recusa explicita
// (rejectUnimplementedConsumable) ate a migracao 0057 dar onde guardar.
func (d *Dispatcher) usePedraMisteriosa(w *world.World, s *world.Session, e *world.Entity, src int) {
	if e.ClassMaster != classMasterCelestialCS && e.ClassMaster != classMasterSCelestial {
		d.notify(w, s, NoticeReqNotMet)
		sendClientMessage(w, s, "A Pedra Misteriosa so serve para quem tem Sub Celestial.")
		d.sendSlot(w, s, world.ItemPlaceCarry, src, e.Carry[src])
		return
	}
	guardada, err := decodificaVida(e.SubCelestialGuardada)
	if err != nil {
		// Chegar aqui e estado impossivel pela trava do banco (ativo = 0 OU
		// guardada NAO NULA). Se chegou, e defeito nosso, e recusar e o unico
		// caminho honesto: trocar para uma vida que nao existe apagaria a que
		// existe.
		d.log.Warn("pedra misteriosa: no stored life",
			"conn", s.Conn, "name", e.Name, "classmaster", e.ClassMaster, "err", err)
		d.notify(w, s, NoticeReqNotMet)
		sendClientMessage(w, s, "Nao ha vida guardada para trocar.")
		d.sendSlot(w, s, world.ItemPlaceCarry, src, e.Carry[src])
		return
	}
	// Mesmo motivo da criacao: as pocas da vida que entra saem da formula, e o
	// conjunto vestido e o da vida que sai.
	if slot, ok := firstEquippedSlot(e); ok {
		d.log.Info("pedra misteriosa refused: gear equipped",
			"conn", s.Conn, "account", s.AccountName, "slot", slot)
		d.notify(w, s, NoticeReqNotMet)
		sendClientMessage(w, s, msgIdealStoneUnequipAll)
		d.sendSlot(w, s, world.ItemPlaceCarry, src, e.Carry[src])
		return
	}

	saindo, err := codificaVida(guardaVida(s, e))
	if err != nil {
		d.log.Warn("pedra misteriosa: encode failed", "conn", s.Conn, "name", e.Name, "err", err)
		d.notify(w, s, NoticeDBError)
		d.sendSlot(w, s, world.ItemPlaceCarry, src, e.Carry[src])
		return
	}
	staged := *e
	nivelQueSai := staged.Level
	// A barra da sessao passa a ser a da vida que ENTRA. Se a gravacao falhar,
	// persisteTrocaDeVida devolve a de antes: barra apontando para magia que o
	// personagem nao tem faz o servidor contar peso de trapaca (combat.go).
	barraQueSai := s.ShortSkill
	restauraVida(s, &staged, guardada)
	staged.SubCelestialGuardada = saindo
	staged.SubCelestialLevel = uint16(nivelQueSai)
	staged.SubCelestialAtivo = 1 - staged.SubCelestialAtivo
	staged.ResetAffects()
	consumeOneItem(&staged.Carry[src])
	d.refreshScore(&staged)
	d.persisteTrocaDeVidaComBarra(w, s, e, src, staged, "sub celestial swapped", barraQueSai)
}

// persisteTrocaDeVidaComBarra grava a vida nova e devolve o jogador para a
// selecao de personagem — o mesmo caminho da subida a Celestial, e pela mesma
// razao: o cliente nao sabe redesenhar um personagem que trocou de nivel, de
// pontos e de habilidades debaixo dele.
//
// A entidade em memoria SO e substituida depois que a gravacao confirma. Se o
// banco falhar, o jogador continua com a vida que tinha e o item volta para o
// espaco, em vez de ficar com uma vida que nao existe no banco.
//
// A barra de atalho entra por parametro porque ela vive na SESSAO, fora da
// Entidade: descartar o `staged` nao a devolve sozinha, e uma barra apontando
// para magia que o personagem nao tem faz o servidor contar peso de trapaca.
func (d *Dispatcher) persisteTrocaDeVidaComBarra(w *world.World, s *world.Session, e *world.Entity, src int, staged world.Entity, motivo string, barraDeVolta [16]uint8) {
	save := w.CharacterSaveFor(s, &staged)
	p := w.Persistence()
	s.Mode = world.UserWaitDB
	w.Go(s, func() func(*world.World, *world.Session) {
		err := p.SaveOnShutdown(context.Background(), save)
		return func(w *world.World, s *world.Session) {
			if err != nil {
				s.Mode = world.UserPlay
				s.ShortSkill = barraDeVolta
				d.log.Warn("sub celestial save failed", "conn", s.Conn, "name", e.Name, "err", err)
				d.notify(w, s, NoticeDBError)
				d.sendSlot(w, s, world.ItemPlaceCarry, src, e.Carry[src])
				return
			}
			*e = staged
			d.sendSlot(w, s, world.ItemPlaceCarry, src, e.Carry[src])
			d.sendScore(w, s, e)
			d.sendEtc(w, s, e)
			d.returnPersistedCharacterToSelection(w, s, nil)
			d.log.Info(motivo, "conn", s.Conn, "name", e.Name,
				"nivel_ativo", e.Level, "nivel_guardado", e.SubCelestialLevel, "ativo", e.SubCelestialAtivo)
		}
	})
}
