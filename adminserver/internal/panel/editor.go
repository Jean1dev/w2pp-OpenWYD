package panel

import (
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/audit"
	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/donate"
	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/gamedata"
	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/personagem"
)

// The character editor and the donate wallet — the two screens that hang off one
// account and did not exist before.
//
// They share a file because they share a shape: resolve the account from the
// path, resolve what is being looked at, refuse or write, audit, redirect.
//
// The rule worth knowing is in the personagem package: an item write is refused
// while the tmServer owns the character. Getting a character OUT of play is not
// this file's job — the operator does it from the /servidor page, which already
// kicks an account through the game control API.

// tmplFuncs are the template helpers.
//
// dict exists because html/template passes exactly one value to a nested
// template, and the shared "slot" and "abas-conta" blocks each need several. The
// alternative is a named Go type per block, which is more code for the same
// thing and puts view plumbing in a handler file.
var tmplFuncs = template.FuncMap{
	"dict": func(pares ...any) (map[string]any, error) {
		if len(pares)%2 != 0 {
			return nil, errors.New("dict: precisa de pares chave/valor")
		}
		m := make(map[string]any, len(pares)/2)
		for i := 0; i < len(pares); i += 2 {
			chave, ok := pares[i].(string)
			if !ok {
				return nil, fmt.Errorf("dict: chave %v não é texto", pares[i])
			}
			m[chave] = pares[i+1]
		}
		return m, nil
	},
}

// Audit actions. Spelled out rather than derived so a grep for what the panel
// can do finds them all in one place.
const (
	acaoItemGravado    = "ITEM_SET"
	acaoItemRemovido   = "ITEM_CLEAR"
	acaoAtributos      = "CHAR_STATS"
	acaoDonateAjustado = "DONATE_ADJUST"
	acaoMobEquip       = "MOB_EQUIP"
)

// itemView is one slot as the grid draws it.
type itemView struct {
	Slot       int
	Index      int16
	Nome       string
	IconURL    string
	Refino     int // EF_SANC value, shown as +N
	Efeitos    [3]efeitoView
	Vazio      bool
	Alcancavel bool // false for a Bolsa do Andarilho band the character has not unlocked
	Marcador   bool // the bag item itself, in slot 60/61
}

type efeitoView struct {
	Tipo  uint8
	Nome  string
	Valor uint8
}

// efSanc is EF_SANC (ItemEffect.h:100), an item's refine level. It is shown
// apart from the other effects because it is what a player calls the item's
// "+N", not a stat among stats.
const efSanc = 43

// nomesEfeito is the small subset the editor labels. The full dictionary lives
// in webserver/internal/itembrowser/effects.go; duplicating all of it here would
// couple the panel to a package it does not otherwise use, and an unlabelled
// effect still shows its number and value.
var nomesEfeito = map[uint8]string{
	2: "Dano", 3: "Defesa", 4: "HP", 5: "MP",
	7: "Força", 8: "Inteligência", 9: "Destreza", 10: "Constituição",
	11: "Maestria 1", 12: "Maestria 2", 13: "Maestria 3", 14: "Maestria 4",
	42: "Crítico", 43: "Refino", 45: "HP %", 46: "MP %",
	// Os que o sorteio de drop escreve na cópia (refine.Drop). Antes deste
	// sorteio existir nenhum item os carregava, então nenhum tinha rótulo aqui e
	// apareciam como "efeito 26". EF_UNIQUE é o marcador de espaço vazio: um
	// número dele não quer dizer nada, e o rótulo precisa dizer isso.
	26: "Velocidade de ataque", 59: "Sem efeito", 72: "Defesa 2", 73: "Dano 2",
	49: "Resist. fogo", 50: "Resist. gelo", 51: "Resist. sagrada", 52: "Resist. raio",
	53: "Defesa %", 54: "Todas as resistências", 60: "Ataque mágico",
	67: "Bônus de dano", 68: "Bônus mágico", 71: "Crítico 2", 74: "Todas as maestrias",
	87: "Nível do item", 112: "Tipo de alvo", 113: "Tipo de item",
}

