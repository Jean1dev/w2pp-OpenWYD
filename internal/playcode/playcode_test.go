package playcode

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

const testSecret = "wyd-play-code-test-secret-0123456789abcdef"

// nodeVector is the same assertion signed by the portal (node:crypto) in
// wyd-plataforma src/lib/play-code.test.ts; both sides must agree byte for byte.
const nodeVector = "eyJzdWIiOiI0MiIsImlhdCI6MTc5MDAwMDAwMCwiZXhwIjoxNzkwMDAwMDYwLCJqdGkiOiIwMTIzNDU2Nzg5YWJjZGVmMDEyMzQ1Njc4OWFiY2RlZiJ9.O52BvFIj7L-JOw8oVCdC6Sh5IxFUtDc-yDM4VhheYgE"

var vectorClaims = Claims{Sub: "42", Iat: 1790000000, Exp: 1790000060, Jti: "0123456789abcdef0123456789abcdef"}

func verifierAt(t *testing.T, at time.Time) *Verifier {
	t.Helper()
	v := NewVerifier(testSecret)
	if v == nil {
		t.Fatal("NewVerifier returned nil for a valid secret")
	}
	v.now = func() time.Time { return at }
	return v
}

func TestNewCodeShape(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		c, err := New()
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		if !Valid(c) {
			t.Fatalf("New returned %q, not a valid code", c)
		}
		if seen[c] {
			t.Fatalf("duplicate code %q in 200 draws", c)
		}
		seen[c] = true
	}
}

func TestValid(t *testing.T) {
	for _, s := range []string{"", "abcdefghi", "abcdefghijk", "abcdefghi0", "ABCDEFGHIJ", "abcdefghi1", "abcdefghi "} {
		if Valid(s) {
			t.Errorf("Valid(%q) = true", s)
		}
	}
	if !Valid("abcdefgh29") {
		t.Error(`Valid("abcdefgh29") = false`)
	}
}

func TestHashIsStable(t *testing.T) {
	if !bytes.Equal(Hash("abcdefgh29"), Hash("abcdefgh29")) || bytes.Equal(Hash("abcdefgh29"), Hash("abcdefgh28")) {
		t.Fatal("Hash must be deterministic and distinguish codes")
	}
}

func TestSignMatchesNodeVector(t *testing.T) {
	if got := Sign([]byte(testSecret), vectorClaims); got != nodeVector {
		t.Fatalf("Sign = %s\nwant   %s", got, nodeVector)
	}
}

func TestVerify(t *testing.T) {
	at := time.Unix(vectorClaims.Iat+5, 0)
	id, err := verifierAt(t, at).Verify(nodeVector)
	if err != nil || id != 42 {
		t.Fatalf("Verify(vector) = %d, %v; want 42", id, err)
	}

	sign := func(c Claims) string { return Sign([]byte(testSecret), c) }
	ok := vectorClaims
	cases := []struct {
		name, token, want string
	}{
		{"empty", "", "missing"},
		{"oversized", strings.Repeat("a", maxAssertionBytes+1), "missing"},
		{"no dot", "abc", "malformed"},
		{"other secret", Sign([]byte(strings.Repeat("x", 40)), ok), "bad signature"},
		{"bad subject", sign(Claims{Sub: "alice", Iat: ok.Iat, Exp: ok.Exp, Jti: ok.Jti}), "bad subject"},
		{"zero subject", sign(Claims{Sub: "0", Iat: ok.Iat, Exp: ok.Exp, Jti: ok.Jti}), "bad subject"},
		{"short jti", sign(Claims{Sub: "42", Iat: ok.Iat, Exp: ok.Exp, Jti: "short"}), "bad jti"},
		{"long life", sign(Claims{Sub: "42", Iat: ok.Iat, Exp: ok.Iat + 600, Jti: ok.Jti}), "bad lifetime"},
		{"future", sign(Claims{Sub: "42", Iat: at.Unix() + 120, Exp: at.Unix() + 180, Jti: ok.Jti}), "future"},
		{"expired", sign(Claims{Sub: "42", Iat: at.Unix() - 200, Exp: at.Unix() - 140, Jti: ok.Jti}), "expired"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := verifierAt(t, at).Verify(tc.token); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Verify err = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestVerifyRefusesReplay(t *testing.T) {
	v := verifierAt(t, time.Unix(vectorClaims.Iat+5, 0))
	if _, err := v.Verify(nodeVector); err != nil {
		t.Fatalf("first Verify: %v", err)
	}
	if _, err := v.Verify(nodeVector); err == nil || !strings.Contains(err.Error(), "replayed") {
		t.Fatalf("second Verify err = %v, want replayed", err)
	}
}

func TestNewVerifierNeedsLongSecret(t *testing.T) {
	if NewVerifier(strings.Repeat("s", MinSecretLength-1)) != nil {
		t.Fatal("NewVerifier accepted a short secret")
	}
}
