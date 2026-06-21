// Package convert maps the raw save structs (savefmt) into the relational
// domain model, hashing all secrets on import. The legacy plaintext passwords
// and PINs (data-formats.md §1.3) are NEVER carried into the new stack —
// migration-plan.md §5.
package convert

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"

	"golang.org/x/crypto/argon2"
)

// argonParams are the argon2id cost parameters used for all imported secrets.
type argonParams struct {
	memory  uint32
	time    uint32
	threads uint8
	keyLen  uint32
	saltLen uint32
}

// defaultArgon follows OWASP's argon2id guidance (64 MiB, t=1, p=4).
var defaultArgon = argonParams{memory: 64 * 1024, time: 1, threads: 4, keyLen: 32, saltLen: 16}

// HashSecret returns an argon2id PHC-encoded hash of plain, or "" when plain is
// empty (an unset secret — e.g. no block password). A fresh random salt is used
// per call, so the same input yields different hashes.
func HashSecret(plain string) (string, error) {
	if plain == "" {
		return "", nil
	}
	salt := make([]byte, defaultArgon.saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("convert: generate salt: %w", err)
	}
	h := argon2.IDKey([]byte(plain), salt, defaultArgon.time, defaultArgon.memory, defaultArgon.threads, defaultArgon.keyLen)
	b64 := base64.RawStdEncoding
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, defaultArgon.memory, defaultArgon.time, defaultArgon.threads,
		b64.EncodeToString(salt), b64.EncodeToString(h)), nil
}

// ErrBadHash is returned by VerifySecret when phc is not a hash this package
// produced (wrong scheme, version, or malformed encoding).
var ErrBadHash = errors.New("convert: malformed argon2id hash")

// VerifySecret reports whether plain matches the argon2id PHC hash produced by
// HashSecret. The comparison is constant-time. An empty stored hash means the
// secret was unset on import, so it only matches an empty plain. Plaintext is
// never reconstructed — we re-derive the hash from plain and compare digests.
func VerifySecret(plain, phc string) (bool, error) {
	if phc == "" {
		return plain == "", nil
	}
	var (
		version          int
		memory, time     uint32
		threads          uint8
		saltB64, hashB64 string
	)
	// Matches the exact format written by HashSecret.
	if _, err := fmt.Sscanf(phc, "$argon2id$v=%d$m=%d,t=%d,p=%d$%s",
		&version, &memory, &time, &threads, &saltB64); err != nil {
		return false, fmt.Errorf("%w: %v", ErrBadHash, err)
	}
	if version != argon2.Version {
		return false, fmt.Errorf("%w: version %d", ErrBadHash, version)
	}
	// Sscanf's trailing %s captured "salt$hash"; split on the separator.
	sep := -1
	for i := 0; i < len(saltB64); i++ {
		if saltB64[i] == '$' {
			sep = i
			break
		}
	}
	if sep < 0 {
		return false, fmt.Errorf("%w: missing hash segment", ErrBadHash)
	}
	saltB64, hashB64 = saltB64[:sep], saltB64[sep+1:]

	b64 := base64.RawStdEncoding
	salt, err := b64.DecodeString(saltB64)
	if err != nil {
		return false, fmt.Errorf("%w: salt: %v", ErrBadHash, err)
	}
	want, err := b64.DecodeString(hashB64)
	if err != nil {
		return false, fmt.Errorf("%w: hash: %v", ErrBadHash, err)
	}

	got := argon2.IDKey([]byte(plain), salt, time, memory, threads, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}
