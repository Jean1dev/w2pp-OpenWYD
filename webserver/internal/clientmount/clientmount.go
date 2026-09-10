// Package clientmount writes the mount numbers the staff panel decided into the
// two client files that show them: the attribute table inside WYD.exe, which
// the mount tooltip is drawn from, and itemhelp.dat, the per-item text where
// the absorption — a stat the client knows nothing about — can be written out.
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

// Row is one adult lineage as the panel exported it (montarias-cliente.txt).
type Row struct {
	Index  int16
	Bonus  mountbonus.Bonus
	AbsPvP int
	AbsPvE int
	Name   string
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
	rowBytes       = tableCols * 4
	precedingWords = 12 // the twelve 100s of the table right before this one
)

// PatchExe writes rows into a copy of the WYD.exe image and returns it. Only the
// four columns the panel owns are written; the movement tier and the sixth
// column are left exactly as the client shipped them.
func PatchExe(exe []byte, rows []Row) ([]byte, error) {
	if err := recogniseExe(exe); err != nil {
		return nil, err
	}
	out := bytes.Clone(exe)
	for _, r := range rows {
		at := tableOffset + int(r.Index-mountbonus.AdultLo)*rowBytes
		for c, v := range [4]int16{r.Bonus.Attack, r.Bonus.Magic, r.Bonus.Evasion, r.Bonus.Resist} {
			binary.LittleEndian.PutUint32(out[at+c*4:], uint32(int32(v)))
		}
	}
	return out, nil
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
	for row := 0; row < adultRows; row++ {
		at := tableOffset + row*rowBytes
		if tier, sixth := word(at+16), word(at+20); tier < 4 || tier > 6 || sixth < 60 || sixth > 80 {
			return fmt.Errorf("clientmount: a linha %d da tabela de montarias do WYD.exe não tem a forma esperada "+
				"(%d, %d); é outra build, e o gerador se recusa a mexer nele", row, tier, sixth)
		}
	}
	return nil
}

// helpLines is how many text lines follow an item id in itemhelp.dat — the
// shape of 426 of its 427 entries.
const helpLines = 9

// helpMarker is the start of the first line of every entry this package
// writes. It is how a later run tells its own entries (safe to rewrite) from
// text someone wrote by hand (never touched).
const helpMarker = "Absor\xe7\xe3o_PvP:"

// PatchItemHelp adds or rewrites one itemhelp.dat entry per row with the
// mount's absorption, the stat the client has no line for in its own tooltip.
//
// The file is text in the client's codepage (Windows-1252), CRLF, one item id
// on its own line followed by nine lines of "AARRGGBB text" with underscores for
// spaces. Entries it did not write itself are never touched: an id that already
// has help text someone wrote refuses the whole run rather than overwrite it.
func PatchItemHelp(help []byte, rows []Row) ([]byte, error) {
	lines := strings.Split(string(help), "\r\n")
	// Where each item's entry starts: the id line itself.
	start := map[int16]int{}
	for i, l := range lines {
		if id, err := strconv.Atoi(l); err == nil && id > 0 && id < 32768 {
			start[int16(id)] = i
		}
	}

	var appendix []string
	for _, r := range rows {
		entry := helpEntry(r)
		i, ok := start[r.Index]
		if !ok {
			appendix = append(appendix, entry...)
			continue
		}
		end := i + 1
		for end < len(lines) {
			if _, err := strconv.Atoi(lines[end]); err == nil {
				break
			}
			end++
		}
		if end-i-1 < 1 || !strings.Contains(lines[i+1], helpMarker) {
			return nil, fmt.Errorf("clientmount: o itemhelp.dat já tem um texto para a montaria %d que não foi o "+
				"gerador que escreveu; nada foi gravado, para não apagar esse texto", r.Index)
		}
		lines = append(lines[:i], append(entry, lines[end:]...)...)
		// Indices after i moved; recompute before the next rewrite.
		start = map[int16]int{}
		for j, l := range lines {
			if id, err := strconv.Atoi(l); err == nil && id > 0 && id < 32768 {
				start[int16(id)] = j
			}
		}
	}

	out := strings.Join(lines, "\r\n")
	if len(appendix) > 0 {
		if out != "" && !strings.HasSuffix(out, "\r\n") {
			out += "\r\n"
		}
		out += strings.Join(appendix, "\r\n")
	}
	return []byte(out), nil
}

// helpEntry is one item's lines: the id, two absorption lines, and blank lines
// up to the fixed nine. White is the colour the file uses for plain text.
func helpEntry(r Row) []string {
	e := []string{
		strconv.Itoa(int(r.Index)),
		fmt.Sprintf("FFFFFFFF %s_%d%%", helpMarker, r.AbsPvP),
		fmt.Sprintf("FFFFFFFF Absor\xe7\xe3o_PvE:_%d%%", r.AbsPvE),
	}
	for len(e) < helpLines+1 {
		e = append(e, "FFFFFFFF ")
	}
	return e
}
