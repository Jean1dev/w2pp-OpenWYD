package itemicons

import (
	"encoding/binary"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"
)

// clienteFalso monta o mínimo que SetIcon toca: a tabela item→ícone e um atlas
// vazio do tamanho que o cliente usa.
func clienteFalso(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "UI"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "itemicon.bin"), make([]byte, ItemCount*4), 0o644); err != nil {
		t.Fatal(err)
	}
	atlas := image.NewNRGBA(image.Rect(0, 0, Columns*CellSize, (IconsPerAtlas/Columns)*CellSize))
	if err := os.WriteFile(filepath.Join(dir, "UI", "itemicon01.wyt"), encodeWYT(atlas), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// bmp24 escreve um BMP sem compressão, o formato que sai de qualquer editor.
func bmp24(t *testing.T, dir string, w, h int, c color.NRGBA) string {
	t.Helper()
	rowSize := (w*3 + 3) / 4 * 4
	const header = 54
	out := make([]byte, header+rowSize*h)
	out[0], out[1] = 'B', 'M'
	binary.LittleEndian.PutUint32(out[2:], uint32(len(out)))
	binary.LittleEndian.PutUint32(out[10:], header)
	binary.LittleEndian.PutUint32(out[14:], 40)
	binary.LittleEndian.PutUint32(out[18:], uint32(w))
	binary.LittleEndian.PutUint32(out[22:], uint32(h))
	binary.LittleEndian.PutUint16(out[26:], 1)
	binary.LittleEndian.PutUint16(out[28:], 24)
	for row := range h {
		base := header + row*rowSize
		for x := range w {
			p := out[base+x*3:]
			p[0], p[1], p[2] = c.B, c.G, c.R
		}
	}
	path := filepath.Join(dir, "desenho.bmp")
	if err := os.WriteFile(path, out, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestSetIconGravaCelulaEAponta: o desenho entra na primeira célula livre, a
// tabela passa a apontar o item para ela, e o atlas continua legível pelo mesmo
// decodificador que lê os do cliente.
func TestSetIconGravaCelulaEAponta(t *testing.T) {
	dir := clienteFalso(t)
	vermelho := color.NRGBA{R: 200, G: 10, B: 10, A: 0xff}
	bmp := bmp24(t, dir, 32, 32, vermelho)

	const item = 3222
	icone, err := SetIcon(dir, item, bmp)
	if err != nil {
		t.Fatalf("SetIcon: %v", err)
	}
	if icone != 1 {
		t.Errorf("ícone gravado = %d, want 1 (primeira célula de um cliente vazio)", icone)
	}

	tabela, err := os.ReadFile(filepath.Join(dir, "itemicon.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if got := binary.LittleEndian.Uint32(tabela[item*4:]); got != 1 {
		t.Errorf("tabela[%d] = %d, want 1", item, got)
	}

	dados, err := os.ReadFile(filepath.Join(dir, "UI", "itemicon01.wyt"))
	if err != nil {
		t.Fatal(err)
	}
	atlas, err := decodeWYT(dados)
	if err != nil {
		t.Fatalf("o atlas gravado não volta pelo decodificador: %v", err)
	}
	// O desenho de 32 fica centralizado na célula de 35: sobra 1 de cada lado
	// em cima e à esquerda.
	centro := atlas.NRGBAAt(CellSize/2, CellSize/2)
	if centro.R != vermelho.R || centro.G != vermelho.G || centro.B != vermelho.B || centro.A != 0xff {
		t.Errorf("centro da célula = %+v, want o desenho %+v", centro, vermelho)
	}
	if canto := atlas.NRGBAAt(CellSize-1, CellSize-1); canto.A != 0 {
		t.Errorf("a borda da célula ficou opaca (%+v), want transparente", canto)
	}
}

// TestSetIconRecusaDesenhoGrande: a célula é 35×35 e um desenho maior invadiria
// o ícone do vizinho, então é recusado antes de gravar qualquer arquivo.
func TestSetIconRecusaDesenhoGrande(t *testing.T) {
	dir := clienteFalso(t)
	bmp := bmp24(t, dir, CellSize+1, CellSize, color.NRGBA{R: 1, A: 0xff})
	antes, err := os.ReadFile(filepath.Join(dir, "itemicon.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := SetIcon(dir, 3222, bmp); err == nil {
		t.Fatal("SetIcon aceitou um desenho maior que a célula")
	}
	depois, err := os.ReadFile(filepath.Join(dir, "itemicon.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if string(antes) != string(depois) {
		t.Error("a tabela mudou numa gravação recusada")
	}
}
