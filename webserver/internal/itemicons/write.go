package itemicons

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"os"
	"path/filepath"
)

// Escrever um ícone novo no cliente é o outro lado de Generate: em vez de tirar
// os desenhos do cliente para a web, põe um desenho nosso dentro do cliente,
// para um item que o jogo ainda não sabe desenhar (a Chave do Inferno, 12/09/2026).
//
// São dois arquivos: a célula vai para o atlas numerado (UI\itemiconNN.wyt) e a
// tabela itemicon.bin passa a apontar o item para ela. Os dois têm de mudar
// juntos — um item apontando para uma célula em branco desenha um buraco, e uma
// célula pintada que ninguém aponta é peso morto no cliente.
//
// O atlas é reescrito do jeito que o cliente o entrega: "WT10" e um TGA tipo 2
// (sem RLE) de 350×350 e 32 bits, com o descritor 8 (origem embaixo, alfa de 8
// bits). O de 24 bits (itemicon09) vira 32 na volta, que é o que os outros dez
// já são e o que o decodificador aceita.

// SetIcon desenha bmpPath na primeira célula livre do atlas e aponta o item
// para ela, devolvendo o id do ícone gravado (1-based, como o cliente conta).
//
// O BMP tem de caber na célula de 35×35; um menor é centralizado, que é como as
// artes do cliente já estão dentro das suas células.
func SetIcon(clientDir string, item int, bmpPath string) (int, error) {
	if item <= 0 || item >= ItemCount {
		return 0, fmt.Errorf("itemicons: item %d fora de 1..%d", item, ItemCount-1)
	}
	art, err := readBMP24(bmpPath)
	if err != nil {
		return 0, err
	}
	if art.Bounds().Dx() > CellSize || art.Bounds().Dy() > CellSize {
		return 0, fmt.Errorf("itemicons: o desenho é %dx%d e a célula é %dx%d",
			art.Bounds().Dx(), art.Bounds().Dy(), CellSize, CellSize)
	}

	tablePath := filepath.Join(clientDir, "itemicon.bin")
	table, err := os.ReadFile(tablePath)
	if err != nil {
		return 0, fmt.Errorf("itemicons: ler %s: %w", tablePath, err)
	}
	itemToIcon, err := decodeIconTable(table)
	if err != nil {
		return 0, fmt.Errorf("itemicons: decodificar %s: %w", tablePath, err)
	}
	icon := freeIcon(itemToIcon)
	atlasIndex := icon / IconsPerAtlas
	atlasName := fmt.Sprintf("itemicon%02d.wyt", atlasIndex+1)
	atlasPath := filepath.Join(clientDir, "UI", atlasName)
	atlasData, err := os.ReadFile(atlasPath)
	if err != nil {
		return 0, fmt.Errorf("itemicons: ler %s: %w", atlasPath, err)
	}
	atlas, err := decodeWYT(atlasData)
	if err != nil {
		return 0, fmt.Errorf("itemicons: decodificar %s: %w", atlasName, err)
	}

	cell := icon % IconsPerAtlas
	x0 := (cell % Columns) * CellSize
	y0 := (cell / Columns) * CellSize
	if x0+CellSize > atlas.Bounds().Dx() || y0+CellSize > atlas.Bounds().Dy() {
		return 0, fmt.Errorf("itemicons: a célula %d não cabe em %s (%dx%d)",
			cell, atlasName, atlas.Bounds().Dx(), atlas.Bounds().Dy())
	}
	offX := x0 + (CellSize-art.Bounds().Dx())/2
	offY := y0 + (CellSize-art.Bounds().Dy())/2
	for y := range art.Bounds().Dy() {
		for x := range art.Bounds().Dx() {
			atlas.SetNRGBA(offX+x, offY+y, art.NRGBAAt(x, y))
		}
	}

	// A tabela guarda o id 1-based; 0 é "este item não tem ícone".
	if (item+1)*4 > len(table) {
		return 0, fmt.Errorf("itemicons: %s tem %d bytes e não alcança o item %d", tablePath, len(table), item)
	}
	binary.LittleEndian.PutUint32(table[item*4:], uint32(icon+1))

	// Os dois arquivos são gravados só depois de tudo dar certo, e o atlas
	// primeiro: um atlas novo com a tabela velha é invisível para o jogador,
	// enquanto a tabela nova com o atlas velho desenha lixo.
	if err := os.WriteFile(atlasPath, encodeWYT(atlas), 0o644); err != nil {
		return 0, fmt.Errorf("itemicons: gravar %s: %w", atlasName, err)
	}
	if err := os.WriteFile(tablePath, table, 0o644); err != nil {
		return 0, fmt.Errorf("itemicons: gravar %s: %w", tablePath, err)
	}
	return icon + 1, nil
}

