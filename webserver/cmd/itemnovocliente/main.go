// Command itemnovocliente põe no cliente um item que o jogo ainda não sabe
// desenhar nem descrever: grava o ícone numa célula livre do atlas, aponta a
// tabela itemicon.bin para ela e escreve a descrição no itemhelp.dat.
//
//	itemnovocliente -cliente "C:\...\WYD-Cliente-Pronto" -item 3222 \
//	                -icone chave.bmp \
//	                -linha "[Item_Premium]:premium" \
//	                -linha "Um lugar infernal, mas com grandes recompensas." \
//	                -linha "Só venha se tiver coragem, NOOB!:vermelho"
//
// A pasta do cliente é só LIDA: os arquivos alterados vão para -saida (por
// padrão uma pasta "gerado-item" dentro dela), com a mesma disposição, para os
// originais sobreviverem e um jogo aberto — que segura os arquivos — não
// atrapalhar. Publicar para os jogadores, pelo launcher, é um passo à parte de
// propósito, como no montariacliente.
//
// O ItemList.bin (nome, preço e efeitos) não sai daqui: ele vem do catálogo do
// servidor, pelo montariacliente -catalogo.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/jeanluca/w2pp-openwyd/webserver/internal/clientitemhelp"
	"github.com/jeanluca/w2pp-openwyd/webserver/internal/clientitemlist"
	"github.com/jeanluca/w2pp-openwyd/webserver/internal/itemicons"
)

type linhas []string

func (l *linhas) String() string     { return strings.Join(*l, " | ") }
func (l *linhas) Set(v string) error { *l = append(*l, v); return nil }

func main() {
	cliente := flag.String("cliente", "", "pasta do cliente original (só é lida)")
	saida := flag.String("saida", "", `pasta onde gravar os arquivos alterados (padrão: <cliente>\gerado-item)`)
	item := flag.Int("item", 0, "índice do item no catálogo")
	icone := flag.String("icone", "", "BMP de até 35x35 com o ícone do item (opcional)")
	catalogo := flag.String("catalogo", "", "ItemList.csv do servidor: o ItemList.bin do cliente é reescrito a partir dele")
	var desc linhas
	flag.Var(&desc, "linha", `uma linha da descrição; "texto:vermelho" ou "texto:premium" mudam a cor (repita a opção)`)
	flag.Parse()
	if *cliente == "" || *item <= 0 {
		flag.Usage()
		os.Exit(2)
	}
	if *saida == "" {
		*saida = filepath.Join(*cliente, "gerado-item")
	}
	if err := run(*cliente, *saida, *item, *icone, *catalogo, desc); err != nil {
		log.Fatal(err)
	}
}

func run(cliente, saida string, item int, icone, catalogo string, desc linhas) error {
	// Tudo o que será tocado é copiado antes, para o comando nunca escrever na
	// pasta do cliente.
	copiar := []string{"itemicon.bin", "itemhelp.dat"}
	if catalogo != "" {
		copiar = append(copiar, "ItemList.bin")
	}
	if icone != "" {
		atlas, err := filepath.Glob(filepath.Join(cliente, "UI", "itemicon*.wyt"))
		if err != nil {
			return fmt.Errorf("itemnovocliente: procurar os atlas: %w", err)
		}
		for _, a := range atlas {
			copiar = append(copiar, filepath.Join("UI", filepath.Base(a)))
		}
	}
	for _, nome := range copiar {
		if err := copia(filepath.Join(cliente, nome), filepath.Join(saida, nome)); err != nil {
			return err
		}
	}

	if icone != "" {
		id, err := itemicons.SetIcon(saida, item, icone)
		if err != nil {
			return err
		}
		fmt.Printf("ícone %d gravado para o item %d\n", id, item)
	}
	if len(desc) > 0 {
		caminho := filepath.Join(saida, "itemhelp.dat")
		atual, err := os.ReadFile(caminho)
		if err != nil {
			return fmt.Errorf("itemnovocliente: ler o itemhelp.dat: %w", err)
		}
		novo, err := clientitemhelp.Set(atual, item, converte(desc))
		if err != nil {
			return err
		}
		if err := os.WriteFile(caminho, novo, 0o644); err != nil {
			return fmt.Errorf("itemnovocliente: gravar o itemhelp.dat: %w", err)
		}
		fmt.Printf("descrição com %d linha(s) escrita para o item %d\n", len(desc), item)
	}
	if catalogo != "" {
		if err := reescreveItemList(catalogo, filepath.Join(saida, "ItemList.bin")); err != nil {
			return err
		}
	}
	fmt.Printf("arquivos gerados em %s\n", saida)
	return nil
}

// reescreveItemList põe o catálogo do servidor dentro do ItemList.bin do
// cliente, que é como o nome e o preço de um item novo chegam à bolsa. É o
// mesmo caminho que o montariacliente usa com -catalogo, aqui sem as montarias.
func reescreveItemList(catalogo, destino string) error {
	f, err := os.Open(catalogo)
	if err != nil {
		return fmt.Errorf("itemnovocliente: abrir o catálogo: %w", err)
	}
	linhas, err := clientitemlist.ParseCSV(f)
	_ = f.Close() // lido por inteiro acima; fechar um arquivo só de leitura não muda nada
	if err != nil {
		return err
	}
	base, err := os.ReadFile(destino)
	if err != nil {
		return fmt.Errorf("itemnovocliente: ler o ItemList.bin: %w", err)
	}
	novo, err := clientitemlist.Build(linhas, base)
	if err != nil {
		return err
	}
	if err := os.WriteFile(destino, novo, 0o644); err != nil {
		return fmt.Errorf("itemnovocliente: gravar o ItemList.bin: %w", err)
	}
	fmt.Printf("ItemList.bin reescrito com %d itens do catálogo do servidor\n", len(linhas))
	return nil
}

// converte lê o sufixo de cor de cada linha: sem sufixo é o branco do corpo.
func converte(desc linhas) []clientitemhelp.Linha {
	out := make([]clientitemhelp.Linha, 0, len(desc))
	for _, texto := range desc {
		cor := clientitemhelp.Branco
		if i := strings.LastIndex(texto, ":"); i >= 0 {
			switch strings.ToLower(texto[i+1:]) {
			case "vermelho":
				cor, texto = clientitemhelp.Vermelho, texto[:i]
			case "premium":
				cor, texto = clientitemhelp.Premium, texto[:i]
			}
		}
		out = append(out, clientitemhelp.Linha{Cor: cor, Texto: texto})
	}
	return out
}

func copia(origem, destino string) error {
	dados, err := os.ReadFile(origem)
	if err != nil {
		return fmt.Errorf("itemnovocliente: ler %s: %w", origem, err)
	}
	if err := os.MkdirAll(filepath.Dir(destino), 0o755); err != nil {
		return fmt.Errorf("itemnovocliente: criar %s: %w", filepath.Dir(destino), err)
	}
	if err := os.WriteFile(destino, dados, 0o644); err != nil {
		return fmt.Errorf("itemnovocliente: gravar %s: %w", destino, err)
	}
	return nil
}
