// Package donate is the staff panel's view of an account's donate wallet: the
// balance, the timeline of everything that moved it, and the manual adjustment.
//
// Nothing here needed new recording. Top-ups, purchases and staff credits were
// already written by the web services (donate_topup_order and donate_shop_audit,
// migrations 0008 and 0010) — what was missing was somewhere to read them
// together. That is why this package is almost entirely SELECTs.
//
// The balance write is safe in a way item writes are not: it is a partial UPDATE
// of one account column, and no tmServer save rewrites it (the character save
// touches character rows and items; the cargo save touches cargo_coin). The
// single-owner rule that forbids editing a live character's inventory does not
// reach the wallet.
package donate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNaoEncontrado is returned when the account does not exist.
var ErrNaoEncontrado = errors.New("donate: conta não encontrada")

// ErrMotivoVazio refuses an adjustment with no stated reason. A credit with no
// reason is indistinguishable from an error later, and this is the log people
// consult when they suspect one.
var ErrMotivoVazio = errors.New("donate: motivo obrigatório")

// ErrSaldoInsuficiente refuses a debit larger than the balance.
var ErrSaldoInsuficiente = errors.New("donate: saldo insuficiente")

// Store reads and writes the donate wallet.
type Store struct{ pool *pgxpool.Pool }

// New builds a Store over the shared pool.
func New(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// Tipo classifies a timeline entry, so the screen can show direction and origin
// without re-deriving them from the text.
type Tipo string

const (
	TipoRecarga  Tipo = "recarga"  // donate_topup_order, paid
	TipoPendente Tipo = "pendente" // donate_topup_order, never confirmed
	TipoCompra   Tipo = "compra"   // donate_shop_audit, purchase
	TipoAjuste   Tipo = "ajuste"   // donate_shop_audit, credit_balance
)

// Evento is one line of the wallet timeline.
type Evento struct {
	Tipo     Tipo
	Quando   time.Time
	Creditos int64 // signed: positive adds, negative spends, zero for a dead top-up
	Titulo   string
	Detalhe  string
	// Saldo is the balance the source recorded right after this event, when it
	// recorded one (purchases and adjustments do; top-ups do not).
	Saldo  *int64
	ItemID int64 // donate_shop_item id for a purchase, 0 otherwise
	// ItemTitulo is the shop item's registered title for a purchase, carried
	// apart from Titulo so a reader that writes its own sentence does not have
	// to take this one apart. The panel keeps reading Titulo.
	ItemTitulo string
	Entregue   string // delivery status for a purchase: pending/delivered/lost, or ""
}

// Saldo returns the account's current donate balance.
func (s *Store) Saldo(ctx context.Context, accountID int64) (int32, error) {
	var bal int32
	err := s.pool.QueryRow(ctx, `SELECT donate_balance FROM account WHERE id = $1`, accountID).Scan(&bal)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrNaoEncontrado
	}
	if err != nil {
		return 0, fmt.Errorf("donate: saldo a=%d: %w", accountID, err)
	}
	return bal, nil
}

// Historico merges the three sources into one timeline, newest first.
//
// They are queried separately and merged in Go rather than UNIONed: the three
// have different shapes and different meanings for "amount", and a UNION that
// flattens them would need casts in SQL that hide exactly the distinctions the
// screen is trying to show.
func (s *Store) Historico(ctx context.Context, accountID int64, limite int) ([]Evento, error) {
	if limite <= 0 || limite > 500 {
		limite = 100
	}
	recargas, err := s.recargas(ctx, accountID, limite)
	if err != nil {
		return nil, err
	}
	auditoria, err := s.auditoria(ctx, accountID, limite)
	if err != nil {
		return nil, err
	}
	todos := append(recargas, auditoria...)
	sort.SliceStable(todos, func(i, j int) bool { return todos[i].Quando.After(todos[j].Quando) })
	if len(todos) > limite {
		todos = todos[:limite]
	}
	return todos, nil
}

