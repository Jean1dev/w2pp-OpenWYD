package secret

import "testing"

func TestHashVerifyRoundTrip(t *testing.T) {
	const pw = "s3cr3t-pass"
	phc, err := HashSecret(pw)
	if err != nil {
		t.Fatalf("HashSecret: %v", err)
	}
	if phc == "" {
		t.Fatal("HashSecret returned empty for non-empty input")
	}

	ok, err := VerifySecret(pw, phc)
	if err != nil {
		t.Fatalf("VerifySecret: %v", err)
	}
	if !ok {
		t.Fatal("correct password did not verify")
	}

	ok, err = VerifySecret("wrong", phc)
	if err != nil {
		t.Fatalf("VerifySecret(wrong): %v", err)
	}
	if ok {
		t.Fatal("wrong password verified")
	}
}

func TestHashSaltIsRandom(t *testing.T) {
	a, _ := HashSecret("same")
	b, _ := HashSecret("same")
	if a == b {
		t.Fatal("identical inputs produced identical hashes (salt not random)")
	}
}

func TestVerifyEmptySecret(t *testing.T) {
	// An unset secret hashes to "" and only matches an empty plaintext.
	ok, err := VerifySecret("", "")
	if err != nil || !ok {
		t.Fatalf("empty/empty should match: ok=%v err=%v", ok, err)
	}
	ok, _ = VerifySecret("x", "")
	if ok {
		t.Fatal("non-empty plaintext matched an unset secret")
	}
}

func TestVerifyMalformedHash(t *testing.T) {
	for _, bad := range []string{"not-a-hash", "$argon2id$v=19$bad", "$argon2id$v=1$m=1,t=1,p=1$x$y"} {
		if _, err := VerifySecret("x", bad); err == nil {
			t.Errorf("expected error for malformed hash %q", bad)
		}
	}
}
