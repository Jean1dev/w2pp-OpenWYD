package siteapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/entrega"
)

// ErrContaNaoExiste is returned for an id with no account behind it.
var ErrContaNaoExiste = errors.New("siteapi: account not found")

// Leitor runs the two reads this API needs that no other package has. They live
// here, not in internal/store, for the reason the rest of the panel's queries
// do: every service embeds internal/, and adding there would redeploy the game
// to ship a panel change.
type Leitor struct{ pool *pgxpool.Pool }

// NovoLeitor wraps a pool.
func NovoLeitor(pool *pgxpool.Pool) *Leitor { return &Leitor{pool: pool} }

// Nome returns the name the game knows an account by. The live link works with
// names and the site sends ids; resolving here is what keeps the site from ever
// naming the account a live call acts on.
func (l *Leitor) Nome(ctx context.Context, id int64) (string, error) {
	var nome string
	err := l.pool.QueryRow(ctx, `SELECT name FROM account WHERE id = $1`, id).Scan(&nome)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrContaNaoExiste
	}
	if err != nil {
		return "", fmt.Errorf("siteapi: name of %d: %w", id, err)
	}
	return nome, nil
}

// payloadItem mirrors delivery_queue.payload for kind='item'. entrega spells the
// same fields out for the same reason: they are the contract the tmServer drain
// reads, and a round-trip test pins them there.
type payloadItem struct {
	ItemIndex int32 `json:"item_index"`
	Eff1      uint8 `json:"eff1"`
	EffV1     uint8 `json:"effv1"`
	Eff2      uint8 `json:"eff2"`
	EffV2     uint8 `json:"effv2"`
	Eff3      uint8 `json:"eff3"`
	EffV3     uint8 `json:"effv3"`
	ExpiresAt int64 `json:"expires_at"`
}

// Perdidos lists the items the game tried to deliver and could not place — a
// full warehouse — newest first.
//
// 'lost' is written only by the drain (internal/store/donate.go, markDeliveries
// with lostIDs). A cancel deletes the row instead (entrega.Cancelar), precisely
// so this status keeps meaning "the player was owed this and did not get it".
func (l *Leitor) Perdidos(ctx context.Context, contaID int64, limite int) ([]entrega.Pendente, error) {
	rows, err := l.pool.Query(ctx, `
		SELECT id, payload, coalesce(source, ''), created_at
		  FROM delivery_queue
		 WHERE account_id = $1 AND status = 'lost' AND kind = 'item'
		 ORDER BY id DESC
		 LIMIT $2`, contaID, limite)
	if err != nil {
		return nil, fmt.Errorf("siteapi: lost deliveries of %d: %w", contaID, err)
	}
	defer rows.Close()

	out := []entrega.Pendente{}
	for rows.Next() {
		var p entrega.Pendente
		var corpo []byte
		if err := rows.Scan(&p.ID, &corpo, &p.Origem, &p.CriadoEm); err != nil {
			return nil, fmt.Errorf("siteapi: scan lost delivery: %w", err)
		}
		var pl payloadItem
		// A row whose payload does not parse is still an item the player lost; it
		// is listed with what could be read (index 0) rather than hidden.
		if json.Unmarshal(corpo, &pl) == nil {
			p.ItemIndex = pl.ItemIndex
			p.Eff = [3][2]uint8{{pl.Eff1, pl.EffV1}, {pl.Eff2, pl.EffV2}, {pl.Eff3, pl.EffV3}}
			p.ExpiresAt = pl.ExpiresAt
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("siteapi: iterate lost deliveries: %w", err)
	}
	return out, nil
}
