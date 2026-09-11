// Package clientmount writes the mount numbers the staff panel decided into the
// client files that show them:
//
//   - WYD.exe: the attribute table the mount tooltip is drawn from, and the
//     tooltip's own list of lines, where two entries are set aside for the
//     absorption — a stat the original client has no line for;
//   - ItemList.bin: each mount's absorption, as the two catalog effects those
//     entries print;
//   - UI\strdef.bin: the labels of those two lines.
//
// itemhelp.dat — the per-item description text — was the first attempt and does
// not work: the mount tooltip never shows it, wherever the entry sits in the file.
// The colours of the lines are not data at all; they come from GamePatch.dll
// (client/gamepatch), which the client loads on its own.
//
// It exists because the server and the client each carry a copy of the mount
// table, and the day they disagreed every mount on the server hit like a Dragão
// Vermelho while the tooltip said otherwise. The panel changes the server's
// copy; this is the only way the client's copy follows.
//
// Nothing here guesses. The executable is recognised by its size and by the
// shape of the table before a single byte is written, and a build that does not
// match is refused whole: writing a mount table into the wrong offset of the
// wrong executable corrupts it silently, and the player only finds out when the
// game crashes.
package clientmount

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/jeanluca/w2pp-openwyd/internal/mountbonus"
)

// Row is one adult lineage as the panel exported it (montarias-cliente.txt), or
// one temporary mount from the compiled table (TempRows).
type Row struct {
	Index  int16
	Bonus  mountbonus.Bonus
	AbsPvP int
	AbsPvE int
	Name   string
	// NoAbsorb leaves the catalog entry without the two absorption effects, so
	// its tooltip shows no "Absorção" lines. Only a temporary mount with no
	// absorption of its own sets it; every adult absorbs something.
	NoAbsorb bool
}

// TempRows is every temporary mount (3980-3994) as the server applies it: the
// compiled attribute row, and the absorption of the ones that have it — the
// cash-shop mounts the team decided on 2026-09-11 (mountbonus.TempExtra). The
// panel does not configure these yet, so they come from the code, which is
// also what the game reads.
func TempRows() []Row {
	var out []Row
	for i := int16(mountbonus.TempLo); i <= mountbonus.TempHi; i++ {
		b, _ := mountbonus.Default(i)
		r := Row{Index: i, Bonus: b, NoAbsorb: true}
		if ex, ok := mountbonus.TempExtra(i); ok {
			r.AbsPvP, r.AbsPvE, r.NoAbsorb = ex.AbsorbPvP, ex.AbsorbPvE, false
		}
		out = append(out, r)
	}
	return out
}

// tableRow is the row of the client's mount table an item sits in: the adults
// first, the temporary mounts right after them.
func tableRow(index int16) (int, bool) {
	switch {
	case mountbonus.IsAdult(index):
		return int(index - mountbonus.AdultLo), true
	case mountbonus.IsTemp(index):
		return adultRows + int(index-mountbonus.TempLo), true
	}
	return 0, false
}

