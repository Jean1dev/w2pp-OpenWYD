package handler

import (
	"testing"

	"github.com/jeanluca/w2pp-openwyd/internal/level"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

const (
	tierMortal    = classMasterMortal
	tierArch      = classMasterArch
	tierCelestial = 3
)

func quem(cm uint8, lvl int32) *world.Entity {
	return &world.Entity{ClassMaster: cm, Level: lvl}
}

// The celestial tiers restart their level count at 1 on a separate curve, so a
// raw Level comparison puts a Celestial 10 three hundred and seventy levels
// below a Mortal 380 — and the invite was refused for two players who are two
// levels apart in progression. The legacy adds MAX_CLEVEL before comparing
// (_MSG_SendReqParty.cpp:69); the port did not.
func TestNivelDeGrupoNormalizaAsEvolucoesCelestiais(t *testing.T) {
	if got := partyLevelForParty(quem(tierMortal, 380)); got != 380 {
		t.Errorf("mortal 380 = %d, want 380 (sem deslocamento)", got)
	}
	if got := partyLevelForParty(quem(tierArch, 380)); got != 380 {
		t.Errorf("arch 380 = %d, want 380 (sem deslocamento)", got)
	}
	want := 10 + int(level.MaxCLevel)
	if got := partyLevelForParty(quem(tierCelestial, 10)); got != want {
		t.Errorf("celestial 10 = %d, want %d (Level + MAX_CLEVEL)", got, want)
	}
}

// The case from the report: a Mortal near cap inviting a fresh Celestial. It has
// to work — they are neighbours in progression — and before the fix it was
// refused, silently.
func TestMortalNoTopoPodeGruparCelestialNovo(t *testing.T) {
	mortal, celestial := quem(tierMortal, 380), quem(tierCelestial, 10)
	if !partyLevelOK(mortal, celestial) {
		t.Errorf("mortal 380 não pôde convidar celestial 10 (níveis normalizados %d e %d)",
			partyLevelForParty(mortal), partyLevelForParty(celestial))
	}
	if !partyLevelOK(celestial, mortal) {
		t.Error("e o convite no sentido contrário também tem que valer")
	}
}

// Same tier short-circuits before any arithmetic, exactly as the legacy does:
// two Mortals group whatever the gap.
func TestMesmaEvolucaoIgnoraADiferenca(t *testing.T) {
	if !partyLevelOK(quem(tierMortal, 1), quem(tierMortal, 399)) {
		t.Error("dois mortais devem poder grupar em qualquer diferença de nível")
	}
	if !partyLevelOK(quem(tierCelestial, 1), quem(tierCelestial, 199)) {
		t.Error("dois celestiais idem")
	}
}

// The gate still has to refuse a real gap across tiers, or it stops being a
// gate. PARTY_DIF is 200 — the number Language.txt:215 quotes back to the
// player — and the bound is asymmetric as the legacy writes it.
func TestGateAindaRecusaDiferencaDeVerdade(t *testing.T) {
	// Arch 10 vs Celestial 199 → 10 e 398: 388 de diferença.
	if partyLevelOK(quem(tierArch, 10), quem(tierCelestial, 199)) {
		t.Error("arch 10 e celestial 199 estão longe demais; devia recusar")
	}
	// Exactly at the lower edge, which the legacy includes.
	leader, alvo := quem(tierMortal, 300), quem(tierCelestial, 300-200-level.MaxCLevel)
	if partyLevelForParty(alvo) != 100 {
		t.Fatalf("o caso de borda foi montado errado: alvo normalizado = %d", partyLevelForParty(alvo))
	}
	if !partyLevelOK(leader, alvo) {
		t.Error("a borda de baixo é inclusiva no legado (lvl >= leaderlv - PARTY_DIF)")
	}
	// And one below it is out.
	fora := quem(tierCelestial, 300-201-level.MaxCLevel)
	if partyLevelOK(leader, fora) {
		t.Error("um nível abaixo da borda tem que ficar de fora")
	}
}

// PARTY_DIF must match what the game tells the player. Language.txt:215 says
// "diferença de 200 níveis"; shipping 100 made the server contradict its own
// refusal message.
func TestPartyDifBateComAMensagemDoJogo(t *testing.T) {
	if partyDif != 200 {
		t.Errorf("partyDif = %d, want 200 (Server.cpp:51 e Language.txt:215)", partyDif)
	}
}