func nomeEfeito(t uint8) string {
	if n, ok := nomesEfeito[t]; ok {
		return n
	}
	if t == 0 {
		return ""
	}
	return fmt.Sprintf("efeito %d", t)
}

// grade turns stored items into view rows, labelling them from the catalog.
// catalogo may be nil (no webServer): the grid still renders with indices.
func grade(itens []personagem.Item, catalogo map[int32]gamedata.Item, limite int, marcadores bool) []itemView {
	out := make([]itemView, 0, len(itens))
	for _, it := range itens {
		v := itemView{
			Slot:       it.Slot,
			Index:      it.Index,
			Vazio:      it.Vazio(),
			Alcancavel: limite <= 0 || it.Slot < limite,
			Marcador:   marcadores && (it.Slot == personagem.SlotMarcadorBolsa1 || it.Slot == personagem.SlotMarcadorBolsa2),
		}
		// The bag markers sit past the reachable band by design; they are not
		// "locked", they are the thing that does the unlocking.
		if v.Marcador {
			v.Alcancavel = true
		}
		efeitos := [3]struct {
			t uint8
			v uint8
		}{{it.Eff1, it.EffV1}, {it.Eff2, it.EffV2}, {it.Eff3, it.EffV3}}
		for i, e := range efeitos {
			v.Efeitos[i] = efeitoView{Tipo: e.t, Nome: nomeEfeito(e.t), Valor: e.v}
			if e.t == efSanc {
				v.Refino = int(e.v)
			}
		}
		if c, ok := catalogo[int32(it.Index)]; ok {
			v.Nome, v.IconURL = c.DisplayName, c.IconURL
		} else if !v.Vazio {
			v.Nome = fmt.Sprintf("item %d", it.Index)
		}
		out = append(out, v)
	}
	return out
}

// selecao is the slot the operator picked, carried in the query string.
//
// The picking is a plain link, not a click handler, because the panel serves a
// Content-Security-Policy of default-src 'none' with no script-src: inline
// JavaScript does not run here at all, and a grid that filled the form from an
// onclick would look alive locally and be dead in production. Every other screen
// in this panel is scripts-free for the same reason.
type selecao struct {
	Ativa   bool
	Destino string
	Slot    int
	Item    itemView
	// Busca is what the operator typed to look an item up, and Achados the
	// matches. Searching is a GET on this same page rather than a live dropdown
	// for the same reason the grid uses links: no script runs here.
	Busca   string
	Achados []itemAchado
	Demais  int // matches beyond the ones listed
}

// itemAchado is one catalog hit, with the link that puts it in the form.
type itemAchado struct {
	Index int32
	Nome  string
	Onde  string // where it equips, for telling apart items with similar names
	Href  string
}

// maxAchados caps the result list. The catalog has thousands of entries and this
// renders inside a side panel: past a couple dozen the list stops being a
// shortcut and becomes something to scroll through.
const maxAchados = 25

// selecaoDe reads ?onde=&slot= and resolves it against the loaded character, so
// the form comes back filled with what is actually in that slot.
func selecaoDe(r *http.Request, f personagem.Ficha, catalogo map[int32]gamedata.Item) selecao {
	onde := r.URL.Query().Get("onde")
	bruto := r.URL.Query().Get("slot")
	if onde == "" || bruto == "" {
		return selecao{}
	}
	slot, err := strconv.Atoi(bruto)
	if err != nil {
		return selecao{}
	}
	dest := personagem.Destino(onde)
	if _, conhecido := destinoValido(dest); !conhecido {
		return selecao{}
	}
	it := itemDoSlot(f, dest, slot)
	if it.Slot != slot {
		// itemDoSlot returns the zero value for an out-of-range slot; keep the
		// slot the operator asked for so the form still targets it.
		it = personagem.Item{Slot: slot}
	}
	// An index in the query wins over what is in the slot: it means the operator
	// just picked an item from the search and expects to see THAT, not what the
	// slot still holds.
	if q := r.URL.Query().Get("indice"); q != "" {
		if n, err := strconv.Atoi(q); err == nil && n >= 0 && n <= 32767 {
			it = personagem.Item{Slot: slot, Index: int16(n)}
		}
	}
	linhas := grade([]personagem.Item{it}, catalogo, 0, false)
	return selecao{Ativa: true, Destino: onde, Slot: slot, Item: linhas[0]}
}

