package content

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

// O ouro do campo de treino é o Coin dos templates (STRUCT_MOB.Coin @28): a Mesa
// de Drops não dá moeda. A Águia vinha com 0 e não dava ouro nenhum; o Porco,
// com 5, dava uns 9 por morte. Com 10, os dois dão ~17 por morte (1 em 3 mortes,
// 48-56 de ouro, loot.GoldDrop). Os dois arquivos só nascem no campo, então o
// número não vaza para outro mapa.
func TestOuroDoCampoDeTreino(t *testing.T) {
	npcDir := filepath.Join("..", "..", "..", "Release", "TMsrv", "run", "npc")
	if _, err := os.Stat(npcDir); err != nil {
		t.Skip("Release content unavailable")
	}
	for _, nome := range []string{"Aguia", "Porco"} {
		b, err := os.ReadFile(filepath.Join(npcDir, nome))
		if err != nil {
			t.Fatalf("%s: %v", nome, err)
		}
		if len(b) < 32 {
			t.Fatalf("%s: template com %d bytes", nome, len(b))
		}
		if coin := int32(binary.LittleEndian.Uint32(b[28:])); coin != 10 {
			t.Errorf("%s: Coin = %d, want 10", nome, coin)
		}
	}
}
