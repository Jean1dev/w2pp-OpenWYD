// Package playcode issues and checks the one-time login codes that let a player
// who is already signed in on the portal enter the web client without typing
// the password again (web client ADR 017).
//
// The code rides in the legacy password field of MSG_AccountLogin (0x020D), so
// the wire format and the 7662 client stay untouched. That field is 12 bytes
// and the client copies it with sprintf_s, which needs the terminator, hence
// the 10-character length.
//
// Codes are stored as SHA-256, not argon2id: argon2id protects low-entropy
// human passwords against offline guessing, while a code is 50 random bits,
// single-use and lives two minutes, so a fast hash loses nothing and lets the
// store look it up directly.
package playcode

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// Length is the code length; it must stay below the 12-byte password field.
	Length = 10
	// Alphabet has 32 symbols without look-alikes (no 0/o, 1/l), so each byte
	// of randomness maps to a symbol without modulo bias.
	Alphabet = "abcdefghijkmnpqrstuvwxyz23456789"
	// TTL is how long an issued code stays valid.
	TTL = 2 * time.Minute

	// AssertionLabel domain-separates the portal's request signature from the
	// web client's play ticket, which uses the same token format.
	AssertionLabel = "wyd-play-code.v1."
	// MinSecretLength is the minimum PLAY_CODE_SECRET length.
	MinSecretLength = 32

	maxAssertionBytes = 1024
	maxAssertionLife  = 2 * time.Minute
	clockSkew         = 30 * time.Second
	maxSeen           = 4096
)

// New returns a fresh random code.
func New() (string, error) {
	b := make([]byte, Length)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("playcode: random: %w", err)
	}
	for i := range b {
		b[i] = Alphabet[int(b[i])%len(Alphabet)]
	}
	return string(b), nil
}

// Hash is the stored form of a code.
func Hash(code string) []byte {
	h := sha256.Sum256([]byte(code))
	return h[:]
}

// Valid reports whether s has the shape of a code, so a password that cannot
// be one never costs a database round-trip.
func Valid(s string) bool {
	if len(s) != Length {
		return false
	}
	for i := 0; i < len(s); i++ {
		if !strings.ContainsRune(Alphabet, rune(s[i])) {
			return false
		}
	}
	return true
}

// Claims is the portal's signed request: "issue a code for account Sub".
// The account id is the portal session's accountId (a decimal string).
type Claims struct {
	Sub string `json:"sub"`
	Iat int64  `json:"iat"`
	Exp int64  `json:"exp"`
	Jti string `json:"jti"`
}

// Sign encodes claims as base64url(JSON) "." base64url(HMAC-SHA256(secret,
// AssertionLabel + payload)), the format of the web client's play ticket.
func Sign(secret []byte, c Claims) string {
	b, err := json.Marshal(c)
	if err != nil {
		panic(err) // Claims holds only strings and ints
	}
	payload := base64.RawURLEncoding.EncodeToString(b)
	return payload + "." + base64.RawURLEncoding.EncodeToString(mac(secret, payload))
}

func mac(secret []byte, payload string) []byte {
	m := hmac.New(sha256.New, secret)
	m.Write([]byte(AssertionLabel + payload))
	return m.Sum(nil)
}

// Verifier checks portal assertions and refuses a jti seen before. It is safe
// for concurrent use; it lives in the webserver, outside the game loop.
type Verifier struct {
	secret []byte
	now    func() time.Time

	mu   sync.Mutex
	seen map[string]time.Time // jti -> expiry
}

// NewVerifier returns a Verifier, or nil when the secret is too short (the
// feature then stays off).
func NewVerifier(secret string) *Verifier {
	if len(secret) < MinSecretLength {
		return nil
	}
	return &Verifier{secret: []byte(secret), now: time.Now, seen: map[string]time.Time{}}
}

// Verify validates an assertion and consumes its jti, returning the account id.
// The error text is safe to log; the token itself never is.
func (v *Verifier) Verify(token string) (int64, error) {
	if token == "" || len(token) > maxAssertionBytes {
		return 0, errors.New("missing or oversized assertion")
	}
	payload, sig, ok := strings.Cut(token, ".")
	if !ok || strings.Contains(sig, ".") {
		return 0, errors.New("malformed assertion")
	}
	got, err := base64.RawURLEncoding.DecodeString(sig)
	if err != nil || !hmac.Equal(got, mac(v.secret, payload)) {
		return 0, errors.New("bad signature")
	}
	raw, err := base64.RawURLEncoding.DecodeString(payload)
	var c Claims
	if err != nil || json.Unmarshal(raw, &c) != nil {
		return 0, errors.New("malformed payload")
	}
	id, err := strconv.ParseInt(c.Sub, 10, 64)
	now := v.now()
	switch {
	case err != nil || id <= 0:
		return 0, errors.New("bad subject")
	case len(c.Jti) < 16 || len(c.Jti) > 64:
		return 0, errors.New("bad jti")
	case c.Exp <= c.Iat || time.Duration(c.Exp-c.Iat)*time.Second > maxAssertionLife:
		return 0, errors.New("bad lifetime")
	case now.Add(clockSkew).Unix() < c.Iat:
		return 0, errors.New("issued in the future")
	case now.Add(-clockSkew).Unix() >= c.Exp:
		return 0, errors.New("expired")
	}

	v.mu.Lock()
	defer v.mu.Unlock()
	if _, used := v.seen[c.Jti]; used {
		return 0, errors.New("replayed")
	}
	if len(v.seen) >= maxSeen {
		for jti, exp := range v.seen {
			if now.Add(-clockSkew).After(exp) {
				delete(v.seen, jti)
			}
		}
		if len(v.seen) >= maxSeen {
			return 0, errors.New("replay cache full")
		}
	}
	v.seen[c.Jti] = time.Unix(c.Exp, 0)
	return id, nil
}