// ParseTable reads the file the panel serves at /rates/montarias/cliente.txt:
// comment lines start with '#', every other line is
// indice;dano;magia;evasao_decimos;imunidade;absorcao_pvp;absorcao_pve;nome.
//
// Every value is checked against the same limits the game uses. A file that
// fails is refused as a whole, with the line that broke it — half a table
// written into a client is worse than none.
func ParseTable(r io.Reader) ([]Row, error) {
	var rows []Row
	seen := map[int16]bool{}
	sc := bufio.NewScanner(r)
	n := 0
	for sc.Scan() {
		n++
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		f := strings.SplitN(line, ";", 8)
		if len(f) < 7 {
			return nil, fmt.Errorf("clientmount: linha %d tem %d campos, esperava 8", n, len(f))
		}
		var v [7]int
		for i := range v {
			x, err := strconv.Atoi(strings.TrimSpace(f[i]))
			if err != nil {
				return nil, fmt.Errorf("clientmount: linha %d, campo %d não é número: %q", n, i+1, f[i])
			}
			v[i] = x
		}
		row := Row{
			Index:  int16(v[0]),
			Bonus:  mountbonus.Bonus{Attack: int16(v[1]), Magic: int16(v[2]), Evasion: int16(v[3]), Resist: int16(v[4])},
			AbsPvP: v[5], AbsPvE: v[6],
		}
		if len(f) == 8 {
			row.Name = strings.TrimSpace(f[7])
		}
		switch {
		case int(row.Index) != v[0] || !mountbonus.IsAdult(row.Index):
			return nil, fmt.Errorf("clientmount: linha %d: %d não é montaria adulta (%d..%d)", n, v[0], mountbonus.AdultLo, mountbonus.AdultHi)
		case !row.Bonus.Valid() || int(row.Bonus.Attack) != v[1] || int(row.Bonus.Magic) != v[2]:
			return nil, fmt.Errorf("clientmount: linha %d: atributos fora dos limites do jogo: %v", n, v[1:5])
		case v[5] < 0 || v[5] > 100 || v[6] < 0 || v[6] > 100:
			return nil, fmt.Errorf("clientmount: linha %d: absorção fora de 0..100: %d/%d", n, v[5], v[6])
		case seen[row.Index]:
			return nil, fmt.Errorf("clientmount: linha %d: a montaria %d aparece duas vezes", n, row.Index)
		}
		seen[row.Index] = true
		rows = append(rows, row)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("clientmount: ler a tabela: %w", err)
	}
	if len(rows) == 0 {
		return nil, errors.New("clientmount: a tabela não tem nenhuma montaria")
	}
	return rows, nil
}

// The client build these offsets belong to (WYD.exe 7662, 2 347 008 bytes). The
// table sits in .data at VA 0x61DCE0: 30 adult rows then the temporary ones, six
// int32 per row — Attack, Magic, Evasion, Resist, a movement tier, and a sixth
// column whose meaning is not known and which this package never writes.
const (
	exeSize        = 2347008
	tableOffset    = 0x21DCE0
	tableCols      = 6
	adultRows      = mountbonus.AdultHi - mountbonus.AdultLo + 1
	tempRows       = mountbonus.TempHi - mountbonus.TempLo + 1
	rowBytes       = tableCols * 4
	precedingWords = 12 // the twelve 100s of the table right before this one
)

// PatchExe writes rows into a copy of the WYD.exe image and returns it: the four
// attribute columns of each mount, and — once, idempotently — the two entries of
// the item-tooltip list that make the absorption show.
//
// Only the four columns the panel owns are written in the mount table; the
// movement tier and the sixth column are left exactly as the client shipped them.
func PatchExe(exe []byte, rows []Row) ([]byte, error) {
	if err := recogniseExe(exe); err != nil {
		return nil, err
	}
	out := bytes.Clone(exe)
	for _, r := range rows {
		row, ok := tableRow(r.Index)
		if !ok {
			return nil, fmt.Errorf("clientmount: %d não é montaria, e não tem linha na tabela do WYD.exe", r.Index)
		}
		at := tableOffset + row*rowBytes
		for c, v := range [4]int16{r.Bonus.Attack, r.Bonus.Magic, r.Bonus.Evasion, r.Bonus.Resist} {
			binary.LittleEndian.PutUint32(out[at+c*4:], uint32(int32(v)))
		}
	}
	if err := patchTooltipList(out); err != nil {
		return nil, err
	}
	return out, nil
}

// The item tooltip does not name its lines in code. It walks a list of 49
// (effect code → label pointer) pairs in .data — codes at VA 0x60F354, label
// pointers at VA 0x60F418 — and, for each effect the item has, prints the label
// and the value. The labels point into the strdef table the client loads at VA
// 0x109A4B8, 128 bytes per entry.
//
// The absorption rides on two effect codes nothing else uses, 62 and 63
// (EF_HWORDINDEX / EF_LWORDINDEX: no item in the catalog carries either, and the
// server never writes them). 62 was already on the list, with its own label.
// 63 was not, and has no label, so it takes over entry 28 — EF_SPECIALALL,
// "Aumento de Aprendizagem de Skill.", which no item in the client catalog
// carries either — keeping that entry's label slot, strdef 140. The seven
// entries 28..34 are then reordered so the two absorption lines come right after
// Ataque Mágico instead of in the middle of the attribute block.
const (
	tooltipCodes  = 0x20F354 // file offset of the code list (VA 0x60F354)
	tooltipLabels = 0x20F418 // file offset of the label-pointer list (VA 0x60F418)
	strdefVA      = 0x109A4B8
	efAbsPvP      = 62
	efAbsPvE      = 63
	labelAbsPvP   = 105 // strdef "Número de índice exclusivo" → "Absorção PvP (%)"
	labelAbsPvE   = 140 // strdef "Aumento de Aprendizagem de Skill." → "Absorção PvE (%)"
	listFirst     = 28
)

