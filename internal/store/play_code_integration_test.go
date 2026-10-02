//go:build integration

// Integration tests for the one-time play codes (web client ADR 017).
//
//	W2PP_TEST_DSN=postgres://postgres:dev@localhost:5432/postgres go test -tags=integration ./internal/store/
package store

import (
	"errors"
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/internal/domain"
)

func TestPlayCodeIssueAndConsume(t *testing.T) {
	s, ctx := freshStore(t)
	alice, err := s.SaveAccount(ctx, domain.Account{Name: "alice", PassHash: "$argon2id$x"})
	if err != nil {
		t.Fatalf("SaveAccount alice: %v", err)
	}
	bob, err := s.SaveAccount(ctx, domain.Account{Name: "bob", PassHash: "$argon2id$x"})
	if err != nil {
		t.Fatalf("SaveAccount bob: %v", err)
	}
	soon := time.Now().Add(2 * time.Minute)

	issued, err := s.IssuePlayCode(ctx, alice, []byte("code-1"), soon)
	if err != nil || issued.Name != "alice" || issued.Blocked {
		t.Fatalf("IssuePlayCode = %+v, %v", issued, err)
	}
	// Another account cannot use it.
	if ok, err := s.ConsumePlayCode(ctx, bob, []byte("code-1")); err != nil || ok {
		t.Fatalf("ConsumePlayCode(bob) = %v, %v; want false", ok, err)
	}
	// The owner can, exactly once.
	if ok, err := s.ConsumePlayCode(ctx, alice, []byte("code-1")); err != nil || !ok {
		t.Fatalf("first ConsumePlayCode = %v, %v; want true", ok, err)
	}
	if ok, err := s.ConsumePlayCode(ctx, alice, []byte("code-1")); err != nil || ok {
		t.Fatalf("second ConsumePlayCode = %v, %v; want false", ok, err)
	}

	// An expired code does not log in.
	if _, err := s.IssuePlayCode(ctx, alice, []byte("old"), time.Now().Add(-time.Second)); err != nil {
		t.Fatalf("IssuePlayCode(expired): %v", err)
	}
	if ok, err := s.ConsumePlayCode(ctx, alice, []byte("old")); err != nil || ok {
		t.Fatalf("ConsumePlayCode(expired) = %v, %v; want false", ok, err)
	}

	// Unknown account.
	if _, err := s.IssuePlayCode(ctx, 999999, []byte("x"), soon); !errors.Is(err, ErrNotFound) {
		t.Fatalf("IssuePlayCode(unknown) err = %v, want ErrNotFound", err)
	}
}

func TestPlayCodeCapAndBlocked(t *testing.T) {
	s, ctx := freshStore(t)
	id, err := s.SaveAccount(ctx, domain.Account{Name: "carol", PassHash: "$argon2id$x"})
	if err != nil {
		t.Fatalf("SaveAccount: %v", err)
	}
	soon := time.Now().Add(2 * time.Minute)
	codes := []string{"c1", "c2", "c3", "c4", "c5", "c6"}
	for _, c := range codes {
		if _, err := s.IssuePlayCode(ctx, id, []byte(c), soon); err != nil {
			t.Fatalf("IssuePlayCode(%s): %v", c, err)
		}
		time.Sleep(2 * time.Millisecond) // distinct created_at for the ordering
	}
	// Only the newest maxActivePlayCodes survive: the first one is gone.
	if ok, _ := s.ConsumePlayCode(ctx, id, []byte("c1")); ok {
		t.Fatal("oldest code survived the cap")
	}
	if ok, _ := s.ConsumePlayCode(ctx, id, []byte("c6")); !ok {
		t.Fatal("newest code was dropped by the cap")
	}

	if err := s.SetBlockedByName(ctx, "carol", true); err != nil {
		t.Fatalf("SetBlockedByName: %v", err)
	}
	issued, err := s.IssuePlayCode(ctx, id, []byte("blocked"), soon)
	if err != nil || !issued.Blocked {
		t.Fatalf("IssuePlayCode(blocked) = %+v, %v; want Blocked", issued, err)
	}
	if ok, _ := s.ConsumePlayCode(ctx, id, []byte("blocked")); ok {
		t.Fatal("a code was stored for a blocked account")
	}
}
