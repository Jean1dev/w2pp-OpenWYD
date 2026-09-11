// Package audit records and reads the admin panel's action log.
//
// The queries live here rather than in internal/store on purpose. Every service
// embeds internal/, so adding to it redeploys tmServer, dbServer, binServer and
// webServer — restarting the live game to ship a panel screen. No game service
// reads this table, so there is nothing to share; keeping it local is what lets
// the panel's own changes stay inside /adminserver/**.
//
// The table refuses UPDATE and DELETE at the database level (migration 0022), so
// nothing here needs to defend the append-only property — it cannot be violated
// through this package or around it.
package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Action names. Kept as constants so a typo becomes a compile error rather than
// a row nobody can filter for later.
const (
	ActionSetRole            = "SET_ROLE"
	ActionSetBlocked         = "SET_BLOCKED"
	ActionSetVip             = "SET_VIP"
	ActionSetItemPrice       = "SET_ITEM_PRICE"
	ActionRestartGame        = "RESTART_GAME"
	ActionSetNpcShop         = "SET_NPC_SHOP"
	ActionSetNpc             = "SET_NPC"
	ActionDeleteNpc          = "DELETE_NPC"
	ActionSetMobStat         = "SET_MOB_STAT"
	ActionClearMobStat       = "CLEAR_MOB_STAT"
	ActionSetItemStat        = "SET_ITEM_STAT"
	ActionClearItemStat      = "CLEAR_ITEM_STAT"
	ActionDeliverItem        = "DELIVER_ITEM"
	ActionCancelDelivery     = "CANCEL_DELIVERY"
	ActionSetPassword        = "SET_PASSWORD"
	ActionCreateAccount      = "CREATE_ACCOUNT"
	ActionKick               = "KICK"
	ActionUnstuck            = "UNSTUCK"
	ActionSetWorldEvent      = "SET_WORLD_EVENT"
	ActionHandleReport       = "HANDLE_REPORT"
	ActionBroadcast          = "BROADCAST"
	ActionSafeRestart        = "SAFE_RESTART"
	ActionStopGame           = "STOP_GAME"
	ActionStartGame          = "START_GAME"
	ActionSetXPRule          = "SET_XP_RULE"
	ActionSetDungeonGate     = "SET_DUNGEON_GATE"
	ActionSetSpawnRate       = "SET_SPAWN_RATE"
	ActionClearSpawnRate     = "CLEAR_SPAWN_RATE"
	ActionSetCombatRule      = "SET_COMBAT_RULE"
	ActionClearCombatRule    = "CLEAR_COMBAT_RULE"
	ActionSetQuestReward     = "SET_QUEST_REWARD"
	ActionClearQuestReward   = "CLEAR_QUEST_REWARD"
	ActionSetDropBonus       = "SET_DROP_BONUS"
	ActionClearDropBonus     = "CLEAR_DROP_BONUS"
	ActionSetDropRule        = "SET_DROP_RULE"
	ActionDeleteDropRule     = "DELETE_DROP_RULE"
	ActionSetDropBonusLigado = "SET_DROP_BONUS_LIGADO"
	ActionSetCombineRate     = "SET_COMBINE_RATE"
	ActionClearCombineRate   = "CLEAR_COMBINE_RATE"
	ActionSetCombineBands    = "SET_COMBINE_BANDS"
	ActionSetCombineTag      = "SET_COMBINE_TAG"
	ActionClearXPRule        = "CLEAR_XP_RULE"
	ActionSetMountGrowth     = "SET_MOUNT_GROWTH"
	ActionClearMountGrowth   = "CLEAR_MOUNT_GROWTH"
	ActionSetMountAbsorb     = "SET_MOUNT_ABSORB"
	ActionClearMountAbsorb   = "CLEAR_MOUNT_ABSORB"
	ActionSetMountBonus      = "SET_MOUNT_BONUS"
	ActionClearMountBonus    = "CLEAR_MOUNT_BONUS"
)

// listLimit caps one page of the log.
const listLimit = 100

// Record is one action to write, as the caller knows it.
type Record struct {
	ActorID   int64
	ActorRole string // the role AT THE TIME of the action, not looked up later
	Action    string
	TargetID  int64 // 0 when the action has no target
	Old, New  any   // marshalled to JSONB; nil writes SQL NULL
}

// Entry is one row as the log page shows it, with names resolved.
type Entry struct {
	ID         int64
	ActorID    int64
	ActorName  string
	ActorRole  string
	Action     string
	TargetID   int64
	TargetName string
	Old, New   string // pretty-printed JSON, or "" when absent
	CreatedAt  time.Time
}

// Store reads and writes the log.
type Store struct{ pool *pgxpool.Pool }

