// Package accounts performs the panel's writes to the account table.
//
// The queries live here rather than in internal/store for the same reason the
// audit ones do: every service embeds internal/, so adding there redeploys the
// game to ship a panel change. Nothing in the game writes account.role or
// account.is_blocked from a panel, so there is nothing to share.
//
// Every write here is a two-step: check the guards, then apply. The guards are
// not cosmetic — each one prevents a state the panel could not get itself out
// of afterwards.
package accounts

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jeanluca/w2pp-openwyd/internal/domain"
	"github.com/jeanluca/w2pp-openwyd/internal/store"
)

// Roles the panel is allowed to set. A value outside this set is refused rather
// than written: world.ParseAccess fails closed on anything unrecognised, so a
// typo would silently strip someone's authority instead of erroring.
const (
	RolePlayer    = "player"
	RoleModerator = "moderator"
	RoleAdmin     = "admin"
)

// Refusals. They are values, not strings, so a handler can tell them apart and
// say something useful instead of "erro".
var (
	ErrUnknownRole = errors.New("accounts: unknown role")
	ErrNotFound    = errors.New("accounts: account not found")
	ErrSelf        = errors.New("accounts: an account cannot change its own access")
	ErrLastAdmin   = errors.New("accounts: this is the last admin")
)

// Store performs the writes.
type Store struct{ pool *pgxpool.Pool }

