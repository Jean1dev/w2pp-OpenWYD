// Package clientitemlist writes the client's ItemList.bin from the server's
// ItemList.csv, so the catalog the player reads is the catalog the server runs.
//
// The two had drifted apart: the client that became the installer base shows
// "Cupom da Sorte" where the server has the Baú da Pedra Secreta and the other
// baús (3210-3221), names the Âmagos de Svadilfari/Andaluz N and de
// Sleipnir/Unicórnio the other way round, and carries different grades,
// effects and prices on some two hundred items. A player reading one item while
// the server hands out another is the worst kind of bug — nothing fails. The
// fix is not to patch the client item by item but to stop keeping two
// catalogs: the CSV is the source, and this writes the client's copy from it.
//
// Two things the CSV does not have come from the client's own file: each
// record's visual effect (IndexVisualEffect, +68) and the 4 trailing bytes the
// client does not check. Records the CSV does not define are cleared.
//
// Nothing is guessed. A row the encoder cannot represent — an effect name it
// does not know, more than twelve effects, a name that does not fit — refuses
// the whole file, with the line: half a catalog in a client is worse than none.
package clientitemlist

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// The client ItemList.bin: 6500 records of 140 bytes under a flat XOR 0x5A,
// plus 4 trailing bytes. STRUCT_ITEMLIST: Name[64]; shorts IndexMesh (+64),
// IndexTexture (+66), IndexVisualEffect (+68), ReqLvl/Str/Int/Dex/Con (+70);
// twelve (short effect, short value) pairs (+80); int Price (+128); shorts
// nUnique (+132), nPos (+134), Extra (+136), Grade (+138).
const (
	ItemCount   = 6500
	recordSize  = 140
	FileSize    = ItemCount*recordSize + 4
	xorKey      = 0x5A
	nameSize    = 64
	offMesh     = 64
	offTexture  = 66
	offVFX      = 68
	offReq      = 70
	offEffects  = 80
	effectSlots = 12
	offPrice    = 128
	offUnique   = 132
	offPos      = 134
	offExtra    = 136
	offGrade    = 138
)

// Effect is one catalog effect pair.
type Effect struct {
	Code  int16
	Value int16
}

// Row is one ItemList.csv line, as the client record stores it.
type Row struct {
	Index   int
	Name    []byte // the catalog's own bytes (Windows-1252, "_" for space)
	Mesh    int16
	Texture int16
	Req     [5]int16
	Unique  int16
	Price   int32
	Pos     int16
	Extra   int16
	Grade   int16
	Effects []Effect
}