// buscaItens looks the catalog up by name or index and builds the picker links.
//
// The index is matched too, because half the time the operator already knows the
// number and is only confirming which item it is.
func (h *Handler) buscaItens(r *http.Request, sel selecao) selecao {
	sel.Busca = strings.TrimSpace(r.URL.Query().Get("q"))
	if sel.Busca == "" || h.cfg.GameData == nil {
		return sel
	}
	sess, _ := staffFrom(r.Context())
	itens, err := h.cfg.GameData.Items(r.Context(), sess.AccountID, sel.Busca)
	if err != nil {
		h.cfg.Logger.Error("item search failed", "q", sel.Busca, "err", err)
		return sel
	}
	// Items() matches on name only; an all-digits query is almost certainly an
	// index, so resolve that too rather than answering "nada encontrado" for a
	// number that exists.
	if n, err := strconv.Atoi(sel.Busca); err == nil {
		if catalogo, e := h.cfg.GameData.ItemLookup(r.Context()); e == nil {
			if it, ok := catalogo[int32(n)]; ok {
				itens = append([]gamedata.Item{it}, itens...)
			}
		}
	}
	for i, it := range itens {
		if i >= maxAchados {
			sel.Demais = len(itens) - maxAchados
			break
		}
		sel.Achados = append(sel.Achados, itemAchado{
			Index: it.Index,
			Nome:  it.DisplayName,
			Onde:  strings.Join(it.Slots, ", "),
			Href: fmt.Sprintf("?onde=%s&slot=%d&indice=%d&q=%s",
				url.QueryEscape(sel.Destino), sel.Slot, it.Index, url.QueryEscape(sel.Busca)),
		})
	}
	return sel
}

// destinoValido reports whether dest is a container this package writes to.
func destinoValido(dest personagem.Destino) (personagem.Destino, bool) {
	switch dest {
	case personagem.DestinoEquip, personagem.DestinoCarry, personagem.DestinoCargo:
		return dest, true
	default:
		return "", false
	}
}

// editor renders one character's items and attributes.
func (h *Handler) editor(w http.ResponseWriter, r *http.Request) {
	nome, auth, ok := h.alvo(w, r)
	if !ok {
		return
	}
	ficha, ok := h.ficha(w, r, auth.ID)
	if !ok {
		return
	}

	catalogo := h.catalogo(r)
	limite := ficha.LimiteCarry()
	sel := h.buscaItens(r, selecaoDe(r, ficha, catalogo))

	h.render(w, "editor.html", struct {
		page
		Conta  string
		Ficha  personagem.Ficha
		Equip  []itemView
		Carry  []itemView
		Cargo  []itemView
		Limite int
		Sel    selecao
		// Editavel is the condition every form hangs off: the character has left
		// play, which is the moment its last save committed and the database
		// became the authority again.
		Editavel bool
		Aviso    string
	}{
		h.pageFor(r, "contas"), nome, ficha,
		grade(ficha.Equip, catalogo, 0, false),
		grade(ficha.Carry, catalogo, limite, true),
		grade(ficha.Cargo, catalogo, 0, false),
		limite, sel,
		!ficha.EmJogo(),
		r.URL.Query().Get("aviso"),
	})
}

