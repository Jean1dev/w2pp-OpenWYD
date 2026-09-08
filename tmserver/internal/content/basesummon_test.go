package content

import (
	"path/filepath"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
)

func TestLoadBaseSummons(t *testing.T) {
	dir := filepath.Join("..", "..", "..", "Release")
	templates, missing, err := LoadBaseSummons(dir)
	if err != nil {
		t.Skipf("BaseSummon tree unavailable: %v", err)
	}
	if len(templates) != 40 {
		t.Fatalf("templates = %d slots, want 40 (CNPCSummon::Initialize order)", len(templates))
	}
	// The 8 BM evocation creatures are mandatory and byte-sized like any
	// STRUCT_MOB template.
	for i := 0; i < bmSummonCount; i++ {
		if templates[i] == nil {
			t.Errorf("BM summon %d (%s) missing", i, baseSummonFiles[i])
			continue
		}
		if len(templates[i]) != BaseMobSize {
			t.Errorf("summon %d (%s) = %d bytes, want %d", i, baseSummonFiles[i], len(templates[i]), BaseMobSize)
		}
	}
	// The case-corrected mount template must load on Linux (the C++ says
	// "Dragao_menor"; the file is Dragao_Menor).
	if templates[13] == nil {
		t.Errorf("summon 13 (Dragao_Menor) missing — case fix regressed?")
	}
	for _, name := range missing {
		t.Logf("optional summon missing: %s", name)
	}
}

// TestEvocacoesLigamDaSkillAteOTemplate percorre as oito evocações do BM de
// ponta a ponta contra o conteúdo REAL: a linha do SkillData, o InstanceValue, o
// summonID que ele produz, e o template que esse id carrega.
//
// Existe porque toda falha da evocação nesta sessão veio da mesma coisa — um
// teste montado sobre dados que o jogo não tem. O fixture do handler constrói o
// STRUCT_MOB à mão e acertava por ser degenerado (Merchant zero, Equip zerado);
// os arquivos de verdade trazem Merchant 16, e era ele que tirava o bicho do
// laço de IA. Um teste que lê os arquivos é o único que fecha essa porta.
func TestEvocacoesLigamDaSkillAteOTemplate(t *testing.T) {
	sk, err := LoadSkillData(filepath.Join("..", "..", "..", "Release", "Common", "SkillData.csv"))
	if err != nil {
		t.Skipf("SkillData unavailable: %v", err)
	}
	templates, _, err := LoadBaseSummons(filepath.Join("..", "..", "..", "Release"))
	if err != nil {
		t.Skipf("BaseSummon tree unavailable: %v", err)
	}

	// As oito magias de evocação são as skills 56..63, em ordem.
	const primeira, ultima = 56, 63
	for idx := primeira; idx <= ultima; idx++ {
		sp, ok := sk.Get(idx)
		if !ok {
			t.Errorf("skill %d não está no SkillData; a magia não tem como ser lançada", idx)
			continue
		}
		// InstanceType 11 é o que roteia para GenerateSummon (_MSG_Attack.cpp:809).
		// Sem ele a magia gasta mana e não faz nada.
		if sp.InstanceType != 11 {
			t.Errorf("skill %d (%s): InstanceType = %d, want 11", idx, sp.Name, sp.InstanceType)
		}
		// summonID = InstanceValue-1 (_MSG_Attack.cpp:830). Fora de 0..7 o
		// GenerateSummon recusa calado e a magia some sem aviso.
		sumID := sp.InstanceValue - 1
		if sumID < 0 || sumID >= bmSummonCount {
			t.Errorf("skill %d (%s): InstanceValue %d dá summonID %d, fora de 0..%d",
				idx, sp.Name, sp.InstanceValue, sumID, bmSummonCount-1)
			continue
		}
		if templates[sumID] == nil {
			t.Errorf("skill %d (%s) aponta para o template %d (%s), que não carregou",
				idx, sp.Name, sumID, baseSummonFiles[sumID])
		}
	}

	// Cada uma das oito magias tem de cair num summonID diferente: duas magias no
	// mesmo bicho significaria uma linhagem inalcançável.
	visto := map[int]int{}
	for idx := primeira; idx <= ultima; idx++ {
		sp, ok := sk.Get(idx)
		if !ok {
			continue
		}
		sumID := sp.InstanceValue - 1
		if antes, repetido := visto[sumID]; repetido {
			t.Errorf("skills %d e %d apontam para o mesmo summonID %d (%s)",
				antes, idx, sumID, baseSummonFiles[sumID])
		}
		visto[sumID] = idx
	}
	if len(visto) != bmSummonCount {
		t.Errorf("as oito magias cobrem %d criaturas, want %d", len(visto), bmSummonCount)
	}
}

// TestSummonsNaoHesitamNaBatalha trava o Int dos templates de evocação.
//
// A IA do mob decide se age no tique rolando contra o próprio Int
// (mobBattle: `Int < rand(0..99)` faz pular o turno). Isso quer dizer que Int
// abaixo de 100 não é "menos esperto": é uma fração dos turnos jogada fora, e
// aparece em jogo como bicho lerdo sem nenhuma mensagem que explique.
//
// A Succubus vinha com 70 e perdia perto de um terço dos ataques — a última
// magia da linha, com o pior aproveitamento de todas. O valor foi corrigido no
// arquivo; este teste é o que impede a regressão passar de novo despercebida.
func TestSummonsNaoHesitamNaBatalha(t *testing.T) {
	templates, _, err := LoadBaseSummons(filepath.Join("..", "..", "..", "Release"))
	if err != nil {
		t.Skipf("BaseSummon tree unavailable: %v", err)
	}
	for i := 0; i < bmSummonCount; i++ {
		if templates[i] == nil {
			continue
		}
		b := protocol.ParseMobBasics(templates[i])
		if b.Int < 100 {
			t.Errorf("%s: Int %d — perde ~%d%% dos turnos na rolagem da IA",
				baseSummonFiles[i], b.Int, 100-int(b.Int))
		}
	}
}
