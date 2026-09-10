// Command montariacliente writes the mount numbers the staff panel decided into
// a copy of the client files that show them: WYD.exe (the attribute table and
// the tooltip's list of lines), ItemList.bin (each mount's absorption) and
// UI\strdef.bin (the labels of the two absorption lines). Alongside them it
// writes GamePatchItens.bin, the rarity tier of every weapon and armour
// (webserver/internal/clientrarity), which GamePatch.dll turns into the border
// and the "Item nível X" line of the tooltip.
//
//	montariacliente -tabela montarias-cliente.txt -cliente "C:\...\WYD-Cliente-Pronto" \
//	                -gamepatch client\gamepatch\out\GamePatch.dll
//
// The table is the file the panel serves at /rates/montarias/cliente.txt. The
// client folder is only READ: the generated files go to -saida (by default a
// "gerado-montarias" folder inside it), laid out like the client folder, so the
// originals survive and a running game — which locks WYD.exe — does not stop
// the run. -gamepatch copies the DLL that colours the lines alongside; without
// it the numbers are right and the lines stay white. Publishing the result to
// the players, through the launcher, is a separate step on purpose.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/jeanluca/w2pp-openwyd/webserver/internal/clientmount"
	"github.com/jeanluca/w2pp-openwyd/webserver/internal/clientrarity"
)

func main() {
	tabela := flag.String("tabela", "", "montarias-cliente.txt baixado do painel (/rates/montarias/cliente.txt)")
	cliente := flag.String("cliente", "", "pasta do cliente original, com WYD.exe, ItemList.bin e UI\\strdef.bin (só é lida)")
	saida := flag.String("saida", "", `pasta onde gravar os arquivos gerados (padrão: <cliente>\gerado-montarias)`)
	gamepatch := flag.String("gamepatch", "", "GamePatch.dll compilado (client/gamepatch); vai junto para a pasta de saída")
	flag.Parse()
	if *tabela == "" || *cliente == "" {
		flag.Usage()
		os.Exit(2)
	}
	if *saida == "" {
		*saida = filepath.Join(*cliente, "gerado-montarias")
	}
	if err := run(*tabela, *cliente, *saida, *gamepatch); err != nil {
		log.Fatal(err)
	}
}

type arquivo struct {
	nome  string // relativo à pasta do cliente
	dados []byte
	modo  os.FileMode
}

func run(tabela, cliente, saida, gamepatch string) error {
	f, err := os.Open(tabela)
	if err != nil {
		return fmt.Errorf("montariacliente: abrir a tabela: %w", err)
	}
	rows, err := clientmount.ParseTable(f)
	_ = f.Close() // lida por inteiro acima; um erro ao fechar um arquivo só de leitura não muda nada
	if err != nil {
		return err
	}

	ler := func(nome string) ([]byte, error) {
		b, err := os.ReadFile(filepath.Join(cliente, nome))
		if err != nil {
			return nil, fmt.Errorf("montariacliente: ler %s: %w", nome, err)
		}
		return b, nil
	}

	// Everything is computed before anything is written, so a refusal on one
	// file cannot leave the others generated alone — a client with the new
	// labels but the old list would print the wrong text on the wrong line.
	exe, err := ler("WYD.exe")
	if err != nil {
		return err
	}
	itemList, err := ler("ItemList.bin")
	if err != nil {
		return err
	}
	strdef, err := ler(filepath.Join("UI", "strdef.bin"))
	if err != nil {
		return err
	}
	novoExe, err := clientmount.PatchExe(exe, rows)
	if err != nil {
		return err
	}
	novoItemList, err := clientmount.PatchItemList(itemList, rows)
	if err != nil {
		return err
	}
	novoStrdef, err := clientmount.PatchStrdef(strdef)
	if err != nil {
		return err
	}
	itens, err := clientrarity.ReadItemList(itemList)
	if err != nil {
		return err
	}
	saidas := []arquivo{
		{"WYD.exe", novoExe, 0o755},
		{"ItemList.bin", novoItemList, 0o644},
		{filepath.Join("UI", "strdef.bin"), novoStrdef, 0o644},
		{"GamePatchItens.bin", clientrarity.Table(itens), 0o644},
	}
	if gamepatch != "" {
		dll, err := os.ReadFile(gamepatch)
		if err != nil {
			return fmt.Errorf("montariacliente: ler o GamePatch.dll: %w", err)
		}
		saidas = append(saidas, arquivo{"GamePatch.dll", dll, 0o755})
	}

	for _, a := range saidas {
		destino := filepath.Join(saida, a.nome)
		if err := os.MkdirAll(filepath.Dir(destino), 0o755); err != nil {
			return fmt.Errorf("montariacliente: criar %s: %w", filepath.Dir(destino), err)
		}
		if err := os.WriteFile(destino, a.dados, a.modo); err != nil {
			return fmt.Errorf("montariacliente: gravar %s: %w", a.nome, err)
		}
	}

	fmt.Printf("%d montarias gravadas em %s\n", len(rows), saida)
	for _, r := range rows {
		fmt.Printf("  %d %-22s dano %4d  magia %3d  evasão %d,%d%%  imunidade %3d  absorção PvP %d%% / PvE %d%%\n",
			r.Index, r.Name, r.Bonus.Attack, r.Bonus.Magic, r.Bonus.Evasion/10, r.Bonus.Evasion%10,
			r.Bonus.Resist, r.AbsPvP, r.AbsPvE)
	}
	porNivel := clientrarity.Count(itens)
	fmt.Print("raridade dos equipamentos (GamePatchItens.bin):")
	for t := clientrarity.Comum; t <= clientrarity.Divino; t++ {
		fmt.Printf("  %s %d", t, porNivel[t])
	}
	fmt.Println()
	if gamepatch == "" {
		fmt.Println("sem -gamepatch: os números vão certos, mas as linhas ficam brancas")
	}
	return nil
}