// ficha loads the character named in the path and confirms it belongs to the
// account in the path. The ownership check is not decoration: without it the
// URL is an editor for any character, reachable by guessing a name.
// The character is addressed by its SLOT, not its name. Names repeat by design:
// an Arch and a Celestial carry their Mortal's name (0011_arch_name_tier_unique),
// so a name in the URL is ambiguous — opening a level-3 Celestial would land on
// the level-399 Mortal beside it. Slot is unique per account.
func (h *Handler) ficha(w http.ResponseWriter, r *http.Request, accountID int64) (personagem.Ficha, bool) {
	slot, err := strconv.Atoi(strings.TrimSpace(r.PathValue("char")))
	if err != nil || slot < 0 || slot > 3 {
		http.NotFound(w, r)
		return personagem.Ficha{}, false
	}
	ficha, err := h.cfg.Personagens.Carregar(r.Context(), accountID, slot)
	if errors.Is(err, personagem.ErrNaoEncontrado) {
		http.NotFound(w, r)
		return personagem.Ficha{}, false
	}
	if err != nil {
		h.cfg.Logger.Error("character load failed", "account", accountID, "slot", slot, "err", err)
		http.Error(w, "Erro ao carregar o personagem.", http.StatusInternalServerError)
		return personagem.Ficha{}, false
	}
	return ficha, true
}

// catalogo returns the item catalog, or nil when there is no webServer. A nil
// map is a working lookup that finds nothing, so callers need no special case.
func (h *Handler) catalogo(r *http.Request) map[int32]gamedata.Item {
	if h.cfg.GameData == nil {
		return nil
	}
	c, err := h.cfg.GameData.ItemLookup(r.Context())
	if err != nil {
		h.cfg.Logger.Error("item catalog failed", "err", err)
		return nil
	}
	return c
}

// redirectEditor bounces back to the editor with a message.
func (h *Handler) redirectEditor(w http.ResponseWriter, r *http.Request, conta string, slot int, msg string) {
	http.Redirect(w, r,
		fmt.Sprintf("/contas/%s/personagens/%d?aviso=%s", url.PathEscape(conta), slot, url.QueryEscape(msg)),
		http.StatusSeeOther)
}

// setSlot writes or clears one item slot.
func (h *Handler) setSlot(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil || !h.checkCSRF(w, r) {
		if err != nil {
			http.Error(w, "Formulário ilegível.", http.StatusBadRequest)
		}
		return
	}
	nome, auth, ok := h.alvo(w, r)
	if !ok {
		return
	}
	ficha, ok := h.ficha(w, r, auth.ID)
	if !ok {
		return
	}
	sess, _ := staffFrom(r.Context())

	dest := personagem.Destino(r.PostFormValue("destino"))
	slot, err := strconv.Atoi(r.PostFormValue("slot"))
	if err != nil {
		http.Error(w, "Slot inválido.", http.StatusBadRequest)
		return
	}

	antes := itemDoSlot(ficha, dest, slot)
	remover := r.PostFormValue("remover") == "1"

	var novo personagem.Item
	if !remover {
		novo, err = itemDoForm(r, slot)
		if err != nil {
			erroDeForma(w, err)
			return
		}
	}

	// A write into a Bolsa do Andarilho band the character has not unlocked
	// produces an item that exists in the database and cannot be reached in
	// game, which reads to the player exactly like it was taken.
	if !remover && dest == personagem.DestinoCarry && !slotAlcancavel(ficha, slot) {
		h.redirectEditor(w, r, nome, ficha.Slot,
			"Slot fora da área liberada: este personagem não tem a bolsa que abre essa faixa.")
		return
	}

	acao := acaoItemGravado
	if remover {
		acao = acaoItemRemovido
		err = h.cfg.Personagens.LimparSlot(r.Context(), ficha.ID, dest, slot)
	} else {
		err = h.cfg.Personagens.GravarSlot(r.Context(), ficha.ID, dest, slot, novo)
	}
	if !h.recusaEditor(w, r, nome, ficha.Slot, err) {
		return
	}

	if err := h.cfg.Audit.Write(r.Context(), audit.Record{
		ActorID: sess.AccountID, ActorRole: roleFrom(r.Context()),
		Action: acao, TargetID: auth.ID,
		Old: registroItem(ficha.Nome, dest, slot, antes),
		New: registroItem(ficha.Nome, dest, slot, novo),
	}); err != nil {
		h.auditoriaFalhou(w, err)
		return
	}
	h.redirectEditor(w, r, nome, ficha.Slot, "Slot gravado.")
}