// New wraps a pool.
func New(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// ValidRole reports whether role is one the panel may set.
func ValidRole(role string) bool {
	return role == RolePlayer || role == RoleModerator || role == RoleAdmin
}

// SetRole changes an account's role and returns the previous one.
//
// actorID is the signed-in staff member. Two refusals matter more than they
// look:
//
//	Self — an admin demoting themselves is the panel's own foot-gun: the change
//	takes effect on their next request, and if they were the last admin nobody
//	can undo it from the panel at all.
//
//	Last admin — leaving the server with no admin means the only way back is a
//	hand-written UPDATE against the database, which is exactly the thing this
//	panel exists to stop being normal.
func (s *Store) SetRole(ctx context.Context, actorID, targetID int64, role string) (string, error) {
	if !ValidRole(role) {
		return "", ErrUnknownRole
	}
	if actorID == targetID {
		return "", ErrSelf
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("accounts: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// FOR UPDATE so the last-admin count below cannot be raced by a second
	// demotion running at the same instant. Two panels demoting the two
	// remaining admins concurrently would otherwise both see a count of 2.
	var current string
	err = tx.QueryRow(ctx, `SELECT role FROM account WHERE id = $1 FOR UPDATE`, targetID).Scan(&current)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("accounts: read role: %w", err)
	}
	if current == role {
		return current, nil // nothing to do; not an error
	}

	if current == RoleAdmin && role != RoleAdmin {
		var admins int
		if err := tx.QueryRow(ctx,
			`SELECT count(*) FROM account WHERE role = $1`, RoleAdmin).Scan(&admins); err != nil {
			return "", fmt.Errorf("accounts: count admins: %w", err)
		}
		if admins <= 1 {
			return "", ErrLastAdmin
		}
	}

	if _, err := tx.Exec(ctx, `UPDATE account SET role = $2 WHERE id = $1`, targetID, role); err != nil {
		return "", fmt.Errorf("accounts: set role: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("accounts: commit: %w", err)
	}
	return current, nil
}

// Bloqueio is what the panel knows about an account's block: the flag, why, by
// whom and when. Reason is "" for a block that predates the reason column or was
// issued in game through /gm ban, which records nothing.
type Bloqueio struct {
	Blocked bool
	Reason  string
	At      *time.Time
	By      *int64
	// Until is when the ban lifts. nil means it does not — explicitly, not as a
	// date in the past: everything that asks "blocked right now" treats nil as
	// permanent, and a sentinel would read as already lifted.
	Until *time.Time
}

// Vigente reports whether the ban is in force at this moment. It mirrors
// store.BlockedNowSQL, which is what the login, the pre-delete check and the
// account search all evaluate — the panel must not disagree with them about who
// is banned.
func (b Bloqueio) Vigente() bool {
	return b.Blocked && (b.Until == nil || b.Until.After(time.Now()))
}

// MaxDiasBan bounds a timed ban. Longer than this is a permanent ban somebody
// typed as a number, and it should be said out loud instead.
const MaxDiasBan = 3650

// ErrPrazo is returned for a ban length outside the bounds.
var ErrPrazo = errors.New("accounts: ban length out of range")

// MaxMotivoBytes bounds the reason. It is generous — the field is for a sentence
// a colleague or a player will read, not for a case file.
const MaxMotivoBytes = 500

// ErrMotivo is returned for a reason that is too long, or missing on a block.
var ErrMotivo = errors.New("accounts: bad block reason")

// SetBlocked blocks or unblocks an account and returns the previous state.
//
// Blocking yourself is refused for the same reason as demoting yourself: the
// block applies to the game AND to the panel login, so it locks the door with
// the key inside.
//
// A reason is required to block and ignored to unblock. The whole point of the
// column is that a player who writes in can be told why, and a ban with an empty
// reason is the state this migration exists to remove.
//
// It writes whenever ANYTHING changes, not only when the flag flips. Editing the
// reason of a ban that is already in force is a real edit, and the previous
// version short-circuited on the flag alone — so correcting a reason wrote
// nothing, reported "nothing changed", and left no audit trail of the attempt.
func (s *Store) SetBlocked(ctx context.Context, actorID, targetID int64, blocked bool, motivo string, dias int) (Bloqueio, error) {
	if actorID == targetID {
		return Bloqueio{}, ErrSelf
	}
	if dias < 0 || dias > MaxDiasBan {
		return Bloqueio{}, ErrPrazo
	}
	motivo = strings.TrimSpace(motivo)
	if blocked && (motivo == "" || len(motivo) > MaxMotivoBytes) {
		return Bloqueio{}, ErrMotivo
	}
	if !blocked {
		motivo = "" // an unblocked account carries no reason
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Bloqueio{}, fmt.Errorf("accounts: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Read then write, rather than one UPDATE ... RETURNING with a subquery for
	// the old value: whether such a subquery sees the row before or after the
	// update depends on the statement snapshot, and an audit entry that records
	// the wrong "before" is worse than no audit entry.
	var prev Bloqueio
	err = tx.QueryRow(ctx, `
		SELECT is_blocked, block_reason, blocked_at, blocked_by, blocked_until
		  FROM account WHERE id = $1 FOR UPDATE`, targetID).
		Scan(&prev.Blocked, &prev.Reason, &prev.At, &prev.By, &prev.Until)
	if errors.Is(err, pgx.ErrNoRows) {
		return Bloqueio{}, ErrNotFound
	}
	if err != nil {
		return Bloqueio{}, fmt.Errorf("accounts: read blocked: %w", err)
	}
	// 0 days means permanent; anything else is a deadline from now.
	var ate *time.Time
	if blocked && dias > 0 {
		t := time.Now().Add(time.Duration(dias) * 24 * time.Hour)
		ate = &t
	}
	if prev.Blocked == blocked && prev.Reason == motivo && mesmoPrazo(prev.Until, ate, dias) {
		return prev, nil // genuinely nothing to do
	}

	// blocked_at and blocked_by are refreshed on every block, including a reason
	// edit: the row then answers "who is standing behind this ban as it reads
	// now", which is the question somebody reviewing it actually has.
	if blocked {
		_, err = tx.Exec(ctx, `
			UPDATE account
			   SET is_blocked = TRUE, block_reason = $2, blocked_at = now(),
			       blocked_by = $3, blocked_until = $4
			 WHERE id = $1`, targetID, motivo, actorID, ate)
	} else {
		_, err = tx.Exec(ctx, `
			UPDATE account
			   SET is_blocked = FALSE, block_reason = '', blocked_at = NULL,
			       blocked_by = NULL, blocked_until = NULL
			 WHERE id = $1`, targetID)
	}
	if err != nil {
		return Bloqueio{}, fmt.Errorf("accounts: set blocked: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Bloqueio{}, fmt.Errorf("accounts: commit: %w", err)
	}
	return prev, nil
}

// mesmoPrazo reports whether a re-block asks for the deadline the row already
// has. Two dates computed a second apart are never equal, so re-submitting the
// same form would otherwise always look like a change; asking for a permanent
// ban on a permanently banned account is the only case that can be compared
// exactly.
func mesmoPrazo(atual, novo *time.Time, dias int) bool {
	if dias == 0 {
		return atual == nil && novo == nil
	}
	return false
}

// Blocked reports whether an account is blocked right now.
//
// requireStaff calls this on every request, next to the role read, because a
// blocked staff account kept its panel session until now: the role was
// re-checked and the block was not, so banning a moderator left them signed in
// and able to keep working.
func (s *Store) Blocked(ctx context.Context, id int64) (bool, error) {
	// store.BlockedNowSQL, not a bare column: an expired ban must not end a panel
	// session the login would already let through.
	var blocked bool
	err := s.pool.QueryRow(ctx,
		`SELECT `+store.BlockedNowSQL+` FROM account WHERE id = $1`, id).Scan(&blocked)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, ErrNotFound
	}
	if err != nil {
		return false, fmt.Errorf("accounts: blocked %d: %w", id, err)
	}
	return blocked, nil
}

// --- VIP ---

// VIP grant bounds. A day count outside this is a typo, not an intention: one
// day is the smallest useful grant, and ten years is longer than the server is
// likely to outlive. Refusing beats writing a date nobody meant.
const (
	MinVipDays = 1
	MaxVipDays = 3650
)

// ErrVipDays is returned for a grant length outside the bounds above.
var ErrVipDays = errors.New("accounts: vip day count out of range")

// Details is what the panel shows about an account beyond its auth row.
//
// It exists here rather than in internal/store for the same reason the writes
// do — and it carries the email and donate balance the account page had to go
// without while there was no panel-owned read.
type Details struct {
	Email         string
	DonateBalance int32
	// ShopPoints is the personal-shop wallet (0060_shop_points): what the
	// account earned by keeping a stocked lojinha open. A missing wallet row
	// reads as zero, so an account that never opened a shop shows 0 rather than
	// failing the whole page.
	ShopPoints int32
	VipUntil   *time.Time // nil means the account has never been VIP
	Bloqueio   Bloqueio   // why the account is blocked, when, and by whom
}

// Get reads the panel-facing fields of one account.
func (s *Store) Get(ctx context.Context, id int64) (Details, error) {
	var d Details
	err := s.pool.QueryRow(ctx, `
		SELECT email, donate_balance,
		       COALESCE((SELECT balance FROM shop_points WHERE account_id = account.id), 0),
		       vip_until,
		       is_blocked, block_reason, blocked_at, blocked_by, blocked_until
		  FROM account WHERE id = $1`, id).
		Scan(&d.Email, &d.DonateBalance, &d.ShopPoints, &d.VipUntil,
			&d.Bloqueio.Blocked, &d.Bloqueio.Reason, &d.Bloqueio.At, &d.Bloqueio.By, &d.Bloqueio.Until)
	if errors.Is(err, pgx.ErrNoRows) {
		return Details{}, ErrNotFound
	}
	if err != nil {
		return Details{}, fmt.Errorf("accounts: get %d: %w", id, err)
	}
	return d, nil
}

// AddVipDays extends an account's VIP and returns the dates before and after.
//
// Extension counts from whichever is later: now, or the current expiry. Adding
// thirty days to somebody who still has ten left gives forty, not thirty — the
// other reading silently takes time away from a paying player, and it is the
// reading a naive `now() + interval` produces.
//
// There is deliberately no self-grant guard, which is why the actor is accepted
// and then ignored here. VIP is an entitlement, not authority: granting it to
// yourself cannot lock anyone out or escalate what you can do, and the audit
// entry the caller writes names who did it. Blocking it would also stop staff
// testing their own change, which is a real thing they need to do.
//
// The parameter stays for symmetry with SetRole and SetBlocked, where it is load
// bearing: a guard added here later should not have to change the signature and
// every caller with it.
func (s *Store) AddVipDays(ctx context.Context, _, targetID int64, days int) (prev, next *time.Time, err error) {
	if days < MinVipDays || days > MaxVipDays {
		return nil, nil, ErrVipDays
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("accounts: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// FOR UPDATE so two grants landing together both extend, instead of the
	// second computing its new date from the value the first has already
	// replaced — which would silently drop one of the two grants.
	var current *time.Time
	err = tx.QueryRow(ctx, `SELECT vip_until FROM account WHERE id = $1 FOR UPDATE`, targetID).Scan(&current)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, ErrNotFound
	}
	if err != nil {
		return nil, nil, fmt.Errorf("accounts: read vip: %w", err)
	}

	from := time.Now().UTC()
	if current != nil && current.After(from) {
		from = current.UTC()
	}
	novo := from.AddDate(0, 0, days)

	if _, err := tx.Exec(ctx, `UPDATE account SET vip_until = $2 WHERE id = $1`, targetID, novo); err != nil {
		return nil, nil, fmt.Errorf("accounts: set vip: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, nil, fmt.Errorf("accounts: commit: %w", err)
	}
	return current, &novo, nil
}

// ClearVip removes VIP immediately and returns the date it had.
//
// It writes NULL rather than a past date: "never had VIP" and "had VIP, taken
// away" read the same to anything that compares against now(), and the audit log
// is where the difference is preserved.
func (s *Store) ClearVip(ctx context.Context, _, targetID int64) (*time.Time, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("accounts: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var current *time.Time
	err = tx.QueryRow(ctx, `SELECT vip_until FROM account WHERE id = $1 FOR UPDATE`, targetID).Scan(&current)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("accounts: read vip: %w", err)
	}
	if current == nil {
		return nil, nil // already not VIP
	}

	if _, err := tx.Exec(ctx, `UPDATE account SET vip_until = NULL WHERE id = $1`, targetID); err != nil {
		return nil, fmt.Errorf("accounts: clear vip: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("accounts: commit: %w", err)
	}
	return current, nil
}

// VipActive reports whether a stored expiry means the account is VIP right now.
// Comparing against now() is the whole expiry mechanism: nothing sweeps the
// column, so a lapsed date simply stops counting.
func VipActive(until *time.Time) bool {
	return until != nil && until.After(time.Now())
}

// --- pendências de reinício ---

// PendingSince reports how many boot-bound overrides were edited after the
// given moment, and when the most recent edit was.
//
// It counts the two boot-bound tables and nothing else. NPC definitions, shops
// and item PRICES are polled by the tmServer every ~15 seconds and apply live;
// mob template stats and item base stats are read once at boot and there is no
// hot reload — the code says so, and notes that the legacy EDITAPPMOB behaved
// the same way. Counting the live ones would make the warning cry wolf, and a
// warning people learn to ignore is worse than none.
func (s *Store) PendingSince(ctx context.Context, since time.Time) (n int, last time.Time, err error) {
	var lastNull *time.Time
	err = s.pool.QueryRow(ctx, `
		SELECT count(*), max(updated_at) FROM (
			SELECT updated_at FROM mob_template_stat WHERE updated_at > $1
			UNION ALL
			SELECT updated_at FROM item_stat WHERE updated_at > $1
		) AS pendentes`, since).Scan(&n, &lastNull)
	if err != nil {
		return 0, time.Time{}, fmt.Errorf("accounts: pending overrides: %w", err)
	}
	if lastNull != nil {
		last = *lastNull
	}
	return n, last, nil
}

// --- senha ---

// Password rules, all of them imposed by the game client rather than by taste.
//
// The login packet carries AccountPassword as a fixed [12]byte
// (tmserver/internal/protocol/messages.go:28), so anything longer verifies in
// the panel and then never works in game — the worst possible failure, because
// it looks like the reset worked. Spaces are out for the same reason: the
// decoder trims trailing spaces off fixed-width fields, so a password that ends
// in one cannot be typed back.
const (
	MaxSenhaBytes = 12
	MinSenhaBytes = 4
)

// Refusals, as values so a handler can say which rule was broken.
var (
	ErrSenhaVazia     = errors.New("accounts: empty password")
	ErrSenhaLonga     = errors.New("accounts: password longer than the client can carry")
	ErrSenhaCurta     = errors.New("accounts: password too short")
	ErrSenhaEspaco    = errors.New("accounts: password contains a space")
	ErrSenhaCaractere = errors.New("accounts: password has a character the client cannot type")
)

// ValidarSenha checks a password against what the client can carry and type.
//
// Empty is refused loudly rather than silently accepted. secret.HashSecret("")
// returns an empty hash on purpose — it means "no secret set" — and
// secret.VerifySecret then matches it against an empty password. That is correct
// for an unset block PIN and catastrophic for a login password: the account
// would sign in with no password at all, on the panel, the web and the game. The
// guard belongs here, before the hash, not after it.
func ValidarSenha(s string) error {
	switch {
	case s == "":
		return ErrSenhaVazia
	case len(s) > MaxSenhaBytes:
		return ErrSenhaLonga
	case len(s) < MinSenhaBytes:
		return ErrSenhaCurta
	}
	for _, r := range s {
		if r == ' ' {
			return ErrSenhaEspaco
		}
		// The wire is bytes, and the client is a Windows program from 2003. Stay
		// inside printable ASCII rather than discover which code page it uses.
		if r < '!' || r > '~' {
			return ErrSenhaCaractere
		}
	}
	return nil
}

// senhaAlfabeto omits the character pairs people mistype when reading a password
// off a screen and typing it into a game client: 0/O, 1/l/I.
const senhaAlfabeto = "abcdefghijkmnpqrstuvwxyzACDEFGHJKLMNPQRTUVWXY2345679"

// senhaGerada is shorter than the 12-byte ceiling on purpose: the moderator has
// to read it out and the player has to type it, and the two extra characters buy
// less than the transcription errors they cost.
const senhaGerada = 10

// GerarSenha returns a random password that satisfies ValidarSenha.
//
// The panel offers this as the default rather than an empty field, so the empty
// case is not something a distracted moderator can reach by pressing enter.
func GerarSenha() (string, error) {
	out := make([]byte, senhaGerada)
	limite := big.NewInt(int64(len(senhaAlfabeto)))
	for i := range out {
		n, err := rand.Int(rand.Reader, limite)
		if err != nil {
			return "", fmt.Errorf("accounts: generate password: %w", err)
		}
		out[i] = senhaAlfabeto[n.Int64()]
	}
	return string(out), nil
}

// SetPassword replaces an account's password hash.
//
// The hash is computed by the caller so this package never holds the plaintext
// beyond validation, and so the empty-hash case cannot arrive here by accident:
// an empty hash is refused outright rather than written, because it would mean
// "any empty password logs in".
//
// There is no self guard. Changing your own password is a normal thing to do and
// cannot lock anyone else out; the caller decides who may target whom, because
// that rule is about rank and lives with the rest of the panel's authorization.
func (s *Store) SetPassword(ctx context.Context, targetID int64, hash string) error {
	if hash == "" {
		return ErrSenhaVazia
	}
	tag, err := s.pool.Exec(ctx, `UPDATE account SET pass_hash = $2 WHERE id = $1`, targetID, hash)
	if err != nil {
		return fmt.Errorf("accounts: set password: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// --- busca por conta ou por personagem ---

// Achado is one account the search matched, and how it matched.
//
// PorPersonagem carries the character name when that is what the term hit, and
// is empty when the account name did. Without it the result is a list of account
// names a moderator does not recognise: they searched for a character, and
// account names in this game rarely resemble the characters on them.
type Achado struct {
	domain.AccountSummary
	PorPersonagem string
}

// Buscar finds accounts by their own name OR by the name of a character on them.
//
// The second half is the whole point. Moderation starts from a report, and a
// report names a CHARACTER — nobody is told "conta lokitoo está duplicando",
// they are told "Vandalyzz está duplicando". Searching only account names meant
// the first step of every investigation was guessing.
//
// It lives here rather than in internal/store beside the account-name search for
// the reason the account writes do: every service embeds internal/, so adding
// there redeploys the game to ship a panel change.
func (s *Store) Buscar(ctx context.Context, prefixo string, limite int) ([]Achado, error) {
	if limite <= 0 || limite > 200 {
		limite = 50
	}
	padrao := prefixo + "%"

	// One statement, not two merged in Go: the accounts a term matches both ways
	// must appear once, and DISTINCT ON does that in the place that can see both
	// halves. The character name is kept for the row that matched through one.
	//
	// The two halves match differently on purpose. Account names are stored
	// canonical lowercase and the caller lowercases the term, so LIKE is exact
	// and uses the unique index. CHARACTER names keep their real case —
	// "Vandalyzz" — and a moderator types "vandal", so that half has to be
	// case-insensitive.
	//
	// ILIKE cannot use character(name, class_master), so this half is a scan.
	// That is the right trade here: the page is opened a few times an hour by a
	// person, not by a loop. If the character table ever makes it slow, the fix
	// is an index on lower(name) — and it will be obvious, because this is the
	// only query that would have got slower.
	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT ON (a.id)
		       a.id, a.name, a.email, a.donate_balance, a.role,
		       (a.is_blocked AND (a.blocked_until IS NULL OR a.blocked_until > now())),
		       coalesce(c.name, '')
		  FROM account a
		  LEFT JOIN character c ON c.account_id = a.id AND c.name ILIKE $1
		 WHERE a.name LIKE $1 OR c.id IS NOT NULL
		 ORDER BY a.id, c.name
		 LIMIT $2`, padrao, limite)
	if err != nil {
		return nil, fmt.Errorf("accounts: search %q: %w", prefixo, err)
	}
	defer rows.Close()

	var out []Achado
	for rows.Next() {
		var a Achado
		if err := rows.Scan(&a.ID, &a.Name, &a.Email, &a.DonateBalance,
			&a.Role, &a.IsBlocked, &a.PorPersonagem); err != nil {
			return nil, fmt.Errorf("accounts: scan search row: %w", err)
		}
		// An account matched by its own name reports no character, even when it
		// happens to have one whose name also starts with the term: the column
		// answers "why is this row here", and the answer is the account name.
		if strings.HasPrefix(a.Name, prefixo) {
			a.PorPersonagem = ""
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("accounts: iterate search: %w", err)
	}
	// DISTINCT ON orders by a.id, which is not the order to read in. Sorting
	// here rather than in a subquery keeps the statement legible.
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// --- criação de conta ---

// Account-name bounds, matching what the game login carries and what
// webserver/internal/account already enforces for web sign-up (4–12 ASCII
// alphanumeric). The panel and the site must not disagree about what a valid
// login is: a name one accepts and the other refuses is an account that exists
// and cannot be used.
const (
	MinNomeConta = 4
	MaxNomeConta = 12
)

var (
	ErrNomeVazio     = errors.New("accounts: empty account name")
	ErrNomeCurto     = errors.New("accounts: account name too short")
	ErrNomeLongo     = errors.New("accounts: account name too long")
	ErrNomeCaractere = errors.New("accounts: account name must be letters and digits only")
	ErrNomeEmUso     = errors.New("accounts: account name already taken")
)

// ValidarNome checks an account name against the login rule, on the canonical
// (lowercased) form.
//
// Letters and digits only, ASCII: the name travels in a fixed byte field to a
// client from 2003, and anything outside that is a login the player can create
// and then fail to type.
func ValidarNome(nome string) error {
	switch {
	case nome == "":
		return ErrNomeVazio
	case len(nome) < MinNomeConta:
		return ErrNomeCurto
	case len(nome) > MaxNomeConta:
		return ErrNomeLongo
	}
	for i := 0; i < len(nome); i++ {
		c := nome[i]
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') {
			return ErrNomeCaractere
		}
	}
	return nil
}

// CanonicalNome is the stored form of a login: lowercase, trimmed. The game
// looks accounts up by this, so it is what uniqueness is about.
func CanonicalNome(nome string) string {
	return strings.ToLower(strings.TrimSpace(nome))
}

// Criar inserts a new account with an already-hashed password and returns its id.
//
// The hash is taken rather than the password: hashing lives in the caller
// (internal/secret) exactly as it does for SetPassword, and this way no
// plaintext ever reaches this package.
//
// A name collision comes back as ErrNomeEmUso rather than a raw driver error —
// two staff members creating the same name at once is an ordinary race, not a
// failure, and the unique index is what decides it.
func (s *Store) Criar(ctx context.Context, nome, hash, email string) (int64, error) {
	if err := ValidarNome(nome); err != nil {
		return 0, err
	}
	var id int64
	err := s.pool.QueryRow(ctx, `
		INSERT INTO account (name, pass_hash, email)
		VALUES ($1, $2, $3)
		RETURNING id`, nome, hash, strings.TrimSpace(email)).Scan(&id)
	if isUniqueViolation(err) {
		return 0, ErrNomeEmUso
	}
	if err != nil {
		return 0, fmt.Errorf("accounts: criar %q: %w", nome, err)
	}
	return id, nil
}

// isUniqueViolation reports whether err is a Postgres unique-constraint
// violation (SQLSTATE 23505), which is how a lost race on the account name comes
// back from the driver.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// PontoDeLojinha is one movement of an account's personal-shop wallet, as the
// account page shows it.
//
// Personagem is who had the stall standing and is informative only — the wallet
// belongs to the ACCOUNT, and character names are not unique on this server.
type PontoDeLojinha struct {
	Personagem string
	Delta      int32
	SaldoApos  int32
	Motivo     string
	Quando     time.Time
}

// PontosDeLojinha reads the latest movements of one account's shop-points wallet,
// newest first. It answers the question the balance alone cannot: where those
// points came from, and when.
//
// An empty result is normal (an account that never kept a shop open), so the
// caller shows an empty extrato rather than an error.
func (s *Store) PontosDeLojinha(ctx context.Context, id int64, limite int) ([]PontoDeLojinha, error) {
	if limite <= 0 || limite > 200 {
		limite = 50
	}
	rows, err := s.pool.Query(ctx, `
		SELECT character_name, delta, balance_after, reason, created_at
		  FROM shop_points_audit
		 WHERE account_id = $1
		 ORDER BY created_at DESC, id DESC
		 LIMIT $2`, id, limite)
	if err != nil {
		return nil, fmt.Errorf("accounts: extrato de pontos de %d: %w", id, err)
	}
	defer rows.Close()

	var out []PontoDeLojinha
	for rows.Next() {
		var p PontoDeLojinha
		if err := rows.Scan(&p.Personagem, &p.Delta, &p.SaldoApos, &p.Motivo, &p.Quando); err != nil {
			return nil, fmt.Errorf("accounts: ler linha do extrato de pontos: %w", err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("accounts: percorrer extrato de pontos: %w", err)
	}
	return out, nil
}
