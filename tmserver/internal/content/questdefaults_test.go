package content

import (
	"testing"

	"github.com/jeanluca/w2pp-openwyd/internal/domain"
)

// TestPadroesDeQuestVemDeUmLugarSo is the tie that keeps the admin panel honest.
//
// The panel shows "no arquivo: 1000 de XP" beside an edited quest, and it reads
// that from domain.QuestRewardDefaults because it cannot import this package.
// If the two ever held separate copies, the panel would keep claiming a number
// the game had stopped paying — and nothing would fail.
func TestPadroesDeQuestVemDeUmLugarSo(t *testing.T) {
	if len(domain.QuestRewardDefaults) != questTierCount {
		t.Fatalf("domain tem %d padrões, o jogo tem %d degraus",
			len(domain.QuestRewardDefaults), questTierCount)
	}
	q := DefaultQuestRates()
	for i, d := range domain.QuestRewardDefaults {
		got, ok := q.Tier(i)
		if !ok {
			t.Fatalf("o degrau %d não existe no jogo", i)
		}
		want := QuestTierRate{
			MortalExp: d.MortalExp, ArchExp: d.ArchExp, Coin: d.Coin,
			MortalMin: d.MortalMin, MortalMax: d.MortalMax,
			ArchMin: d.ArchMin, ArchMax: d.ArchMax,
		}
		if got != want {
			t.Errorf("degrau %d: o jogo paga %+v, o painel mostra %+v", i, got, want)
		}
	}
}

// TestSetTierSobrepoeSoODegrauPedido: the panel's overlay must land on one quest
// and leave the others reading the content file.
func TestSetTierSobrepoeSoODegrauPedido(t *testing.T) {
	q := DefaultQuestRates()
	antes, _ := q.Tier(1)

	novo := QuestTierRate{MortalExp: 9_000_000, ArchExp: 4_500_000, Coin: 1_000_000,
		MortalMin: 1, MortalMax: 400, ArchMin: 1, ArchMax: 400}
	if !q.SetTier(0, novo) {
		t.Fatal("SetTier recusou o degrau 0")
	}
	got, _ := q.Tier(0)
	if got != novo {
		t.Errorf("degrau 0 = %+v, quero %+v", got, novo)
	}
	if depois, _ := q.Tier(1); depois != antes {
		t.Errorf("o degrau 1 mudou junto: %+v virou %+v", antes, depois)
	}

	// Um degrau que não existe é recusado em vez de dobrar num vizinho.
	for _, fora := range []int{-1, questTierCount, 99} {
		if q.SetTier(fora, novo) {
			t.Errorf("SetTier(%d) aceitou um degrau que não existe", fora)
		}
	}
}
