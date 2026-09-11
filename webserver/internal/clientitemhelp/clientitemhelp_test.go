package clientitemhelp

import (
	"strings"
	"testing"
)

// arquivo é um itemhelp.dat em miniatura, com a forma do real: sem cabeçalho,
// índices em ordem, linhas de cor e texto.
const arquivo = "410\r\n" +
	"FFFFFFFF Ao_ser_utilizado_no_campo,\r\n" +
	"FFFFFFFF o_personagem_retorna_à_cidade.\r\n" +
	"3343\r\n" +
	"FFFF00FF [Item_Premium]\r\n" +
	"FFFFFFFF Dispensa_os_pontos_caóticos.\r\n"

func TestSetTrocaODoItemESoDele(t *testing.T) {
	out, err := Set([]byte(arquivo), 3343, []Linha{
		{Premium, "[Item_Premium]"},
		{Branco, "Um lugar infernal, mas com grandes recompensas."},
		{Vermelho, "Só venha se tiver coragem, NOOB!"},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	if !strings.Contains(got, "FFFFFFFF Um_lugar_infernal,_mas_com_grandes_recompensas.\r\n") {
		t.Errorf("o texto novo não saiu com os espaços como \"_\":\n%s", got)
	}
	// O "ó" sai em Windows-1252, um byte só, que é o que o cliente lê.
	if !strings.Contains(got, "FFFF0000 S\xf3_venha_se_tiver_coragem,_NOOB!\r\n") {
		t.Errorf("a linha vermelha não saiu certa:\n%q", got)
	}
	if strings.Contains(got, "Dispensa_os_pontos") {
		t.Error("a descrição antiga do item continuou no arquivo")
	}
	if !strings.Contains(got, "410\r\nFFFFFFFF Ao_ser_utilizado_no_campo,") {
		t.Errorf("o bloco do item vizinho mudou:\n%s", got)
	}
}

func TestSetInsereNaOrdem(t *testing.T) {
	out, err := Set([]byte(arquivo), 3222, []Linha{{Branco, "Chave do Inferno."}})
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	i3222 := strings.Index(got, "3222\r\n")
	i3343 := strings.Index(got, "3343\r\n")
	i410 := strings.Index(got, "410\r\n")
	if i3222 < 0 {
		t.Fatalf("o bloco novo não foi escrito:\n%s", got)
	}
	if i410 >= i3222 || i3222 >= i3343 {
		t.Errorf("ordem dos índices errada: 410=%d 3222=%d 3343=%d", i410, i3222, i3343)
	}
}

func TestSetSemLinhasApagaODescricao(t *testing.T) {
	out, err := Set([]byte(arquivo), 3343, nil)
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	if strings.Contains(got, "3343\r\n") || strings.Contains(got, "[Item_Premium]") {
		t.Errorf("o bloco não foi removido:\n%s", got)
	}
	if !strings.Contains(got, "410\r\n") {
		t.Error("apagar um bloco levou junto o do vizinho")
	}
}

// TestSetGravaEmCP1252: o cliente lê Windows-1252, e o acento tem de chegar
// como um byte só — em UTF-8 ele viraria dois e a tela mostraria lixo.
func TestSetGravaEmCP1252(t *testing.T) {
	out, err := Set([]byte(arquivo), 3222, []Linha{{Branco, "coração"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "FFFFFFFF cora\xe7\xe3o\r\n") {
		t.Errorf("acento não saiu em Windows-1252:\n%q", string(out))
	}
}