// itemDoSlot reads the current occupant, for the audit "before".
func itemDoSlot(f personagem.Ficha, dest personagem.Destino, slot int) personagem.Item {
	var origem []personagem.Item
	switch dest {
	case personagem.DestinoEquip:
		origem = f.Equip
	case personagem.DestinoCarry:
		origem = f.Carry
	case personagem.DestinoCargo:
		origem = f.Cargo
	}
	if slot < 0 || slot >= len(origem) {
		return personagem.Item{}
	}
	return origem[slot]
}

func slotAlcancavel(f personagem.Ficha, slot int) bool {
	if slot == personagem.SlotMarcadorBolsa1 || slot == personagem.SlotMarcadorBolsa2 {
		return true // the markers are how the bands get unlocked
	}
	return slot < f.LimiteCarry()
}

// itemDoForm parses the slot form into an item.
// An empty index means "leave this slot empty", not a typo. Emptying a slot is
// the most common edit there is, and refusing it with "índice inválido" — as
// this did — sends the operator hunting for a number to type when what they
// wanted was nothing at all. Zero is the same thing: it is how an empty slot is
// stored.
func itemDoForm(r *http.Request, slot int) (personagem.Item, error) {
	bruto := strings.TrimSpace(r.PostFormValue("indice"))
	if bruto == "" {
		return personagem.Item{Slot: slot}, nil
	}
	indice, err := strconv.Atoi(bruto)
	if err != nil || indice < 0 || indice > 32767 {
		return personagem.Item{}, errors.New("índice de item inválido")
	}
	if indice == 0 {
		return personagem.Item{Slot: slot}, nil
	}
	it := personagem.Item{Slot: slot, Index: int16(indice)}
	pares := []struct {
		tipo, valor string
		destTipo    *uint8
		destValor   *uint8
	}{
		{"eff1", "effv1", &it.Eff1, &it.EffV1},
		{"eff2", "effv2", &it.Eff2, &it.EffV2},
		{"eff3", "effv3", &it.Eff3, &it.EffV3},
	}
	for _, p := range pares {
		t, err := campoByte(r, p.tipo)
		if err != nil {
			return personagem.Item{}, err
		}
		v, err := campoByte(r, p.valor)
		if err != nil {
			return personagem.Item{}, err
		}
		*p.destTipo, *p.destValor = t, v
	}
	return it, nil
}

// campoByte reads one 0..255 form field; empty means zero.
func campoByte(r *http.Request, nome string) (uint8, error) {
	raw := strings.TrimSpace(r.PostFormValue(nome))
	if raw == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 || n > 255 {
		return 0, fmt.Errorf("valor inválido em %s (use 0 a 255)", nome)
	}
	return uint8(n), nil
}

// registroItem is the audit shape for an item change: flat, named, and readable
// years from now without a Go type to consult.
func registroItem(char string, dest personagem.Destino, slot int, it personagem.Item) map[string]any {
	m := map[string]any{"personagem": char, "onde": string(dest), "slot": slot}
	if it.Vazio() {
		m["item"] = nil
		return m
	}
	m["item"] = it.Index
	m["efeitos"] = []any{
		map[string]any{"tipo": it.Eff1, "valor": it.EffV1},
		map[string]any{"tipo": it.Eff2, "valor": it.EffV2},
		map[string]any{"tipo": it.Eff3, "valor": it.EffV3},
	}
	return m
}

// setAtributos writes the attribute block.
func (h *Handler) setAtributos(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil || !h.checkCSRF(w, r) {
		if err != nil {
			http.Error(w, "Formulário ilegível.", http.StatusBadRequest)
		}
		return
	}
	nome, auth, ok := h.alvo(w, r)
	if !ok {
		return
	}
	ficha, ok := h.ficha(w, r, auth.ID)
	if !ok {
		return
	}
	sess, _ := staffFrom(r.Context())

	a, err := atributosDoForm(r)
	if err != nil {
		erroDeForma(w, err)
		return
	}
	antes, err := h.cfg.Personagens.GravarAtributos(r.Context(), ficha.ID, a)
	if !h.recusaEditor(w, r, nome, ficha.Slot, err) {
		return
	}
	if err := h.cfg.Audit.Write(r.Context(), audit.Record{
		ActorID: sess.AccountID, ActorRole: roleFrom(r.Context()),
		Action: acaoAtributos, TargetID: auth.ID,
		Old: registroAtributos(ficha.Nome, antes),
		New: registroAtributos(ficha.Nome, a),
	}); err != nil {
		h.auditoriaFalhou(w, err)
		return
	}
	h.redirectEditor(w, r, nome, ficha.Slot, "Atributos gravados.")
}

