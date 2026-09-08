package spawnrate

import (
	"path/filepath"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/internal/npcgener"
)

// TestCensoBateComOArquivo re-counts the real NPCGener.txt and compares it with
// the table the panel shows. The numbers are transcribed on purpose — this
// package does no I/O — and a transcription with nothing checking it is a
// transcription that goes stale. If the content tree ever changes, this fails
// with the new census rather than letting the panel quote a world that no longer
// exists.
func TestCensoBateComOArquivo(t *testing.T) {
	gens, err := npcgener.Load(filepath.Join("..", "..", "Release", "TMsrv", "run", "NPCGener.txt"))
	if err != nil {
		t.Skipf("sem a árvore de conteúdo: %v", err)
	}
	contado := map[Area]map[int]int{}
	for _, g := range gens {
		a, ok := AreaForTile(int32(g.SegX[0]), int32(g.SegY[0]))
		if !ok {
			continue
		}
		if contado[a] == nil {
			contado[a] = map[int]int{}
		}
		minutos := g.MinuteGenerate
		if minutos < 0 {
			// Every "no minute period" spelling collapses to the same fact: this
			// block goes through the individual respawn queue.
			minutos = 0
		}
		contado[a][minutos]++
	}
	for _, a := range Areas() {
		esperado := map[int]int{}
		for _, p := range a.Periods() {
			esperado[p.Minutes] = p.Blocks
		}
		got := contado[a]
		if len(got) != len(esperado) {
			t.Errorf("%s: o arquivo tem %v, a tabela diz %v", a.Name(), got, esperado)
			continue
		}
		for minutos, blocos := range esperado {
			if got[minutos] != blocos {
				t.Errorf("%s: %d minutos tem %d blocos no arquivo, %d na tabela",
					a.Name(), minutos, got[minutos], blocos)
			}
		}
	}
}
