package clientmount

import (
	"encoding/binary"
	"strings"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/internal/mountbonus"
)

// fakeExe builds an image with the shape of the 7662 client around the mount
// table: the right size, the twelve 100s before it, and the untouched columns
// on every adult row — the same fingerprint recogniseExe reads.
func fakeExe() []byte {
	b := make([]byte, exeSize)
	for i := 1; i <= precedingWords; i++ {
		binary.LittleEndian.PutUint32(b[tableOffset-4*i:], 100)
	}
	for row := 0; row < adultRows; row++ {
		at := tableOffset + row*rowBytes
		binary.LittleEndian.PutUint32(b[at+16:], 6)
		binary.LittleEndian.PutUint32(b[at+20:], 73)
	}
	return b
}

func col(b []byte, index int16, c int) int32 {
	return int32(binary.LittleEndian.Uint32(b[tableOffset+int(index-mountbonus.AdultLo)*rowBytes+c*4:]))
}

const tabelaAndaluzB = `# comentário do painel
2375;500;85;20;40;10;20;Andaluz B
`

func TestParseTable(t *testing.T) {
	rows, err := ParseTable(strings.NewReader(tabelaAndaluzB))
	if err != nil {
		t.Fatal(err)
	}
	want := Row{Index: 2375, Bonus: mountbonus.Bonus{Attack: 500, Magic: 85, Evasion: 20, Resist: 40}, AbsPvP: 10, AbsPvE: 20, Name: "Andaluz B"}
	if len(rows) != 1 || rows[0] != want {
		t.Errorf("rows = %+v, want [%+v]", rows, want)
	}
}

func TestParseTableRecusaAInteira(t *testing.T) {
	// One bad line refuses the file: half a table written into a client is
	// worse than none.
	for nome, txt := range map[string]string{
		"não é adulta":        "2340;1;1;0;0;25;25;cria\n",
		"imunidade > 100":     "2375;500;85;0;101;25;25;x\n",
		"evasão > 10%":        "2375;500;85;101;0;25;25;x\n",
		"absorção > 100":      "2375;500;85;0;0;101;25;x\n",
		"campo ilegível":      "2375;quinhentos;85;0;0;25;25;x\n",
		"linha curta":         "2375;500;85\n",
		"montaria repetida":   "2375;500;85;0;0;25;25;x\n2375;500;85;0;0;25;25;x\n",
		"tabela sem montaria": "# só comentário\n",
	} {
		if _, err := ParseTable(strings.NewReader(txt)); err == nil {
			t.Errorf("%s: aceito, want recusa", nome)
		}
	}
}

func TestPatchExeEscreveSoAsQuatroColunas(t *testing.T) {
	rows, _ := ParseTable(strings.NewReader(tabelaAndaluzB))
	exe := fakeExe()
	out, err := PatchExe(exe, rows)
	if err != nil {
		t.Fatal(err)
	}
	for c, want := range []int32{500, 85, 20, 40, 6, 73} {
		if got := col(out, 2375, c); got != want {
			t.Errorf("Andaluz B coluna %d = %d, want %d", c, got, want)
		}
	}
	if col(exe, 2375, 3) != 0 {
		t.Error("PatchExe alterou a imagem de entrada; devia devolver uma cópia")
	}
	// A lineage absent from the table is left exactly as it was.
	if col(out, 2374, 3) != 0 {
		t.Error("PatchExe mexeu numa montaria que não estava na tabela")
	}
}

func TestPatchExeRecusaOutraBuild(t *testing.T) {
	rows, _ := ParseTable(strings.NewReader(tabelaAndaluzB))

	if _, err := PatchExe(make([]byte, exeSize-1), rows); err == nil {
		t.Error("exe de tamanho diferente aceito")
	}
	semTabela := fakeExe()
	binary.LittleEndian.PutUint32(semTabela[tableOffset-4:], 7)
	if _, err := PatchExe(semTabela, rows); err == nil {
		t.Error("exe sem os 100 antes da tabela aceito")
	}
	linhaEstranha := fakeExe()
	binary.LittleEndian.PutUint32(linhaEstranha[tableOffset+5*rowBytes+16:], 9)
	if _, err := PatchExe(linhaEstranha, rows); err == nil {
		t.Error("exe com a coluna de movimento fora de 4..6 aceito")
	}
}

const helpOriginal = "410\r\nFFFFFFFF Ao_ser_utilizado\r\nFFFFFFFF \r\nFFFFFFFF \r\nFFFFFFFF \r\n" +
	"FFFFFFFF \r\nFFFFFFFF \r\nFFFFFFFF \r\nFFFFFFFF \r\nFFFFFFFF \r\n3987\r\nFFFF00FF [Item_Premium]\r\n" +
	"FFFFFFFF \r\nFFFFFFFF \r\nFFFFFFFF\r\nFFFFFFFF Montaria\r\nFFFFFFFF Item\r\nFFFF0000 Dura\xe7\xe3o\r\n" +
	"FFFF0000 Consumo\r\nFFFF0000 Some"

func TestPatchItemHelpAcrescentaEReescreveASuaPropria(t *testing.T) {
	rows, _ := ParseTable(strings.NewReader(tabelaAndaluzB))
	once, err := PatchItemHelp([]byte(helpOriginal), rows)
	if err != nil {
		t.Fatal(err)
	}
	s := string(once)
	if !strings.HasPrefix(s, helpOriginal+"\r\n2375\r\nFFFFFFFF Absor\xe7\xe3o_PvP:_10%\r\nFFFFFFFF Absor\xe7\xe3o_PvE:_20%\r\n") {
		t.Fatalf("entrada não foi acrescentada no fim, em CP1252:\n%q", s)
	}
	if n := strings.Count(s, "\r\n2375\r\n"); n != 1 {
		t.Errorf("a entrada 2375 aparece %d vezes", n)
	}
	lines := strings.Split(s, "\r\n")
	idx := -1
	for i, l := range lines {
		if l == "2375" {
			idx = i
		}
	}
	if len(lines)-idx-1 != helpLines {
		t.Errorf("entrada com %d linhas, want %d", len(lines)-idx-1, helpLines)
	}

	// A second run with new numbers rewrites the entry it wrote, in place.
	rows[0].AbsPvP = 35
	twice, err := PatchItemHelp(once, rows)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(twice), "\r\n2375\r\n") != 1 || !strings.Contains(string(twice), "PvP:_35%") ||
		strings.Contains(string(twice), "PvP:_10%") {
		t.Errorf("a segunda passada não reescreveu a própria entrada:\n%q", twice)
	}
	if !strings.HasPrefix(string(twice), helpOriginal) {
		t.Error("a segunda passada mexeu no texto que já existia")
	}
}

func TestPatchItemHelpNaoApagaTextoDeOutraPessoa(t *testing.T) {
	// 2375 already has help someone wrote by hand. Refusing is the rule: an entry the
	// generator did not write is someone's text, and overwriting it is a loss
	// nobody asked for.
	rows := []Row{{Index: 2375, AbsPvP: 10, AbsPvE: 20}}
	manual := helpOriginal + "\r\n2375\r\nFFFFFFFF Montaria_da_guilda\r\nFFFFFFFF \r\nFFFFFFFF \r\nFFFFFFFF \r\n" +
		"FFFFFFFF \r\nFFFFFFFF \r\nFFFFFFFF \r\nFFFFFFFF \r\nFFFFFFFF "
	if _, err := PatchItemHelp([]byte(manual), rows); err == nil {
		t.Error("sobrescreveu uma entrada escrita à mão")
	}
}
