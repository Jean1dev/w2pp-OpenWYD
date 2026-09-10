// Command montariacliente writes the mount numbers the staff panel decided into
// a copy of the client files: the attribute table inside WYD.exe (what the
// mount tooltip shows) and the absorption lines in itemhelp.dat.
//
//	montariacliente -tabela montarias-cliente.txt -cliente "C:\...\WYD-Cliente-Pronto"
//
// The table is the file the panel serves at /rates/montarias/cliente.txt. The
// client folder is only READ: the patched files go to -saida (by default a
// "gerado-montarias" folder inside it), so the originals survive and a running
// game — which locks WYD.exe — does not stop the run. Publishing the result to
// the players, through the launcher, is a separate step on purpose.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/jeanluca/w2pp-openwyd/webserver/internal/clientmount"
)

func main() {
	tabela := flag.String("tabela", "", "montarias-cliente.txt baixado do painel (/rates/montarias/cliente.txt)")
	cliente := flag.String("cliente", "", "pasta do cliente com WYD.exe e itemhelp.dat (só é lida)")
	saida := flag.String("saida", "", `pasta onde gravar os arquivos gerados (padrão: <cliente>\gerado-montarias)`)
	semAjuda := flag.Bool("sem-ajuda", false, "não mexer no itemhelp.dat, só na tabela do WYD.exe")
	flag.Parse()
	if *tabela == "" || *cliente == "" {
		flag.Usage()
		os.Exit(2)
	}
	if *saida == "" {
		*saida = filepath.Join(*cliente, "gerado-montarias")
	}
	if err := run(*tabela, *cliente, *saida, !*semAjuda); err != nil {
		log.Fatal(err)
	}
}

func run(tabela, cliente, saida string, ajuda bool) error {
	f, err := os.Open(tabela)
	if err != nil {
		return fmt.Errorf("montariacliente: abrir a tabela: %w", err)
	}
	rows, err := clientmount.ParseTable(f)
	_ = f.Close() // lida por inteiro acima; um erro ao fechar um arquivo só de leitura não muda nada
	if err != nil {
		return err
	}

	// Everything is computed before anything is written, so a refusal on the
	// second file cannot leave the first one generated alone.
	exe, err := os.ReadFile(filepath.Join(cliente, "WYD.exe"))
	if err != nil {
		return fmt.Errorf("montariacliente: ler o WYD.exe: %w", err)
	}
	novoExe, err := clientmount.PatchExe(exe, rows)
	if err != nil {
		return err
	}
	var novaAjuda []byte
	if ajuda {
		help, err := os.ReadFile(filepath.Join(cliente, "itemhelp.dat"))
		if err != nil {
			return fmt.Errorf("montariacliente: ler o itemhelp.dat: %w", err)
		}
		if novaAjuda, err = clientmount.PatchItemHelp(help, rows); err != nil {
			return err
		}
	}

	if err := os.MkdirAll(saida, 0o755); err != nil {
		return fmt.Errorf("montariacliente: criar a pasta de saída: %w", err)
	}
	if err := os.WriteFile(filepath.Join(saida, "WYD.exe"), novoExe, 0o755); err != nil {
		return fmt.Errorf("montariacliente: gravar o WYD.exe: %w", err)
	}
	if ajuda {
		if err := os.WriteFile(filepath.Join(saida, "itemhelp.dat"), novaAjuda, 0o644); err != nil {
			return fmt.Errorf("montariacliente: gravar o itemhelp.dat: %w", err)
		}
	}
	fmt.Printf("%d montarias gravadas em %s\n", len(rows), saida)
	for _, r := range rows {
		fmt.Printf("  %d %-22s dano %4d  magia %3d  evasão %d,%d%%  imunidade %3d  absorção %d/%d\n",
			r.Index, r.Name, r.Bonus.Attack, r.Bonus.Magic, r.Bonus.Evasion/10, r.Bonus.Evasion%10,
			r.Bonus.Resist, r.AbsPvP, r.AbsPvE)
	}
	return nil
}
