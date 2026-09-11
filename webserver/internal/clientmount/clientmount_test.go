package clientmount

import (
	"bytes"
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
	for row := 0; row < adultRows+tempRows; row++ {
		at := tableOffset + row*rowBytes
		binary.LittleEndian.PutUint32(b[at+16:], 6)
		binary.LittleEndian.PutUint32(b[at+20:], 73)
	}
	return withOriginalTooltipList(b)
}

func col(b []byte, index int16, c int) int32 {
	row, _ := tableRow(index)
	return int32(binary.LittleEndian.Uint32(b[tableOffset+row*rowBytes+c*4:]))
}

// TestTempRowsSaoAsDaLoja: the temporary rows go into the client from the
// compiled table, the shop mounts with their absorption and the rest with none.
func TestTempRowsSaoAsDaLoja(t *testing.T) {
	rows := TempRows()
	if len(rows) != tempRows {
		t.Fatalf("%d linhas temporárias, want %d", len(rows), tempRows)
	}
	out, err := PatchExe(fakeExe(), rows)
	if err != nil {
		t.Fatal(err)
	}
	// Klazedale 3982: 250/45, no evasion or immunity, the untouched columns kept.
	for c, want := range []int32{250, 45, 0, 0, 6, 73} {
		if got := col(out, 3982, c); got != want {
			t.Errorf("Klazedale coluna %d = %d, want %d", c, got, want)
		}
	}
	// Row 30 is 3980, right after the last adult — not on top of it.
	if col(out, 2389, 0) != 0 || col(out, 3980, 0) != 150 {
		t.Errorf("a Shire caiu na linha errada: Pantera %d, Shire %d", col(out, 2389, 0), col(out, 3980, 0))
	}
	porIndice := map[int16]Row{}
	for _, r := range rows {
		porIndice[r.Index] = r
	}
	if r := porIndice[3990]; r.NoAbsorb || r.AbsPvE != 35 || r.AbsPvP != 0 {
		t.Errorf("Tigre de Fogo: %+v, want absorção PvE 35", r)
	}
	if r := porIndice[3989]; !r.NoAbsorb {
		t.Errorf("Gullfaxi ganhou linha de absorção: %+v", r)
	}
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

// withOriginalTooltipList puts the tooltip list entries 28..34 as the client
// ships them into an image.
func withOriginalTooltipList(b []byte) []byte {
	for k, e := range tooltipOriginal {
		binary.LittleEndian.PutUint32(b[tooltipCodes+4*(listFirst+k):], e.code)
		binary.LittleEndian.PutUint32(b[tooltipLabels+4*(listFirst+k):], e.label)
	}
	return b
}

func TestPatchExeAbreAsDuasLinhasDeAbsorcao(t *testing.T) {
	rows, _ := ParseTable(strings.NewReader(tabelaAndaluzB))
	out, err := PatchExe(withOriginalTooltipList(fakeExe()), rows)
	if err != nil {
		t.Fatal(err)
	}
	// Magia, then PvP, then PvE: the two absorption lines together, after Ataque
	// Mágico — the order players read in game.
	for k, want := range []uint32{7, 8, 9, 10, 60, efAbsPvP, efAbsPvE} {
		if got := binary.LittleEndian.Uint32(out[tooltipCodes+4*(listFirst+k):]); got != want {
			t.Errorf("entrada %d = código %d, want %d", listFirst+k, got, want)
		}
	}
	if got := binary.LittleEndian.Uint32(out[tooltipLabels+4*(listFirst+6):]); got != labelPtr(labelAbsPvE) {
		t.Errorf("rótulo da linha PvE = 0x%X, want o do strdef %d", got, labelAbsPvE)
	}
	// A second run over the generated image is harmless.
	if _, err := PatchExe(out, rows); err != nil {
		t.Errorf("rodar de novo sobre o exe gerado falhou: %v", err)
	}
}

func TestPatchExeRecusaListaDesconhecida(t *testing.T) {
	rows, _ := ParseTable(strings.NewReader(tabelaAndaluzB))
	exe := withOriginalTooltipList(fakeExe())
	binary.LittleEndian.PutUint32(exe[tooltipCodes+4*listFirst:], 99)
	if _, err := PatchExe(exe, rows); err == nil {
		t.Error("aceitou uma lista do tooltip que não é a original nem a gerada")
	}
}

func fakeStrdef() []byte {
	b := make([]byte, strdefSize)
	for _, l := range strdefLabels {
		copy(b[l.idx*strdefRecord:], l.original)
	}
	return b
}

func TestPatchStrdefTrocaSoOsDoisRotulos(t *testing.T) {
	sd := fakeStrdef()
	copy(sd[3*strdefRecord:], "outro texto qualquer")
	out, err := PatchStrdef(sd)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(bytes.TrimRight(out[labelAbsPvP*strdefRecord:(labelAbsPvP+1)*strdefRecord], "\x00")); got != "Absor\xe7\xe3o PvP (%)" {
		t.Errorf("rótulo PvP = %q", got)
	}
	if got := string(bytes.TrimRight(out[labelAbsPvE*strdefRecord:(labelAbsPvE+1)*strdefRecord], "\x00")); got != "Absor\xe7\xe3o PvE (%)" {
		t.Errorf("rótulo PvE = %q", got)
	}
	if !bytes.Equal(out[3*strdefRecord:4*strdefRecord], sd[3*strdefRecord:4*strdefRecord]) {
		t.Error("mexeu num rótulo que não era dele")
	}
	if _, err := PatchStrdef(out); err != nil {
		t.Errorf("rodar de novo sobre o strdef gerado falhou: %v", err)
	}
	outro := fakeStrdef()
	copy(outro[labelAbsPvE*strdefRecord:], "Texto que alguem pos")
	if _, err := PatchStrdef(outro); err == nil {
		t.Error("sobrescreveu um rótulo que não era o original")
	}
}

// fakeItemList builds a client catalog with the Andaluz B entry as shipped: a
// name and EF_CLASS 255 in slot 0, all under XOR 0x5A.
func fakeItemList() []byte {
	b := make([]byte, itemListSize)
	for i := range b {
		b[i] = itemXOR
	}
	rec := 2375 * itemRecord
	for i, c := range []byte("Andaluz_B") {
		b[rec+i] = c ^ itemXOR
	}
	b[rec+itemEffects] = 18 ^ itemXOR    // EF_CLASS
	b[rec+itemEffects+2] = 255 ^ itemXOR // todas as classes
	return b
}

func efeitoDoItem(b []byte, index, slot int) (code, value int16) {
	off := index*itemRecord + itemEffects + 4*slot
	return int16(uint16(b[off]^itemXOR) | uint16(b[off+1]^itemXOR)<<8),
		int16(uint16(b[off+2]^itemXOR) | uint16(b[off+3]^itemXOR)<<8)
}

func TestPatchItemListGravaAAbsorcaoNosEfeitosLivres(t *testing.T) {
	rows, _ := ParseTable(strings.NewReader(tabelaAndaluzB))
	out, err := PatchItemList(fakeItemList(), rows)
	if err != nil {
		t.Fatal(err)
	}
	if c, v := efeitoDoItem(out, 2375, 0); c != 18 || v != 255 {
		t.Errorf("slot 0 = (%d,%d), o EF_CLASS 255 original sumiu", c, v)
	}
	if c, v := efeitoDoItem(out, 2375, 1); c != efAbsPvP || v != 10 {
		t.Errorf("slot 1 = (%d,%d), want (62,10) — absorção PvP", c, v)
	}
	if c, v := efeitoDoItem(out, 2375, 2); c != efAbsPvE || v != 20 {
		t.Errorf("slot 2 = (%d,%d), want (63,20) — absorção PvE", c, v)
	}

	// New numbers from the panel rewrite the same slots instead of piling up.
	rows[0].AbsPvP, rows[0].AbsPvE = 35, 5
	twice, err := PatchItemList(out, rows)
	if err != nil {
		t.Fatal(err)
	}
	if c, v := efeitoDoItem(twice, 2375, 1); c != efAbsPvP || v != 35 {
		t.Errorf("segunda passada: slot 1 = (%d,%d), want (62,35)", c, v)
	}
	if c, _ := efeitoDoItem(twice, 2375, 3); c != 0 {
		t.Errorf("segunda passada abriu um slot novo (%d) em vez de reescrever", c)
	}
}

func TestPatchItemListRecusaItemInexistente(t *testing.T) {
	rows := []Row{{Index: 2374, AbsPvP: 10, AbsPvE: 20}} // 2374 não está no catálogo falso
	if _, err := PatchItemList(fakeItemList(), rows); err == nil {
		t.Error("gravou num registro vazio do ItemList")
	}
}

// TestPatchItemListPulaQuemNaoAbsorve: a temporary mount with no absorption of
// its own gets no "Absorção" effects — the catalog entry, not even an empty
// record, is touched only for the ones that have them.
func TestPatchItemListPulaQuemNaoAbsorve(t *testing.T) {
	rows := []Row{{Index: 3989, NoAbsorb: true}} // Gullfaxi: não está no catálogo falso
	if _, err := PatchItemList(fakeItemList(), rows); err != nil {
		t.Errorf("uma montaria sem absorção fez o ItemList ser recusado: %v", err)
	}
}