// freeIcon is the first cell no item points at.
//
// "Free" means unreferenced, not blank: the shipped atlases are painted to the
// last cell, and the table stops pointing at cell 940 of 1100 — the art past it
// is orphan, drawn by nobody, and that is what gets overwritten here. Looking
// for a blank cell instead finds none, and taking a cell below the last
// referenced one would blank an icon some item is using.
func freeIcon(itemToIcon []int) int {
	max := -1
	for _, icon := range itemToIcon {
		if icon > max {
			max = icon
		}
	}
	return max + 1
}

// encodeWYT writes the wrapper and an uncompressed 32-bit TGA, the shape the
// client's own atlases have (type 2, descriptor 8, origin at the bottom).
func encodeWYT(img *image.NRGBA) []byte {
	w := img.Bounds().Dx()
	h := img.Bounds().Dy()
	out := make([]byte, 0, 4+18+w*h*4)
	out = append(out, standardWYTWrapper...)
	header := [18]byte{}
	header[2] = 2 // true-color, sem RLE
	binary.LittleEndian.PutUint16(header[12:], uint16(w))
	binary.LittleEndian.PutUint16(header[14:], uint16(h))
	header[16] = 32
	header[17] = 8 // alfa de 8 bits, origem embaixo
	out = append(out, header[:]...)
	for y := h - 1; y >= 0; y-- {
		for x := range w {
			p := img.NRGBAAt(x, y)
			out = append(out, p.B, p.G, p.R, p.A)
		}
	}
	// Os atlas do cliente terminam com oito bytes zerados depois dos pixels — não
	// é o rodapé do TGA, que traria "TRUEVISION-XFILE", e sim enchimento. O
	// decodificador ignora o que sobra, mas o arquivo volta com o tamanho exato
	// dos outros dez, e um arquivo do mesmo tamanho é um a menos que alguém vai
	// olhar torto no dia em que o cliente reclamar de outra coisa.
	return append(out, make([]byte, 8)...)
}

// readBMP24 reads an uncompressed 24- or 32-bit BMP, which is what a drawing
// exported by any editor is. The standard library has no BMP decoder and this
// is the whole format we need, so it is read here instead of adding a
// dependency to the module.
//
// Black is turned into transparent, the same rule decodeTGAPixels applies to
// the client's own art: the classic icons carry no alpha and the client treats
// black as the hole around the drawing.
func readBMP24(path string) (*image.NRGBA, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("itemicons: ler o desenho: %w", err)
	}
	if len(data) < 54 || data[0] != 'B' || data[1] != 'M' {
		return nil, fmt.Errorf("itemicons: %s não é um BMP", path)
	}
	pixelOffset := int(binary.LittleEndian.Uint32(data[10:]))
	width := int(int32(binary.LittleEndian.Uint32(data[18:])))
	height := int(int32(binary.LittleEndian.Uint32(data[22:])))
	bits := int(binary.LittleEndian.Uint16(data[28:]))
	compression := binary.LittleEndian.Uint32(data[30:])
	if compression != 0 || (bits != 24 && bits != 32) || width <= 0 || height == 0 {
		return nil, fmt.Errorf("itemicons: BMP %dx%d de %d bits com compressão %d não é suportado",
			width, height, bits, compression)
	}
	topDown := height < 0
	if topDown {
		height = -height
	}
	pixelSize := bits / 8
	rowSize := (width*pixelSize + 3) / 4 * 4
	if pixelOffset+rowSize*height > len(data) {
		return nil, fmt.Errorf("itemicons: %s acaba antes dos pixels", path)
	}
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for row := range height {
		y := height - 1 - row // BMP guarda de baixo para cima
		if topDown {
			y = row
		}
		base := pixelOffset + row*rowSize
		for x := range width {
			p := data[base+x*pixelSize:]
			c := color.NRGBA{B: p[0], G: p[1], R: p[2], A: 0xff}
			if pixelSize == 4 {
				c.A = p[3]
			}
			if c.R == 0 && c.G == 0 && c.B == 0 {
				c.A = 0
			}
			img.SetNRGBA(x, y, c)
		}
	}
	return img, nil
}
