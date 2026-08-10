package donaterevenue

import (
	"strings"
	"time"

	"github.com/jeanluca/w2pp-openwyd/internal/store"
)

// Validation bounds. The window cap is load-bearing three ways: it bounds the
// series row count, it keeps the range scan selective, and it stops an
// accidental "since 1970" query from scanning the whole order table.
const (
	defaultLimit       = 50
	maxLimit           = 100
	defaultSearchLimit = 20
	maxSearchLimit     = 50
	minPrefixLen       = 2
	defaultWindowDays  = 30
	maxWindowDays      = 366
)

// Window is the resolved [FromUnix, ToUnix) range a request was actually served
// with. Every windowed method returns it so the caller can echo it back and the
// UI labels the period truthfully rather than the one it asked for.
type Window struct {
	FromUnix int64
	ToUnix   int64
}

// normalizeWindow resolves the caller's Unix-second bounds into a concrete
// bounded range. Both 0 means the last defaultWindowDays ending now; only
// toUnix 0 means "now". ok is false when the range is nonsensical or too wide,
// which the caller turns into Invalid.
//
// The result is ALWAYS bounded, which is what lets every store predicate be a
// plain sargable `>= $1 AND < $2` with no NULL-guarded date logic.
func normalizeWindow(fromUnix, toUnix int64) (store.RevenueWindow, Window, bool) {
	if fromUnix < 0 || toUnix < 0 {
		return store.RevenueWindow{}, Window{}, false
	}

	to := time.Now().UTC()
	if toUnix != 0 {
		to = time.Unix(toUnix, 0).UTC()
	}
	from := to.AddDate(0, 0, -defaultWindowDays)
	if fromUnix != 0 {
		from = time.Unix(fromUnix, 0).UTC()
	}

	if !from.Before(to) {
		return store.RevenueWindow{}, Window{}, false
	}
	if to.Sub(from) > maxWindowDays*24*time.Hour {
		return store.RevenueWindow{}, Window{}, false
	}
	return store.RevenueWindow{From: from, To: to},
		Window{FromUnix: from.Unix(), ToUnix: to.Unix()},
		true
}

// normalizePage clamps the pager. Mirrors ranking.normalizePage; the duplication
// is deliberate — hoisting it would mean a shared util-shaped package.
func normalizePage(limit, offset int) (int, int) {
	if limit <= 0 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

// normalizeSearchLimit clamps the account picker's page size.
func normalizeSearchLimit(limit int) int {
	if limit <= 0 {
		return defaultSearchLimit
	}
	if limit > maxSearchLimit {
		return maxSearchLimit
	}
	return limit
}

// normalizePrefix canonicalizes a typed login to the lowercase form account.name
// is stored in. ok is false for a prefix shorter than minPrefixLen, which would
// return an unbounded slice of the account table.
func normalizePrefix(prefix string) (string, bool) {
	p := strings.ToLower(strings.TrimSpace(prefix))
	if len(p) < minPrefixLen {
		return "", false
	}
	return p, true
}

// maskCPF renders an 11-digit CPF as ***.456.789-** — enough for an operator to
// eyeball-match a payer against a bank statement, not enough to reuse the
// document.
//
// Masking lives here, in the service, rather than in the store or the BFF:
// 'moderator' is a coarse role shared with the NPC and mob editors, so everyone
// who can open the panel would otherwise see every payer's full CPF, and nothing
// in a revenue report needs it — a chargeback is reconciled by
// external_reference. There is deliberately no unmasked variant in the contract.
//
// Anything that is not exactly 11 digits renders as "" rather than leaking a
// partially-parsed value.
func maskCPF(digits string) string {
	if len(digits) != 11 {
		return ""
	}
	for i := 0; i < len(digits); i++ {
		if digits[i] < '0' || digits[i] > '9' {
			return ""
		}
	}
	return "***." + digits[3:6] + "." + digits[6:9] + "-**"
}

// bucketKeyword maps the request enum to the store's bucket constant. ok is
// false for Unspecified AND for any unrecognized value: an unknown bucket
// degrades to "no series" instead of failing, so a newer portal cannot 400
// against an older server.
func bucketKeyword(b Bucket) (string, bool) {
	switch b {
	case BucketDay:
		return store.BucketDay, true
	case BucketWeek:
		return store.BucketWeek, true
	case BucketMonth:
		return store.BucketMonth, true
	default:
		return "", false
	}
}
