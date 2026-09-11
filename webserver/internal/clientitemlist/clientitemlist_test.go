package clientitemlist

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/internal/itemeffect"
)

// Where the two effect tables meet, they must agree: a code that drifted here
// would write the client an effect the server reads as another.
func TestEffectCodesAgreeWithItemEffect(t *testing.T) {
	for _, name := range itemeffect.Names() {
		want, _ := itemeffect.EffectID(name)
		if got, ok := efCode[name]; !ok || got != int(want) {
			t.Errorf("%s: aqui %d (%v), itemeffect %d", name, got, ok, want)
		}
	}
}

const sample = "3212,Ba\xfa_da_Pedra_Secreta,2711.0,0.0.0.0.0,0,10000,0,0,0,EF_CLASS,255,EF_VOLATILE,210\r\n" +
	"\r\n" +
	"572,Manto_do_Campeao,1717.0,399.0.0.0.0,0,0,-32768,0,0,EF_CLASS,255,EF_AC,40\r\n"

func TestParseCSV(t *testing.T) {
	rows, err := ParseCSV(strings.NewReader(sample))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("linhas = %d, want 2 (a linha em branco é pulada)", len(rows))
	}
	bau := rows[0]
	if bau.Index != 3212 || !bytes.Equal(bau.Name, []byte("Ba\xfa_da_Pedra_Secreta")) || bau.Mesh != 2711 || bau.Price != 10000 {
		t.Errorf("baú lido como %+v", bau)
	}
	if len(bau.Effects) != 2 || bau.Effects[1] != (Effect{Code: 38, Value: 210}) {
		t.Errorf("efeitos do baú = %+v, want EF_CLASS 255 e EF_VOLATILE 210", bau.Effects)
	}
	if manto := rows[1]; manto.Pos != -32768 || manto.Req[0] != 399 {
		t.Errorf("manto lido como pos %d req %v", manto.Pos, manto.Req)
	}
}

// Names outlive the line they came from. The scanner reuses its buffer, so a
// name kept as a slice of it turns into a piece of some later line once the
// file is bigger than the buffer — which the real catalog is.
func TestParseCSVNamesSurviveTheBuffer(t *testing.T) {
	var sb strings.Builder
	for i := 1; i <= 3000; i++ {
		n := strconv.Itoa(i)
		sb.WriteString(n + ",Item_" + n + ",1.0,0.0.0.0.0,0,0,0,0,0,EF_CLASS,255\r\n")
	}
	rows, err := ParseCSV(strings.NewReader(sb.String()))
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		if want := "Item_" + strconv.Itoa(r.Index); string(r.Name) != want {
			t.Fatalf("item %d: nome %q, want %q", r.Index, r.Name, want)
		}
	}
}

func TestParseCSVRecusaOQueNaoCabe(t *testing.T) {
	long := strings.Repeat("A", 64)
	thirteen := strings.Repeat(",EF_CLASS,1", 13)
	for name, csv := range map[string]string{
		"efeito desconhecido": "10,X,1.0,0.0.0.0.0,0,0,0,0,0,EF_NAO_EXISTE,1\n",
		"13 efeitos":          "10,X,1.0,0.0.0.0.0,0,0,0,0,0" + thirteen + "\n",
		"nome grande demais":  "10," + long + ",1.0,0.0.0.0.0,0,0,0,0,0\n",
		"índice repetido":     "10,X,1.0,0.0.0.0.0,0,0,0,0,0\n10,Y,1.0,0.0.0.0.0,0,0,0,0,0\n",
		"índice fora":         "6500,X,1.0,0.0.0.0.0,0,0,0,0,0\n",
		"requisito curto":     "10,X,1.0,0.0.0,0,0,0,0,0\n",
		"arquivo vazio":       "\r\n\r\n",
	} {
		if _, err := ParseCSV(strings.NewReader(csv)); err == nil {
			t.Errorf("%s: aceito, want recusa", name)
		}
	}
}

func record(b []byte, index int) []byte {
	rec := make([]byte, recordSize)
	for i := range rec {
		rec[i] = b[index*recordSize+i] ^ xorKey
	}
	return rec
}

func TestBuild(t *testing.T) {
	rows, err := ParseCSV(strings.NewReader(sample))
	if err != nil {
		t.Fatal(err)
	}
	// The client's file: a "Cupom da Sorte" at 3212 with a visual effect, an
	// item at 100 the CSV does not have, and marked trailing bytes.
	base := bytes.Repeat([]byte{xorKey}, FileSize)
	setRec := func(index int, name string, vfx int16) {
		rec := make([]byte, recordSize)
		copy(rec, name)
		binary.LittleEndian.PutUint16(rec[offVFX:], uint16(vfx))
		for i := range rec {
			base[index*recordSize+i] = rec[i] ^ xorKey
		}
	}
	setRec(3212, "Cupom_da_Sorte", 7)
	setRec(100, "So_no_cliente", 0)
	copy(base[ItemCount*recordSize:], []byte{1, 2, 3, 4})

	out, err := Build(rows, base)
	if err != nil {
		t.Fatal(err)
	}
	bau := record(out, 3212)
	if name := string(bytes.TrimRight(bau[:nameSize], "\x00")); name != "Ba\xfa_da_Pedra_Secreta" {
		t.Errorf("nome do 3212 = %q, want o do servidor", name)
	}
	if vfx := binary.LittleEndian.Uint16(bau[offVFX:]); vfx != 7 {
		t.Errorf("efeito visual = %d, want 7 (vem do cliente)", vfx)
	}
	if code, val := binary.LittleEndian.Uint16(bau[offEffects+4:]), binary.LittleEndian.Uint16(bau[offEffects+6:]); code != 38 || val != 210 {
		t.Errorf("2º efeito = (%d,%d), want (38,210)", code, val)
	}
	if price := binary.LittleEndian.Uint32(bau[offPrice:]); price != 10000 {
		t.Errorf("preço = %d, want 10000", price)
	}
	if manto := record(out, 572); int16(binary.LittleEndian.Uint16(manto[offPos:])) != -32768 {
		t.Errorf("posição do manto errada")
	}
	if record(out, 100)[0] != 0 {
		t.Error("item que o servidor não tem continuou no cliente")
	}
	if !bytes.Equal(out[ItemCount*recordSize:], []byte{1, 2, 3, 4}) {
		t.Error("os 4 bytes finais do cliente não foram mantidos")
	}
	if _, err := Build(rows, base[:10]); err == nil {
		t.Error("aceitou um ItemList.bin de tamanho errado")
	}
}

// The server's own catalog encodes whole: every row the game loads fits the
// client record.
func TestServerCatalogEncodes(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	path := filepath.Join(filepath.Dir(file), "..", "..", "..", "Release", "Common", "ItemList.csv")
	f, err := os.Open(path)
	if err != nil {
		t.Skipf("ItemList.csv indisponível: %v", err)
	}
	defer f.Close()
	rows, err := ParseCSV(f)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) < 3000 {
		t.Errorf("só %d itens lidos do catálogo do servidor", len(rows))
	}
	if _, err := Build(rows, bytes.Repeat([]byte{xorKey}, FileSize)); err != nil {
		t.Fatal(err)
	}
}