// ParseCSV reads the server's ItemList.csv:
//
//	index,name,mesh.texture,lvl.str.int.dex.con,unique,price,pos,extra,grade,EF_X,value,...
//
// The file is read as bytes, never as UTF-8 text: names are Windows-1252 and go
// into the client exactly as they are. Blank lines are skipped.
func ParseCSV(r io.Reader) ([]Row, error) {
	var rows []Row
	seen := map[int]bool{}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	line := 0
	for sc.Scan() {
		line++
		raw := bytes.TrimRight(sc.Bytes(), "\r")
		if len(bytes.TrimSpace(raw)) == 0 {
			continue
		}
		row, err := parseRow(raw)
		if err != nil {
			return nil, fmt.Errorf("clientitemlist: linha %d: %w", line, err)
		}
		if seen[row.Index] {
			return nil, fmt.Errorf("clientitemlist: linha %d: item %d repetido", line, row.Index)
		}
		seen[row.Index] = true
		rows = append(rows, row)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("clientitemlist: ler o CSV: %w", err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("clientitemlist: o CSV não tem item nenhum")
	}
	return rows, nil
}

func parseRow(raw []byte) (Row, error) {
	f := bytes.Split(raw, []byte(","))
	if len(f) < 9 {
		return Row{}, fmt.Errorf("%d campos, esperava ao menos 9", len(f))
	}
	num := func(b []byte, what string) (int, error) {
		v, err := strconv.Atoi(strings.TrimSpace(string(b)))
		if err != nil {
			return 0, fmt.Errorf("%s ilegível: %q", what, b)
		}
		return v, nil
	}
	var row Row
	var err error
	if row.Index, err = num(f[0], "índice"); err != nil {
		return Row{}, err
	}
	if row.Index <= 0 || row.Index >= ItemCount {
		return Row{}, fmt.Errorf("índice %d fora de 1..%d", row.Index, ItemCount-1)
	}
	// A copy: f points into the scanner's buffer, which the next line overwrites.
	row.Name = bytes.Clone(bytes.TrimSpace(f[1]))
	if len(row.Name) == 0 || len(row.Name) >= nameSize {
		return Row{}, fmt.Errorf("item %d: nome com %d bytes, o registro guarda até %d", row.Index, len(row.Name), nameSize-1)
	}
	mesh := bytes.Split(f[2], []byte("."))
	if len(mesh) != 2 {
		return Row{}, fmt.Errorf("item %d: ícone %q não é malha.textura", row.Index, f[2])
	}
	m, err := num(mesh[0], "malha")
	if err != nil {
		return Row{}, err
	}
	tx, err := num(mesh[1], "textura")
	if err != nil {
		return Row{}, err
	}
	row.Mesh, row.Texture = int16(m), int16(tx)
	req := bytes.Split(f[3], []byte("."))
	if len(req) != 5 {
		return Row{}, fmt.Errorf("item %d: requisito %q não tem 5 partes", row.Index, f[3])
	}
	for i := range req {
		v, err := num(req[i], "requisito")
		if err != nil {
			return Row{}, err
		}
		row.Req[i] = int16(v)
	}
	fields := []*int16{&row.Unique, nil, &row.Pos, &row.Extra, &row.Grade}
	names := []string{"unique", "preço", "posição", "extra", "grau"}
	for i, dst := range fields {
		v, err := num(f[4+i], names[i])
		if err != nil {
			return Row{}, err
		}
		if dst == nil {
			row.Price = int32(v)
			continue
		}
		*dst = int16(v)
	}
	for i := 9; i < len(f); i += 2 {
		name := strings.TrimSpace(string(f[i]))
		if name == "" {
			continue
		}
		code, ok := efCode[name]
		if !ok {
			return Row{}, fmt.Errorf("item %d: efeito %q desconhecido", row.Index, name)
		}
		if i+1 >= len(f) {
			return Row{}, fmt.Errorf("item %d: efeito %s sem valor", row.Index, name)
		}
		v, err := num(f[i+1], name)
		if err != nil {
			return Row{}, err
		}
		row.Effects = append(row.Effects, Effect{Code: int16(code), Value: int16(v)})
	}
	if len(row.Effects) > effectSlots {
		return Row{}, fmt.Errorf("item %d: %d efeitos, o registro guarda %d", row.Index, len(row.Effects), effectSlots)
	}
	return row, nil
}

// Build writes the client ItemList.bin for rows, taking from base — the
// client's current file — only what the CSV does not carry: each record's
// visual effect and the trailing bytes.
func Build(rows []Row, base []byte) ([]byte, error) {
	if len(base) != FileSize {
		return nil, fmt.Errorf("clientitemlist: o ItemList.bin do cliente tem %d bytes, esperava %d", len(base), FileSize)
	}
	out := make([]byte, FileSize)
	copy(out[ItemCount*recordSize:], base[ItemCount*recordSize:])
	rec := make([]byte, recordSize)
	put := func(off int, v int16) { binary.LittleEndian.PutUint16(rec[off:], uint16(v)) }
	for _, r := range rows {
		clear(rec)
		at := r.Index * recordSize
		// The visual effect of the client's own record, if it had one.
		var old [recordSize]byte
		for i := range old {
			old[i] = base[at+i] ^ xorKey
		}
		if old[0] != 0 {
			copy(rec[offVFX:offVFX+2], old[offVFX:offVFX+2])
		}
		copy(rec[:nameSize-1], r.Name)
		put(offMesh, r.Mesh)
		put(offTexture, r.Texture)
		for i, v := range r.Req {
			put(offReq+2*i, v)
		}
		for i, ef := range r.Effects {
			put(offEffects+4*i, ef.Code)
			put(offEffects+4*i+2, ef.Value)
		}
		binary.LittleEndian.PutUint32(rec[offPrice:], uint32(r.Price))
		put(offUnique, r.Unique)
		put(offPos, r.Pos)
		put(offExtra, r.Extra)
		put(offGrade, r.Grade)
		for i := range rec {
			out[at+i] = rec[i] ^ xorKey
		}
	}
	// Records the CSV does not define stay empty: a zero record under the XOR.
	defined := make([]bool, ItemCount)
	for _, r := range rows {
		defined[r.Index] = true
	}
	for i := 0; i < ItemCount; i++ {
		if defined[i] {
			continue
		}
		for j := 0; j < recordSize; j++ {
			out[i*recordSize+j] = xorKey
		}
	}
	return out, nil
}