// New wraps a pool.
func New(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// Write appends one entry.
//
// It returns an error rather than swallowing one, and every caller must treat a
// failure as a failure of the action itself: an administrative change that was
// applied but not recorded is exactly the change nobody can explain afterwards.
func (s *Store) Write(ctx context.Context, r Record) error {
	oldJSON, err := toJSON(r.Old)
	if err != nil {
		return fmt.Errorf("audit: encode old value: %w", err)
	}
	newJSON, err := toJSON(r.New)
	if err != nil {
		return fmt.Errorf("audit: encode new value: %w", err)
	}

	var target any
	if r.TargetID != 0 {
		target = r.TargetID
	}

	_, err = s.pool.Exec(ctx, `
		INSERT INTO admin_audit_log
		    (actor_account_id, actor_role, action, target_account_id, old_value, new_value)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		r.ActorID, r.ActorRole, r.Action, target, oldJSON, newJSON)
	if err != nil {
		return fmt.Errorf("audit: write %s: %w", r.Action, err)
	}
	return nil
}

// List returns the most recent entries, newest first. A non-zero targetID
// narrows to one account's history.
// List returns audit entries newest first.
//
// limit and offset come from the caller so the page can be turned: the audit
// log is a HISTORY, and a fixed cap that said "showing the 100 most recent" was
// honest and still a dead end — there was no way to reach the 101st. A limit at
// or below zero, or above listLimit, falls back to listLimit rather than
// letting a typed URL ask for the whole table.
func (s *Store) List(ctx context.Context, targetID int64, limit, offset int) ([]Entry, error) {
	if limit <= 0 || limit > listLimit {
		limit = listLimit
	}
	if offset < 0 {
		offset = 0
	}
	// One query with a nullable filter rather than two: the difference is a
	// parameter, and two near-identical SQL strings drift apart over time.
	rows, err := s.pool.Query(ctx, `
		SELECT l.id, l.actor_account_id, COALESCE(a.name, ''), l.actor_role, l.action,
		       COALESCE(l.target_account_id, 0), COALESCE(t.name, ''),
		       l.old_value, l.new_value, l.created_at
		  FROM admin_audit_log l
		  LEFT JOIN account a ON a.id = l.actor_account_id
		  LEFT JOIN account t ON t.id = l.target_account_id
		 WHERE $1::bigint IS NULL OR l.target_account_id = $1
		 ORDER BY l.created_at DESC, l.id DESC
		 LIMIT $2 OFFSET $3`, nullableID(targetID), limit, offset)
	if err != nil {
		return nil, fmt.Errorf("audit: list: %w", err)
	}
	defer rows.Close()
	return scanEntries(rows)
}

// scanEntries drains a result set shaped like the SELECT above. Both listings
// read the same ten columns in the same order, and one decoder is what keeps
// them from drifting apart.
func scanEntries(rows pgx.Rows) ([]Entry, error) {
	out := make([]Entry, 0, listLimit)
	for rows.Next() {
		var e Entry
		var oldRaw, newRaw []byte
		if err := rows.Scan(&e.ID, &e.ActorID, &e.ActorName, &e.ActorRole, &e.Action,
			&e.TargetID, &e.TargetName, &oldRaw, &newRaw, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("audit: scan entry: %w", err)
		}
		e.Old = compact(oldRaw)
		e.New = compact(newRaw)
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("audit: list: %w", err)
	}
	return out, nil
}

// Limit reports the page cap, so the UI can say the list is partial without
// duplicating the number.
func (s *Store) Limit() int { return listLimit }

func nullableID(id int64) any {
	if id == 0 {
		return nil
	}
	return id
}

func toJSON(v any) (any, error) {
	if v == nil {
		return nil, nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// compact renders stored JSONB for display. An unreadable value is shown as
// stored rather than dropped: the log is evidence, and hiding a row because its
// payload does not parse is the wrong failure.
func compact(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	var v map[string]any
	if err := json.Unmarshal(raw, &v); err != nil {
		return string(raw)
	}
	b, err := json.Marshal(v)
	if err != nil {
		return string(raw)
	}
	return string(b)
}

// rotulos turns an action constant into the phrase a person reads.
//
// The stored value stays the constant: it is the machine-readable key, it is
// what a query filters on, and translating it at write time would have made
// every past row unmatchable the first time somebody reworded a label.
//
// This map is why the screen is worth reading at all. Six actions were tolerable
// as SET_ROLE and SET_VIP; there are twenty now, and a page of upper-case
// English constants in a Portuguese panel is a page nobody reads — which defeats
// the log, whose whole purpose is being read after something went wrong.
var rotulos = map[string]string{
	ActionSetRole:            "Mudou o cargo",
	ActionSetBlocked:         "Bloqueou ou desbloqueou",
	ActionSetVip:             "Mexeu no VIP",
	ActionSetPassword:        "Trocou a senha",
	ActionCreateAccount:      "Criou uma conta",
	ActionSetItemPrice:       "Mudou o preço de um item",
	ActionSetNpcShop:         "Mudou a loja de um NPC",
	ActionSetNpc:             "Editou um NPC",
	ActionDeleteNpc:          "Apagou um NPC",
	ActionSetMobStat:         "Editou os atributos de um monstro",
	ActionClearMobStat:       "Restaurou um monstro",
	ActionSetItemStat:        "Editou os atributos de um item",
	ActionClearItemStat:      "Restaurou um item",
	ActionDeliverItem:        "Entregou um item",
	ActionCancelDelivery:     "Cancelou uma entrega",
	ActionKick:               "Derrubou uma conta",
	ActionUnstuck:            "Desatolou um personagem",
	ActionSetWorldEvent:      "Mexeu nos eventos do servidor",
	ActionHandleReport:       "Tratou uma denúncia",
	ActionBroadcast:          "Mandou um aviso para todos",
	ActionRestartGame:        "Reiniciou o servidor",
	ActionSafeRestart:        "Reiniciou com segurança",
	ActionStopGame:           "Desligou o servidor",
	ActionStartGame:          "Ligou o servidor",
	ActionSetXPRule:          "Mexeu na Mesa de XP",
	ActionSetDungeonGate:     "Abriu ou fechou uma masmorra",
	ActionSetSpawnRate:       "Mudou o tempo de spawn de uma área",
	ActionClearSpawnRate:     "Voltou o tempo de spawn de uma área ao conteúdo",
	ActionSetCombatRule:      "Mudou a regra de combate",
	ActionClearCombatRule:    "Voltou a regra de combate ao padrão",
	ActionSetQuestReward:     "Mudou a recompensa de uma quest",
	ActionClearQuestReward:   "Voltou a recompensa de uma quest ao conteúdo",
	ActionSetDropBonus:       "Mudou a escada do bônus de drop",
	ActionSetCombineRate:     "Mudou a taxa de uma máquina",
	ActionClearCombineRate:   "Devolveu a taxa de uma máquina ao arquivo",
	ActionSetCombineBands:    "Mudou as faixas de conjunto de uma máquina",
	ActionSetCombineTag:      "Marcou a operação (ADD/ABS) de uma máquina",
	ActionClearDropBonus:     "Voltou a escada do bônus de drop ao legado",
	ActionSetDropRule:        "Gravou uma regra na Mesa de Drops",
	ActionDeleteDropRule:     "Apagou uma regra da Mesa de Drops",
	ActionSetDropBonusLigado: "Ligou ou desligou o sorteio de bônus de drop",
	ActionClearXPRule:        "Voltou uma tabela de XP ao legado",
	ActionSetMountGrowth:     "Mexeu na taxa de crescimento de uma montaria",
	ActionClearMountGrowth:   "Voltou a curva de uma montaria ao padrão",
	ActionSetMountAbsorb:     "Mexeu na absorção de uma montaria",
	ActionClearMountAbsorb:   "Voltou a absorção de uma montaria ao padrão",
	ActionSetMountBonus:      "Mexeu nos atributos de uma montaria",
	ActionClearMountBonus:    "Voltou os atributos de uma montaria ao padrão",
}

// Rotulo is the readable name of this entry's action.
//
// An unknown action falls back to the raw constant rather than to "—" or to
// nothing: a row written by a version of the panel this one does not know about
// is exactly the row somebody is looking for, and hiding what it says would be
// worse than showing it untranslated.
func (e Entry) Rotulo() string {
	if r, ok := rotulos[e.Action]; ok {
		return r
	}
	return e.Action
}

// ListActions returns the most recent entries for a set of action names,
// newest first. It is the history behind a single screen — the Mesa de XP asks
// for its own SET_XP_RULE / CLEAR_XP_RULE entries — so the page can show what
// was changed there without making the reader filter the whole log by eye.
func (s *Store) ListActions(ctx context.Context, actions []string) ([]Entry, error) {
	if len(actions) == 0 {
		return nil, nil
	}
	rows, err := s.pool.Query(ctx, `
		SELECT l.id, l.actor_account_id, COALESCE(a.name, ''), l.actor_role, l.action,
		       COALESCE(l.target_account_id, 0), COALESCE(t.name, ''),
		       l.old_value, l.new_value, l.created_at
		  FROM admin_audit_log l
		  LEFT JOIN account a ON a.id = l.actor_account_id
		  LEFT JOIN account t ON t.id = l.target_account_id
		 WHERE l.action = ANY($1)
		 ORDER BY l.created_at DESC, l.id DESC
		 LIMIT $2`, actions, listLimit)
	if err != nil {
		return nil, fmt.Errorf("audit: list actions: %w", err)
	}
	defer rows.Close()
	return scanEntries(rows)
}