// recargas reads donate_topup_order. Status 2 is PAID; anything else never
// credited anything and is shown as a dead order — useful precisely because a
// player who paid and sees nothing wants to know the order was never confirmed.
func (s *Store) recargas(ctx context.Context, accountID int64, limite int) ([]Evento, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT credits, amount_cents, payment_method, status, external_reference,
		       created_at, confirmed_at
		  FROM donate_topup_order
		 WHERE account_id = $1
		 ORDER BY created_at DESC
		 LIMIT $2`, accountID, limite)
	if err != nil {
		return nil, fmt.Errorf("donate: recargas a=%d: %w", accountID, err)
	}
	defer rows.Close()

	var out []Evento
	for rows.Next() {
		var (
			creditos   int32
			valorCents int64
			metodo     int16
			status     int16
			referencia string
			criado     time.Time
			confirmado *time.Time
		)
		if err := rows.Scan(&creditos, &valorCents, &metodo, &status, &referencia, &criado, &confirmado); err != nil {
			return nil, fmt.Errorf("donate: recargas: scan: %w", err)
		}
		ev := Evento{
			Quando:  criado,
			Detalhe: fmt.Sprintf("%s · %s · ref. %s", metodoPagamento(metodo), reais(valorCents), referencia),
		}
		if status == statusPago {
			ev.Tipo = TipoRecarga
			ev.Creditos = int64(creditos)
			ev.Titulo = "Recarga confirmada"
			if confirmado != nil {
				ev.Quando = *confirmado
			}
		} else {
			ev.Tipo = TipoPendente
			ev.Titulo = "Recarga não confirmada"
		}
		out = append(out, ev)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("donate: recargas: rows: %w", err)
	}
	return out, nil
}

// statusPago is donate_topup_order.status = PAID (0010_donate_topup).
const statusPago = 2

// auditoria reads donate_shop_audit for the two actions that move a wallet.
//
// account_id on that table means the buyer for 'purchase' and the moderator for
// 'credit_balance' — so a staff credit is found by the target inside the JSON,
// not by the column. That asymmetry is in the schema comment and is the reason
// this query looks the way it does.
func (s *Store) auditoria(ctx context.Context, accountID int64, limite int) ([]Evento, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT a.action, a.shop_item_id, a.before, a.after, a.created_at,
		       COALESCE(m.name, ''), COALESCE(i.title, ''),
		       COALESCE((SELECT d.status FROM delivery_queue d
		                  WHERE d.account_id = $1
		                    AND d.source = 'donate_shop:' || a.shop_item_id
		                    AND d.created_at BETWEEN a.created_at - interval '5 seconds'
		                                         AND a.created_at + interval '5 seconds'
		                  LIMIT 1), '')
		  FROM donate_shop_audit a
		  LEFT JOIN account m ON m.id = a.account_id
		  LEFT JOIN donate_shop_item i ON i.id = a.shop_item_id
		 WHERE (a.action = 'purchase'       AND a.account_id = $1)
		    OR (a.action = 'credit_balance' AND (a.after->>'account_id')::bigint = $1)
		 ORDER BY a.created_at DESC
		 LIMIT $2`, accountID, limite)
	if err != nil {
		return nil, fmt.Errorf("donate: auditoria a=%d: %w", accountID, err)
	}
	defer rows.Close()

	var out []Evento
	for rows.Next() {
		var (
			acao       string
			shopItemID *int64
			antes      []byte
			depois     []byte
			quando     time.Time
			ator       string
			titulo     string
			entrega    string
		)
		if err := rows.Scan(&acao, &shopItemID, &antes, &depois, &quando, &ator, &titulo, &entrega); err != nil {
			return nil, fmt.Errorf("donate: auditoria: scan: %w", err)
		}
		ev := Evento{Quando: quando, Entregue: entrega}
		if shopItemID != nil {
			ev.ItemID = *shopItemID
		}

		campos := decodeCampos(depois)
		if v, ok := campos["balance"]; ok {
			saldo := int64(v)
			ev.Saldo = &saldo
		}

		switch acao {
		case "purchase":
			ev.Tipo = TipoCompra
			ev.Creditos = -int64(campos["price"])
			ev.ItemTitulo = titulo
			ev.Titulo = "Comprou " + primeiroNaoVazio(titulo, fmt.Sprintf("oferta #%d", ev.ItemID))
			ev.Detalhe = "Loja de donate"
		case "credit_balance":
			ev.Tipo = TipoAjuste
			ev.Creditos = int64(campos["amount"])
			ev.Titulo = "Ajuste manual" + porQuem(ator)
			ev.Detalhe = motivo(depois)
		}
		out = append(out, ev)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("donate: auditoria: rows: %w", err)
	}
	return out, nil
}

