package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// maxActivePlayCodes caps the unexpired codes per account: each "play in the
// browser" click issues one, and only the newest few can still be in flight.
const maxActivePlayCodes = 5

// PlayCodeIssue is the outcome of IssuePlayCode.
type PlayCodeIssue struct {
	Name    string // canonical account name, the login field the code pairs with
	Blocked bool   // true: no code was stored
}

// IssuePlayCode stores the hash of a one-time login code for an existing,
// unblocked account (web client ADR 017). It drops the account's expired codes
// and keeps at most maxActivePlayCodes, oldest first out. Returns ErrNotFound
// when no account has that id.
func (s *Store) IssuePlayCode(ctx context.Context, accountID int64, codeHash []byte, expiresAt time.Time) (PlayCodeIssue, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return PlayCodeIssue{}, fmt.Errorf("store: begin issue play code: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// FOR UPDATE serializes concurrent issues for one account, so the cap holds.
	var out PlayCodeIssue
	err = tx.QueryRow(ctx, `SELECT name, is_blocked FROM account WHERE id = $1 FOR UPDATE`, accountID).
		Scan(&out.Name, &out.Blocked)
	if errors.Is(err, pgx.ErrNoRows) {
		return PlayCodeIssue{}, ErrNotFound
	}
	if err != nil {
		return PlayCodeIssue{}, fmt.Errorf("store: lock account %d for play code: %w", accountID, err)
	}
	if out.Blocked {
		return out, nil
	}
	if _, err := tx.Exec(ctx,
		`DELETE FROM account_play_code WHERE account_id = $1 AND expires_at <= now()`, accountID); err != nil {
		return PlayCodeIssue{}, fmt.Errorf("store: prune expired play codes: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		DELETE FROM account_play_code WHERE code_hash IN (
			SELECT code_hash FROM account_play_code WHERE account_id = $1
			 ORDER BY created_at DESC OFFSET $2)`, accountID, maxActivePlayCodes-1); err != nil {
		return PlayCodeIssue{}, fmt.Errorf("store: cap play codes: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO account_play_code (code_hash, account_id, expires_at) VALUES ($1, $2, $3)`,
		codeHash, accountID, expiresAt); err != nil {
		return PlayCodeIssue{}, fmt.Errorf("store: insert play code: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return PlayCodeIssue{}, fmt.Errorf("store: commit play code: %w", err)
	}
	return out, nil
}

// ConsumePlayCode reports whether codeHash is an unexpired code of accountID
// and deletes it in the same statement, so a code logs in at most once even
// under concurrent logins.
func (s *Store) ConsumePlayCode(ctx context.Context, accountID int64, codeHash []byte) (bool, error) {
	var one int
	err := s.pool.QueryRow(ctx, `
		DELETE FROM account_play_code
		WHERE account_id = $1 AND code_hash = $2 AND expires_at > now()
		RETURNING 1`, accountID, codeHash).Scan(&one)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("store: consume play code for account %d: %w", accountID, err)
	}
	return true, nil
}
