// Package campotreino é o campo de treino do novato para as regras que mais de
// um serviço precisa ler: onde ele fica e quem, lá dentro, é monstro.
//
// Existe porque o template tem dois bytes "Merchant". O legado decide quem é NPC
// pelo STRUCT_MOB.Merchant, o byte 17 (_MSG_Quest.cpp:33, _MSG_Attack.cpp:340).
// O port lê o CurrentScore.Merchant, o byte 104, em três lugares: na imunidade a
// golpe (tmserver/internal/world), em quais blocos o boot entrega à tabela de NPCs
// (tmserver/cmd/tmserver) e em quais o importador põe nela (dbserver). O
// Orc_Sniper e as Águias têm 0 no primeiro e 16 no segundo: no legado são
// monstros; aqui eram NPCs imortais e, em produção, linhas de npc_definition
// recriadas sem nome e por isso fora da Mesa de Drops.
//
// DIVERGÊNCIA DELIBERADA, de alcance: a regra do legado vale só dentro do campo,
// por decisão do Marco em 11/09/2026. Fora dele há 51 templates e 386 blocos com
// o mesmo desencontro — Ciclope Cruel, a família Zakum, dragões, guardas de
// reino — e liberá-los todos é outra decisão.
package campotreino

// A área que o AttributeMap marca com o bit 0x80 (_MSG_Action.cpp:215): um bloco
// só, logo ao norte de Armia (tmserver/internal/handler/campo_de_treino.go).
const (
	minX, minY = 2064, 1932
	maxX, maxY = 2159, 2051
)

// Contem diz se o tile está no campo de treino.
func Contem(x, y int) bool {
	return x >= minX && x <= maxX && y >= minY && y <= maxY
}

// MonstroNoCampo diz se um template que nasce em (x, y) é monstro pela regra do
// legado mesmo com o byte de loja do port ligado: dentro do campo e com o
// STRUCT_MOB.Merchant (byte 17) em 0. Fora do campo responde sempre false, e lá
// continua valendo a regra do byte 104.
func MonstroNoCampo(mobMerchant uint8, x, y int) bool {
	return mobMerchant == 0 && Contem(x, y)
}
