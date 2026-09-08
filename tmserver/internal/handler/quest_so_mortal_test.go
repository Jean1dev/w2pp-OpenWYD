package handler

import (
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// As cinco quests — Cemitério, Jardim dos Deuses, Coração do Kaizen, Hidras e
// Elfos — são de Mortal neste servidor, e o legado não é.
//
// O legado deixa Arch entrar em todas: os seis portões de _MSG_Quest.cpp leem
// `!= MORTAL && != ARCH`, e o do troféu (_MSG_UseItem.cpp:2346) está COMENTADO,
// de modo que qualquer classe usa o item. Um Arch nesses níveis é um personagem
// renascido, muito mais forte que o Mortal para quem as faixas foram desenhadas,
// e é esse desequilíbrio que a regra remove.
//
// A regra vale em três lugares e os três importam: o NPC não entrega o bilhete,
// o bilhete não teleporta, e o troféu não paga. Só o primeiro seria uma cerca
// com portão aberto — o troféu é item negociável, então quem não fez a quest
// compraria o de quem fez.

// classesNaoMortais são as que precisam ser recusadas nos três portões.
var classesNaoMortais = []struct {
	nome  string
	class uint8
}{
	{"arch", classMasterArch},
	{"celestial", classMasterCelestial},
}

// TestBilheteDeQuestSoServeParaMortal cobre o portão do meio: usar o bilhete.
func TestBilheteDeQuestSoServeParaMortal(t *testing.T) {
	for _, cl := range classesNaoMortais {
		t.Run(cl.nome, func(t *testing.T) {
			st := world.CharacterState{
				Slot: 0, Name: "Hero", Level: 50, X: 2113, Y: 2079,
				HP: 1000, MaxHP: 1000, LastCity: 0, ClassMaster: cl.class,
			}
			st.Carry[0] = world.Item{Index: 4038} // Vela do Coveiro
			addr, stop, _ := startServerMestreGrifo(t, st, false)
			defer stop()
			c := enterWorld(t, addr)
			defer c.Close()

			body := protocol.MsgUseItemBody{SourType: world.ItemPlaceCarry, SourPos: 0}
			send(t, c, protocol.MsgUseItem, body.Encode())

			if code := noticeCode(t, expect(t, c, protocol.MsgMessageBoxOk)); code != NoticeReqNotMet {
				t.Fatalf("aviso = %d, quero NoticeReqNotMet", code)
			}
			// O bilhete tem de voltar intacto: recusar e ainda assim consumir
			// seria roubar o item de quem apenas não pode usá-lo.
			item := expect(t, c, protocol.MsgSendItem)
			if slot, idx := le16(item[2:4]), le16(item[4:6]); slot != 0 || idx != 4038 {
				t.Fatalf("slot=%d idx=%d, quero o bilhete devolvido", slot, idx)
			}
			if ty, _, ok := readMaybe(t, c); ok && ty == protocol.MsgAction {
				t.Fatalf("teleportou mesmo recusado (%#x)", ty)
			}
		})
	}
}

// TestBilheteDeQuestAindaServeParaMortal é a outra metade: a regra não pode ter
// fechado a quest para quem ela é.
func TestBilheteDeQuestAindaServeParaMortal(t *testing.T) {
	st := world.CharacterState{
		Slot: 0, Name: "Hero", Level: 50, X: 2113, Y: 2079,
		HP: 1000, MaxHP: 1000, LastCity: 0, ClassMaster: classMasterMortal,
	}
	st.Carry[0] = world.Item{Index: 4038}
	addr, stop, _ := startServerMestreGrifo(t, st, false)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	body := protocol.MsgUseItemBody{SourType: world.ItemPlaceCarry, SourPos: 0}
	send(t, c, protocol.MsgUseItem, body.Encode())

	consumido := expect(t, c, protocol.MsgSendItem)
	if slot, idx := le16(consumido[2:4]), le16(consumido[4:6]); slot != 0 || idx != 0 {
		t.Fatalf("slot=%d idx=%d, quero o bilhete consumido", slot, idx)
	}
	if a := expectAction(t, c); a.Effect != 1 {
		t.Fatalf("efeito = %d, quero o teleporte do bilhete", a.Effect)
	}
}

// TestTrofeuDeQuestSoPagaMortal cobre o portão que mais importa, porque o troféu
// é negociável: sem ele, a regra seria contornada comprando o item.
//
// De quebra fecha um buraco que existia mesmo antes da regra: o ramo de classe
// era `== MORTAL ? mortal : arch`, então TODA evolução celestial recebia
// silenciosamente os números de Arch.
func TestTrofeuDeQuestSoPagaMortal(t *testing.T) {
	for _, cl := range classesNaoMortais {
		t.Run(cl.nome, func(t *testing.T) {
			db := questRewardDB(4117, 50, 1)
			st := db.loadResult
			st.ClassMaster = cl.class
			db.loadResult = st

			addr, stop := startServerClockVol(t, db, map[int]int{4117: volQuestReward})
			defer stop()
			c := enterWorld(t, addr)
			defer c.Close()

			useQuestItem(t, c)
			if code := noticeCode(t, expect(t, c, protocol.MsgMessageBoxOk)); code != NoticeReqNotMet {
				t.Fatalf("aviso = %d, quero NoticeReqNotMet", code)
			}
			exp, coin, item, _ := collectQuestResult(t, c, 8)
			if exp != 0 || coin != 0 {
				t.Errorf("pagou %d de XP e %d de ouro a um %s", exp, coin, cl.nome)
			}
			if item == nil {
				t.Fatal("não devolveu o slot")
			}
			if idx := le16(item[4:6]); idx != 4117 {
				t.Errorf("slot = %d, quero o troféu devolvido intacto", idx)
			}
		})
	}
}

// TestTrofeuDeQuestAindaPagaMortal garante que a recusa não pegou quem deve
// receber.
func TestTrofeuDeQuestAindaPagaMortal(t *testing.T) {
	addr, stop := startServerClockVol(t, questRewardDB(4117, 50, 1), map[int]int{4117: volQuestReward})
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	useQuestItem(t, c)
	exp, coin, _, _ := collectQuestResult(t, c, 8)
	if exp <= 0 || coin <= 0 {
		t.Fatalf("um Mortal no nível 50 recebeu %d de XP e %d de ouro", exp, coin)
	}
}
