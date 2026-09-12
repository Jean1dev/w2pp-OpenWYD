package handler

import (
	"encoding/json"
	"errors"

	"github.com/jeanluca/w2pp-openwyd/internal/level"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// O Sub Celestial: a segunda vida do mesmo personagem.
//
// O jogador que ja e Celestial no nivel 120 ou acima, com o Sephirot no espaco
// 11, usa a Pedra Ideal e ganha um Sub — uma vida que comeca no nivel 1 e sobe
// em paralelo. Dai em diante a Pedra Misteriosa alterna entre as duas.
//
// O DESENHO, e por que ele e assim: as colunas de progressao que ja existiam
// continuam sendo A VIDA ATIVA, com o mesmo significado de sempre, e a vida
// guardada mora inteira num JSON (migracao 0057). Trocar de vida e trocar o
// conteudo dos dois lugares. Assim nenhum outro pedaco do servidor precisa
// aprender que existe segunda vida: ele continua lendo o que sempre leu.
//
// A alternativa — espelhar cada campo em dois — multiplicaria por dois um
// caminho que hoje ja toca dezesseis arquivos por campo, e obrigaria todo
// codigo que le `Level` a decidir qual dos dois vale.
//
// A EXCECAO e o nivel da vida guardada, que tem coluna propria: a formula de
// pontos do Celestial CS o le em TODA derivacao de score (login, subida de
// nivel, troca de equipamento). Ler JSON nessa frequencia seria caro a toa.

// errSemVidaGuardada e o caso em que o personagem tentou trocar de vida sem ter
// uma segunda. A trava da migracao ja impede isso no banco; aqui a checagem
// existe porque o handler nao pode confiar que o banco e a unica porta.
var errSemVidaGuardada = errors.New("personagem sem vida guardada")

// vidaGuardada e o que faz de uma vida uma vida: nivel, experiencia, pontos,
// atributos e habilidades.
//
// O QUE NAO ESTA AQUI, DE PROPOSITO: equipamento e bolsa. Eles sao do
// PERSONAGEM, nao da vida — a bolsa acompanha o jogador, e o equipamento tem de
// sair antes da troca de qualquer jeito (um Sub de nivel 1 vestido de Celestial
// 120 teria as pocas derivadas do conjunto errado, que foi o defeito que a
// propria useIdealStone ja conserta recusando em vez de apagar).
type vidaGuardada struct {
	Level int32 `json:"level"`
	Exp   int64 `json:"exp"`

	ScoreBonus   uint16 `json:"score_bonus"`
	SkillBonus   uint16 `json:"skill_bonus"`
	SpecialBonus uint16 `json:"special_bonus"`

	BaseStr int16 `json:"base_str"`
	BaseInt int16 `json:"base_int"`
	BaseDex int16 `json:"base_dex"`
	BaseCon int16 `json:"base_con"`

	BaseSpecial [4]int16 `json:"base_special"`

	LearnedSkill    int32    `json:"learned_skill"`
	SecLearnedSkill int32    `json:"sec_learned_skill"`
	SkillBar        [4]uint8 `json:"skill_bar"`

	// A BARRA DE ATALHO ENTRA AQUI, e a razao e do jogo, nao de arrumacao.
	//
	// Eu ia deixar as duas vidas compartilharem a barra, por ela viver na Sessao
	// e ser desenho de tela. Fui conferir o que acontece com um atalho apontando
	// para magia que a vida nova nao aprendeu, e nao e "botao que nao faz nada":
	// combat.go recusa o lancamento E CHAMA AddCrackError, que e o contador de
	// tentativa de trapaca. O teto e alto (2 bilhoes contra 8 por clique), entao
	// ninguem cai por isso — mas o jogador ficaria com a barra cheia de botao
	// morto e o servidor contando peso de trapaca em jogo normal.
	//
	// Guardar por vida custa passar a Sessao para ca e resolve os dois.
	ShortSkill [16]uint8 `json:"short_skill"`

	CelLv40   uint8 `json:"cel_lv40"`
	CelLv90   uint8 `json:"cel_lv90"`
	CelCircle uint8 `json:"cel_circle"`
}

// guardaVida copia a vida ATIVA do personagem para a estrutura que vai ao banco.
func guardaVida(s *world.Session, e *world.Entity) vidaGuardada {
	return vidaGuardada{
		Level: e.Level, Exp: e.Exp,
		ScoreBonus: e.ScoreBonus, SkillBonus: e.SkillBonus, SpecialBonus: e.SpecialBonus,
		BaseStr: e.BaseStr, BaseInt: e.BaseInt, BaseDex: e.BaseDex, BaseCon: e.BaseCon,
		BaseSpecial:     e.BaseSpecial,
		LearnedSkill:    e.LearnedSkill,
		SecLearnedSkill: e.SecLearnedSkill,
		SkillBar:        e.SkillBar,
		ShortSkill:      s.ShortSkill,
		CelLv40:         e.CelLv40, CelLv90: e.CelLv90, CelCircle: e.CelCircle,
	}
}

// restauraVida escreve uma vida guardada por cima da ativa.
//
// As pocas sao REFEITAS pela formula, nao restauradas do JSON: elas dependem de
// nivel, evolucao e constituicao, e uma copia gravada envelhece calada se
// qualquer uma das tres regras mudar. Derivar e a mesma escolha que o rewrite ja
// fez para a defesa base (playerBaseAC) depois do defeito da vida negativa.
func restauraVida(s *world.Session, e *world.Entity, v vidaGuardada) {
	e.Level, e.Exp = v.Level, v.Exp
	e.ScoreBonus, e.SkillBonus, e.SpecialBonus = v.ScoreBonus, v.SkillBonus, v.SpecialBonus
	e.BaseStr, e.BaseInt, e.BaseDex, e.BaseCon = v.BaseStr, v.BaseInt, v.BaseDex, v.BaseCon
	e.Str, e.Int, e.Dex, e.Con = v.BaseStr, v.BaseInt, v.BaseDex, v.BaseCon
	e.BaseSpecial, e.Special = v.BaseSpecial, v.BaseSpecial
	e.LearnedSkill, e.SecLearnedSkill = v.LearnedSkill, v.SecLearnedSkill
	e.SkillBar = v.SkillBar
	s.ShortSkill = v.ShortSkill
	e.CelLv40, e.CelLv90, e.CelCircle = v.CelLv40, v.CelLv90, v.CelCircle
	e.BaseMaxHP, e.BaseMaxMP = level.BasePools(e.Class, e.ClassMaster, e.Level, int32(e.BaseCon), int32(e.BaseInt))
	e.HP, e.MaxHP, e.MP, e.MaxMP = e.BaseMaxHP, e.BaseMaxHP, e.BaseMaxMP, e.BaseMaxMP
}

func codificaVida(v vidaGuardada) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func decodificaVida(s string) (vidaGuardada, error) {
	var v vidaGuardada
	if s == "" {
		return v, errSemVidaGuardada
	}
	err := json.Unmarshal([]byte(s), &v)
	return v, err
}