// atributosDoForm parses the attribute form.
//
// Each field carries its own ceiling because the columns are narrow: an int16
// that wraps turns "3000" into a negative attribute, which the game then reads
// as an enormous one. Refusing the value beats storing a wrapped one.
func atributosDoForm(r *http.Request) (personagem.Atributos, error) {
	var a personagem.Atributos
	campos := []struct {
		nome  string
		teto  int64
		grava func(int64)
	}{
		{"level", 999, func(v int64) { a.Level = int32(v) }},
		{"exp", 9223372036854775807, func(v int64) { a.Exp = v }},
		{"coin", 2147483647, func(v int64) { a.Coin = int32(v) }},
		{"str", 32767, func(v int64) { a.Str = int16(v) }},
		{"int", 32767, func(v int64) { a.Int = int16(v) }},
		{"dex", 32767, func(v int64) { a.Dex = int16(v) }},
		{"con", 32767, func(v int64) { a.Con = int16(v) }},
	}
	for _, c := range campos {
		raw := strings.TrimSpace(r.PostFormValue(c.nome))
		if raw == "" {
			continue
		}
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || n < 0 {
			return personagem.Atributos{}, fmt.Errorf("valor inválido em %s", c.nome)
		}
		if n > c.teto {
			return personagem.Atributos{}, fmt.Errorf("valor de %s acima do limite (%d)", c.nome, c.teto)
		}
		c.grava(n)
	}
	return a, nil
}

func registroAtributos(char string, a personagem.Atributos) map[string]any {
	return map[string]any{
		"personagem": char, "level": a.Level, "exp": a.Exp, "coin": a.Coin,
		"str": a.Str, "int": a.Int, "dex": a.Dex, "con": a.Con,
	}
}

// conceder queues an item for delivery to the account warehouse.
//
// This is the path that works for a character in play: nothing live is touched,
// the row waits in delivery_queue, and the tmServer applies it inside its loop
// at the next login.
func (h *Handler) recusaEditor(w http.ResponseWriter, r *http.Request, conta string, slot int, err error) bool {
	switch {
	case err == nil:
		return true
	case errors.Is(err, personagem.ErrEmJogo):
		h.redirectEditor(w, r, conta, slot,
			"Personagem em jogo: quem manda no inventário agora é o servidor. "+
				"Use “conceder ao baú”, ou espere ele sair.")
	case errors.Is(err, personagem.ErrSlotInvalido):
		http.Error(w, "Slot inválido para este destino.", http.StatusBadRequest)
	case errors.Is(err, personagem.ErrNaoEncontrado):
		http.NotFound(w, r)
	default:
		h.cfg.Logger.Error("character write failed", "account", conta, "slot", slot, "err", err)
		http.Error(w, "Erro ao gravar.", http.StatusInternalServerError)
	}
	return false
}

// erroDeForma answers a rejected form field.
//
// The Go error is lowercase and unpunctuated, as errors should be; this is the
// one place it becomes a sentence for a person to read, which is worth doing
// because "valor inválido em eff1" tells the moderator which box to fix and a
// generic "dados inválidos" does not.
func erroDeForma(w http.ResponseWriter, err error) {
	msg := err.Error()
	if r, n := utf8.DecodeRuneInString(msg); n > 0 {
		msg = string(unicode.ToUpper(r)) + msg[n:]
	}
	http.Error(w, msg+".", http.StatusBadRequest)
}