type tooltipEntry struct{ code, label uint32 }

func labelPtr(idx uint32) uint32 { return strdefVA + idx*128 }

// As the client shipped it, and as this package leaves it.
var (
	tooltipOriginal = [7]tooltipEntry{
		{74, labelPtr(140)}, {7, labelPtr(100)}, {8, labelPtr(101)}, {9, labelPtr(102)},
		{10, labelPtr(103)}, {60, labelPtr(104)}, {62, labelPtr(105)},
	}
	tooltipPatched = [7]tooltipEntry{
		{7, labelPtr(100)}, {8, labelPtr(101)}, {9, labelPtr(102)}, {10, labelPtr(103)},
		{60, labelPtr(104)}, {efAbsPvP, labelPtr(labelAbsPvP)}, {efAbsPvE, labelPtr(labelAbsPvE)},
	}
)

// patchTooltipList rewrites entries 28..34 of the tooltip list. It accepts the
// list as the client shipped it or as this package already left it — so a run
// over an already-generated exe is harmless — and refuses anything else.
func patchTooltipList(exe []byte) error {
	var got [7]tooltipEntry
	for k := range got {
		got[k] = tooltipEntry{
			binary.LittleEndian.Uint32(exe[tooltipCodes+4*(listFirst+k):]),
			binary.LittleEndian.Uint32(exe[tooltipLabels+4*(listFirst+k):]),
		}
	}
	if got != tooltipOriginal && got != tooltipPatched {
		return fmt.Errorf("clientmount: a lista de linhas do tooltip no WYD.exe não é a original nem a que o "+
			"gerador deixa (%v); alguém mexeu nela, e o gerador não vai adivinhar", got)
	}
	for k, e := range tooltipPatched {
		binary.LittleEndian.PutUint32(exe[tooltipCodes+4*(listFirst+k):], e.code)
		binary.LittleEndian.PutUint32(exe[tooltipLabels+4*(listFirst+k):], e.label)
	}
	return nil
}

// recogniseExe refuses anything that is not the build these offsets belong to.
//
// The four columns this package writes cannot be part of the fingerprint — they
// are the ones that change — so it checks what never does: the file size, the
// twelve 100s that end the table before this one, and the two columns nobody
// edits (a movement tier of 4-6 and a sixth column between 60 and 80 on every
// adult row).
func recogniseExe(exe []byte) error {
	if len(exe) != exeSize {
		return fmt.Errorf("clientmount: o WYD.exe tem %d bytes, e o gerador só conhece a build 7662 de %d — "+
			"escrever a tabela no lugar errado corrompe o executável", len(exe), exeSize)
	}
	word := func(off int) int32 { return int32(binary.LittleEndian.Uint32(exe[off:])) }
	for i := 1; i <= precedingWords; i++ {
		if word(tableOffset-4*i) != 100 {
			return errors.New("clientmount: o WYD.exe tem o tamanho certo mas não tem a tabela de montarias onde ela " +
				"deveria estar; é outra build, e o gerador se recusa a mexer nele")
		}
	}
	// The temporary rows right after the adults have the same shape (a tier of
	// 6, a sixth column of 65-75), and they are written too, so they are checked
	// too.
	for row := 0; row < adultRows+tempRows; row++ {
		at := tableOffset + row*rowBytes
		if tier, sixth := word(at+16), word(at+20); tier < 4 || tier > 6 || sixth < 60 || sixth > 80 {
			return fmt.Errorf("clientmount: a linha %d da tabela de montarias do WYD.exe não tem a forma esperada "+
				"(%d, %d); é outra build, e o gerador se recusa a mexer nele", row, tier, sixth)
		}
	}
	return nil
}