// decodeCampos pulls the numeric fields out of an audit JSONB blob. The shape is
// whatever the writer chose, so a missing key is normal and yields zero.
func decodeCampos(raw []byte) map[string]float64 {
	out := map[string]float64{}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return out
	}
	for k, v := range m {
		if f, ok := v.(float64); ok {
			out[k] = f
		}
	}
	return out
}

func motivo(raw []byte) string {
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return ""
	}
	if s, ok := m["reason"].(string); ok && s != "" {
		return "Motivo: " + s
	}
	return ""
}

func porQuem(ator string) string {
	if ator == "" {
		return ""
	}
	return " por " + ator
}

func primeiroNaoVazio(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// metodoPagamento names donate_topup_order.payment_method (0010_donate_topup).
func metodoPagamento(m int16) string {
	switch m {
	case 1:
		return "PIX"
	case 2:
		return "Cartão de crédito"
	default:
		return fmt.Sprintf("método %d", m)
	}
}

// reais renders cents as BRL. Formatted here rather than in the template so the
// separators are right without a template function.
func reais(cents int64) string {
	sinal := ""
	if cents < 0 {
		sinal, cents = "-", -cents
	}
	return fmt.Sprintf("%sR$ %d,%02d", sinal, cents/100, cents%100)
}

// Ajustar credits or debits the wallet and records it, in one transaction.
//
// It reuses the donate_shop_audit 'credit_balance' action rather than inventing
// a second log: the web moderation tools already write it, and a wallet with two
// separate histories is a wallet nobody can reconcile.
func (s *Store) Ajustar(ctx context.Context, actorID, accountID int64, delta int32, motivo string) (int32, error) {
	if motivo == "" {
		return 0, ErrMotivoVazio
	}
	if delta == 0 {
		return s.Saldo(ctx, accountID)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("donate: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var antes int32
	err = tx.QueryRow(ctx, `SELECT donate_balance FROM account WHERE id = $1 FOR UPDATE`, accountID).Scan(&antes)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrNaoEncontrado
	}
	if err != nil {
		return 0, fmt.Errorf("donate: ler saldo: %w", err)
	}
	if antes+delta < 0 {
		return 0, ErrSaldoInsuficiente
	}

	var depois int32
	if err := tx.QueryRow(ctx,
		`UPDATE account SET donate_balance = donate_balance + $2 WHERE id = $1 RETURNING donate_balance`,
		accountID, delta).Scan(&depois); err != nil {
		return 0, fmt.Errorf("donate: ajustar a=%d: %w", accountID, err)
	}

	registro, err := json.Marshal(map[string]any{
		"account_id": accountID,
		"amount":     delta,
		"balance":    depois,
		"reason":     motivo,
	})
	if err != nil {
		return 0, fmt.Errorf("donate: ajustar: registro: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO donate_shop_audit (shop_item_id, account_id, action, before, after)
		VALUES (NULL, $1, 'credit_balance', $2, $3)`,
		actorID, mustJSON(map[string]any{"balance": antes}), registro); err != nil {
		return 0, fmt.Errorf("donate: ajustar: auditoria: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("donate: commit: %w", err)
	}
	return depois, nil
}

// mustJSON marshals a map that cannot fail (only strings and numbers).
func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		return []byte(`{}`)
	}
	return b
}