// auditoriaFalhou answers a failed audit write.
//
// The change is already committed at this point, so this reports a gap in the
// log rather than a failed action — and it is deliberately loud: an unlogged
// administrative write is the thing the log exists to make impossible.
func (h *Handler) auditoriaFalhou(w http.ResponseWriter, err error) {
	h.cfg.Logger.Error("audit write failed after a committed change", "err", err)
	http.Error(w, "A alteração foi gravada, mas não entrou na auditoria. Avise um admin.",
		http.StatusInternalServerError)
}

// --- Donate ---

// carteira shows the donate balance and its timeline.
func (h *Handler) carteira(w http.ResponseWriter, r *http.Request) {
	nome, auth, ok := h.alvo(w, r)
	if !ok {
		return
	}
	saldo, err := h.cfg.Carteira.Saldo(r.Context(), auth.ID)
	if err != nil {
		h.cfg.Logger.Error("donate balance failed", "account", nome, "err", err)
		http.Error(w, "Erro ao carregar o saldo.", http.StatusInternalServerError)
		return
	}
	eventos, err := h.cfg.Carteira.Historico(r.Context(), auth.ID, 200)
	if err != nil {
		// The balance alone is still worth showing; the timeline is the extra.
		h.cfg.Logger.Error("donate history failed", "account", nome, "err", err)
	}
	h.render(w, "donate.html", struct {
		page
		Conta   string
		Saldo   int32
		Eventos []donate.Evento
		Aviso   string
	}{h.pageFor(r, "contas"), nome, saldo, eventos, r.URL.Query().Get("aviso")})
}

// ajustarDonate credits or debits the wallet.
func (h *Handler) ajustarDonate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil || !h.checkCSRF(w, r) {
		if err != nil {
			http.Error(w, "Formulário ilegível.", http.StatusBadRequest)
		}
		return
	}
	nome, auth, ok := h.alvo(w, r)
	if !ok {
		return
	}
	sess, _ := staffFrom(r.Context())

	delta, err := strconv.Atoi(strings.TrimSpace(r.PostFormValue("valor")))
	if err != nil || delta < -1000000 || delta > 1000000 {
		http.Error(w, "Informe um valor entre -1.000.000 e 1.000.000.", http.StatusBadRequest)
		return
	}
	motivo := strings.TrimSpace(r.PostFormValue("motivo"))

	saldo, err := h.cfg.Carteira.Ajustar(r.Context(), sess.AccountID, auth.ID, int32(delta), motivo)
	switch {
	case errors.Is(err, donate.ErrMotivoVazio):
		h.redirectDonate(w, r, nome, "Informe o motivo do ajuste.")
		return
	case errors.Is(err, donate.ErrSaldoInsuficiente):
		h.redirectDonate(w, r, nome, "O débito é maior que o saldo da conta.")
		return
	case errors.Is(err, donate.ErrNaoEncontrado):
		http.NotFound(w, r)
		return
	case err != nil:
		h.cfg.Logger.Error("donate adjust failed", "account", nome, "err", err)
		http.Error(w, "Erro ao ajustar o saldo.", http.StatusInternalServerError)
		return
	}

	// donate.Ajustar already wrote the wallet's own log (donate_shop_audit), so
	// the account is auditable from either side; this entry is what makes the
	// change visible in the panel's timeline alongside role and VIP changes.
	if err := h.cfg.Audit.Write(r.Context(), audit.Record{
		ActorID: sess.AccountID, ActorRole: roleFrom(r.Context()),
		Action: acaoDonateAjustado, TargetID: auth.ID,
		New: map[string]any{"delta": delta, "saldo": saldo, "motivo": motivo},
	}); err != nil {
		h.auditoriaFalhou(w, err)
		return
	}
	h.redirectDonate(w, r, nome, fmt.Sprintf("Saldo ajustado. Agora: %d.", saldo))
}

func (h *Handler) redirectDonate(w http.ResponseWriter, r *http.Request, nome, msg string) {
	http.Redirect(w, r, "/contas/"+url.PathEscape(nome)+"/donate?aviso="+url.QueryEscape(msg),
		http.StatusSeeOther)
}