// The strdef.bin the client loads: 128-byte records, text in Windows-1252.
const (
	strdefSize   = 255992
	strdefRecord = 128
)

var strdefLabels = []struct {
	idx            int
	original, novo string
}{
	{labelAbsPvP, "N\xfamero de \xedndice exclusivo", "Absor\xe7\xe3o PvP (%)"},
	{labelAbsPvE, "Aumento de Aprendizagem de Skill.", "Absor\xe7\xe3o PvE (%)"},
}

// PatchStrdef returns a copy of UI\strdef.bin with the two absorption labels.
// Like the tooltip list, it accepts each record as shipped or as already
// patched, and refuses anything else rather than overwrite text it does not know.
func PatchStrdef(sd []byte) ([]byte, error) {
	if len(sd) != strdefSize {
		return nil, fmt.Errorf("clientmount: o strdef.bin tem %d bytes, esperava %d", len(sd), strdefSize)
	}
	out := bytes.Clone(sd)
	for _, l := range strdefLabels {
		rec := out[l.idx*strdefRecord : (l.idx+1)*strdefRecord]
		cur := string(bytes.TrimRight(rec, "\x00"))
		if cur != l.original && cur != l.novo {
			return nil, fmt.Errorf("clientmount: o rótulo %d do strdef.bin é %q, e o gerador só troca %q", l.idx, cur, l.original)
		}
		clear(rec)
		copy(rec, l.novo)
	}
	return out, nil
}

// The client ItemList.bin: 6500 records of 140 bytes under a flat XOR 0x5A, plus
// 4 trailing bytes the client does not check (they are the same in two catalogs
// whose contents differ). Twelve catalog effects per record at +80, each a
// (short code, short value) pair.
const (
	itemListSize    = 6500*140 + 4
	itemRecord      = 140
	itemEffects     = 80
	itemEffectSlots = 12
	itemXOR         = 0x5A
)

// PatchItemList writes each mount's absorption into its catalog entry as effects
// 62 (PvP) and 63 (PvE), which the tooltip list now shows as the two
// "Absorção" lines. The value is per lineage, not per mount, which is exactly
// how the server applies absorption (0035_mount_absorb).
//
// The server never reads these two effects: for the mount slot it takes its
// numbers from the mount tables and skips catalog effects altogether, so this
// only ever changes what the player reads.
func PatchItemList(il []byte, rows []Row) ([]byte, error) {
	if len(il) != itemListSize {
		return nil, fmt.Errorf("clientmount: o ItemList.bin tem %d bytes, esperava %d", len(il), itemListSize)
	}
	out := bytes.Clone(il)
	get := func(off int) int16 {
		return int16(uint16(out[off]^itemXOR) | uint16(out[off+1]^itemXOR)<<8)
	}
	put := func(off int, v int16) {
		out[off], out[off+1] = byte(uint16(v))^itemXOR, byte(uint16(v)>>8)^itemXOR
	}
	for _, r := range rows {
		if r.NoAbsorb {
			continue
		}
		rec := int(r.Index) * itemRecord
		if out[rec]^itemXOR == 0 {
			return nil, fmt.Errorf("clientmount: o ItemList.bin não tem o item %d", r.Index)
		}
		for _, ef := range []struct {
			code  int16
			value int
		}{{efAbsPvP, r.AbsPvP}, {efAbsPvE, r.AbsPvE}} {
			slot := -1
			for i := 0; i < itemEffectSlots; i++ {
				c := get(rec + itemEffects + 4*i)
				if c == ef.code {
					slot = i
					break
				}
				if c == 0 && slot < 0 {
					slot = i
				}
			}
			if slot < 0 {
				return nil, fmt.Errorf("clientmount: a montaria %d não tem espaço de efeito livre no ItemList.bin", r.Index)
			}
			put(rec+itemEffects+4*slot, ef.code)
			put(rec+itemEffects+4*slot+2, int16(ef.value))
		}
	}
	return out, nil
}
